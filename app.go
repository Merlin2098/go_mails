package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"boleta-mailer/app/application"
	"boleta-mailer/app/domain"
	"boleta-mailer/app/infrastructure/excel"
	"boleta-mailer/app/infrastructure/outlook"
)

// App struct
type App struct {
	ctx          context.Context
	log          *slog.Logger
	currentBatch domain.Batch

	// sendMu guards cancelSend and sending — StartBatch/CancelBatch are
	// bindings, callable concurrently from the UI thread's perspective, but
	// only one batch may run at a time (context.md §13: one Outlook worker).
	sendMu     sync.Mutex
	cancelSend context.CancelFunc
	sending    bool
}

// NewApp creates a new App application struct. log must never receive
// boleta contents, salary data, or full email bodies — only operational
// metadata (context.md §23-24).
func NewApp(log *slog.Logger) *App {
	return &App{log: log}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// OutlookStatus is the Wails-bound shape consumed by the "Probar conexión
// con Outlook" button (docs/specs/ui_spec_mvp.md §3).
type OutlookStatus struct {
	Available bool
	Email     string
	Detail    string
}

// TestOutlookConnection probes for Outlook Classic via COM and returns the
// SMTP address of the configured default account. It performs no send and
// has no side effects — safe to call repeatedly from the UI.
func (a *App) TestOutlookConnection() OutlookStatus {
	result := outlook.TestConnection()
	a.log.Info("outlook connection test", "available", result.Available, "detail", result.Detail)
	return OutlookStatus{
		Available: result.Available,
		Email:     result.Email,
		Detail:    result.Detail,
	}
}

// SelectDocumentsFolder opens a native folder picker for the boletas folder
// (docs/specs/ui_spec_mvp.md §3). Returns "" if the user cancels.
func (a *App) SelectDocumentsFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Seleccionar carpeta de boletas",
	})
}

// SelectExcel opens a native file picker filtered to .xlsx. Returns "" if
// the user cancels.
func (a *App) SelectExcel() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Seleccionar Excel de destinatarios",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
}

// ValidationError is the Wails-bound shape of one per-row validation
// failure, matching docs/specs/ui_spec_mvp.md §3.
type ValidationError struct {
	Row     int
	Field   string
	Message string
}

// BatchValidationSummary is the Wails-bound result of validating a batch,
// matching docs/specs/ui_spec_mvp.md §3.
type BatchValidationSummary struct {
	Total   int
	Valid   int
	Invalid int
	Errors  []ValidationError
}

// ValidateBatch reads excelPath and resolves documents inside folderPath,
// validating the full batch before any send can happen (context.md §15).
// The resolved domain.Batch is kept on App for a later StartBatch/
// ExportReport call — the UI only ever sees the projected summary below.
func (a *App) ValidateBatch(excelPath string, folderPath string) (BatchValidationSummary, error) {
	batch, err := application.ValidateBatch(excelPath, folderPath)
	if err != nil {
		a.log.Error("validate batch failed", "error", err)
		return BatchValidationSummary{}, err
	}
	a.currentBatch = batch

	summary := batch.Summary()
	a.log.Info("batch validated", "total", summary.Total, "valid", summary.Valid, "invalid", summary.Invalid)
	result := BatchValidationSummary{
		Total:   summary.Total,
		Valid:   summary.Valid,
		Invalid: summary.Invalid,
	}
	for i, item := range batch.Items {
		if item.Status == domain.StatusInvalid {
			result.Errors = append(result.Errors, ValidationError{
				Row:     i + 2, // 1-based, plus header row
				Field:   item.ValidationField,
				Message: item.ValidationError,
			})
		}
	}
	return result, nil
}

// BatchProgressEvent is the payload emitted on the "batch:progress" Wails
// event, matching docs/specs/ui_spec_mvp.md §3.
type BatchProgressEvent struct {
	Total     int
	Processed int
	Sent      int
	Failed    int
	Current   string
}

// StartBatch sends the batch produced by the last ValidateBatch call. It
// returns immediately — the actual send runs on a dedicated goroutine
// (required by COM apartment threading: the same goroutine that calls
// outlook.Connect must be the one calling Send and Close) and reports
// progress via the "batch:progress" event. Only one batch can run at a
// time; calling StartBatch again while one is in flight returns an error.
func (a *App) StartBatch() error {
	a.sendMu.Lock()
	if a.sending {
		a.sendMu.Unlock()
		return errBatchAlreadyRunning
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelSend = cancel
	a.sending = true
	a.sendMu.Unlock()

	go a.runBatchWorker(ctx)
	return nil
}

// runBatchWorker owns the Outlook COM connection for the duration of one
// batch: it connects once, sends every item sequentially on this same
// goroutine, then closes — never touching COM from any other goroutine.
//
// LockOSThread is required, not optional: COM objects are apartment-
// threaded — CoInitialize binds the connection to whatever OS thread calls
// it, but Go's scheduler is free to migrate an unlocked goroutine to a
// different OS thread between calls. Without the lock, a later Send() can
// run on a different OS thread than Connect() did, and Outlook rejects it
// with RPC_E_WRONG_THREAD ("La aplicación llamó a una interfaz que se
// aplanó para un diferente subproceso") — observed in practice before this
// fix, logged in logs/app.log.
func (a *App) runBatchWorker(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	defer func() {
		a.sendMu.Lock()
		a.sending = false
		a.cancelSend = nil
		a.sendMu.Unlock()
	}()

	a.log.Info("batch send starting", "total", len(a.currentBatch.Items))

	adapter, err := outlook.Connect()
	if err != nil {
		a.log.Error("outlook connect failed, batch aborted", "error", err)
		wailsruntime.EventsEmit(a.ctx, "batch:fatal-error", err.Error())
		return
	}
	defer adapter.Close()

	var final application.Progress
	application.RunBatch(ctx, &a.currentBatch, adapter, func(p application.Progress) {
		if p.Item.Status == domain.StatusFailed {
			a.log.Error("item send failed", "dni", p.Item.Employee.DNI, "file", p.Item.Document.Path, "error", p.Item.SendError)
		} else {
			a.log.Info("item sent", "dni", p.Item.Employee.DNI, "file", p.Item.Document.Path)
		}
		final = p

		wailsruntime.EventsEmit(a.ctx, "batch:progress", BatchProgressEvent{
			Total:     p.Total,
			Processed: p.Processed,
			Sent:      p.Sent,
			Failed:    p.Failed,
			Current:   p.Current,
		})
	})

	a.log.Info("batch send finished", "sent", final.Sent, "failed", final.Failed)
}

// CancelBatch requests cooperative cancellation of the in-flight batch
// (context.md §22) — the item currently sending is allowed to finish; items
// not yet reached are left as CANCELLED. No-op if no batch is running.
func (a *App) CancelBatch() {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()
	if a.cancelSend != nil {
		a.cancelSend()
	}
}

// RetryFailed re-sends only the items left in FAILED status by the last
// StartBatch/RetryFailed run (context.md §15: never re-send the ones that
// already succeeded). Same async/event/worker mechanics as StartBatch —
// see runBatchWorker. Returns an error immediately, without starting a
// worker, if there is nothing to retry.
func (a *App) RetryFailed() error {
	a.sendMu.Lock()
	if a.sending {
		a.sendMu.Unlock()
		return errBatchAlreadyRunning
	}
	retried := application.PrepareRetry(&a.currentBatch)
	if retried == 0 {
		a.sendMu.Unlock()
		return errNothingToRetry
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelSend = cancel
	a.sending = true
	a.sendMu.Unlock()

	a.log.Info("retrying failed items", "count", retried)
	go a.runBatchWorker(ctx)
	return nil
}

// ExportReport opens a native "save as" dialog and writes the current
// batch's send result to the chosen path as an .xlsx
// (docs/specs/ui_spec_mvp.md §3, "Generar reporte Excel"). Safe to call
// after StartBatch/RetryFailed finishes — reads the in-memory currentBatch,
// no dependency on Outlook. Returns "" (no error) if the user cancels the
// dialog.
func (a *App) ExportReport() (string, error) {
	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Guardar reporte de envío",
		DefaultFilename: "reporte_envio.xlsx",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil || savePath == "" {
		return "", err
	}

	if err := excel.WriteReport(a.currentBatch, savePath); err != nil {
		a.log.Error("export report failed", "error", err)
		return "", err
	}
	a.log.Info("report exported", "path", savePath)
	return savePath, nil
}

var errBatchAlreadyRunning = fmt.Errorf("ya hay un lote en proceso de envío")
var errNothingToRetry = fmt.Errorf("no hay elementos fallidos para reintentar")
