package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"runtime"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"boleta-mailer/app/application"
	"boleta-mailer/app/domain"
	"boleta-mailer/app/infrastructure/excel"
	"boleta-mailer/app/infrastructure/gmail"
	"boleta-mailer/app/infrastructure/outlook"
)

// mailProvider identifies which transport StartBatch/RetryFailed should
// use. Selecting an account for one provider (SelectOutlookAccount /
// SelectGmailAccount) implicitly makes it the active one — there is no
// separate "choose provider" step, matching how each provider's own
// account selection already gates sending.
type mailProvider int

const (
	providerNone mailProvider = iota
	providerOutlook
	providerGmail
)

// App struct
type App struct {
	ctx          context.Context
	log          *slog.Logger
	currentBatch domain.Batch

	// selectedAccount is the SMTP address of the Outlook account the user
	// explicitly chose to send from. Empty means no explicit choice was made
	// yet — the app must never silently fall back to "whichever account
	// Outlook treats as default" (docs/specs/POC_Outlook_Hotmail_Desktop.md
	// §3, §6): StartBatch refuses to run until this is set.
	selectedAccount string

	// gmailEmail/gmailAppPassword hold the Gmail credentials the user
	// entered, kept only in memory for the current session — never written
	// to disk or logged (docs/specs/POC_Gmail_Desktop.md, revisión SMTP).
	gmailEmail       string
	gmailAppPassword string

	// activeProvider selects which MailSender StartBatch/RetryFailed use.
	// Empty means neither Outlook nor Gmail has been configured yet.
	activeProvider mailProvider

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

// OutlookAccount is the Wails-bound shape of one account configured in the
// user's local Outlook Classic profile
// (docs/specs/POC_Outlook_Hotmail_Desktop.md §5, "Selecciona cuenta de
// Outlook").
type OutlookAccount struct {
	DisplayName string
	SmtpAddress string
}

// ListOutlookAccounts enumerates every account configured in Outlook Classic
// on this machine, so the UI can show an explicit picker instead of the app
// assuming the first/default configured account is the one to send from.
func (a *App) ListOutlookAccounts() ([]OutlookAccount, error) {
	accounts, err := outlook.ListAccounts()
	if err != nil {
		a.log.Error("list outlook accounts failed", "error", err)
		return nil, err
	}
	result := make([]OutlookAccount, len(accounts))
	for i, acc := range accounts {
		result[i] = OutlookAccount{DisplayName: acc.DisplayName, SmtpAddress: acc.SmtpAddress}
	}
	return result, nil
}

// SelectOutlookAccount records which configured account the user wants to
// send from, and makes Outlook the active provider for StartBatch/RetryFailed.
func (a *App) SelectOutlookAccount(smtpAddress string) {
	a.log.Info("outlook account selected", "account", smtpAddress)
	a.selectedAccount = smtpAddress
	a.activeProvider = providerOutlook
}

// GmailStatus is the Wails-bound shape consumed by the Gmail connection
// section (docs/specs/POC_Gmail_Desktop.md, revisión SMTP).
type GmailStatus struct {
	Available bool
	Email     string
	Detail    string
}

// TestGmailConnection validates email/appPassword against Gmail's SMTP
// server without sending anything (gmail.TestConnection). It does not
// change the active provider — SelectGmailAccount does that, after the
// caller has confirmed the credentials work. The email format is checked
// here first so a malformed address fails with a clear message instead of
// an opaque SMTP/TLS error from gmail.TestConnection.
func (a *App) TestGmailConnection(email string, appPassword string) GmailStatus {
	if _, err := mail.ParseAddress(email); err != nil {
		return GmailStatus{Detail: "La dirección de correo no es válida."}
	}
	result := gmail.TestConnection(email, appPassword)
	a.log.Info("gmail connection test", "available", result.Available, "detail", result.Detail)
	return GmailStatus{
		Available: result.Available,
		Email:     result.Email,
		Detail:    result.Detail,
	}
}

// SelectGmailAccount stores the Gmail credentials for the current session
// (memory only — never written to disk or logged) and makes Gmail the
// active provider for StartBatch/RetryFailed. Callers should have already
// validated the credentials via TestGmailConnection.
func (a *App) SelectGmailAccount(email string, appPassword string) {
	a.log.Info("gmail account selected", "email", email)
	a.gmailEmail = email
	a.gmailAppPassword = appPassword
	a.activeProvider = providerGmail
}

// DisconnectGmailAccount clears the in-memory Gmail credentials and, if
// Gmail was the active provider, requires the user to configure a provider
// again before the next StartBatch/RetryFailed.
func (a *App) DisconnectGmailAccount() {
	a.log.Info("gmail account disconnected")
	a.gmailEmail = ""
	a.gmailAppPassword = ""
	if a.activeProvider == providerGmail {
		a.activeProvider = providerNone
	}
}

// SendTestGmail sends a single test email through the currently configured
// Gmail credentials, independent of any batch — used by the "Enviar
// prueba" button once a connection has been validated. Unlike batch sends,
// this recipient never goes through application.ValidateBatch, so its
// format is checked here — otherwise an invalid value (e.g. a name typed
// into the wrong field) only surfaces as a raw SMTP rejection from Gmail
// (see logs/bug-gmail.log).
func (a *App) SendTestGmail(to string) error {
	if a.gmailEmail == "" {
		return errNoAccountSelected
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return errInvalidRecipient
	}
	adapter := gmail.NewAdapter(a.gmailEmail, a.gmailAppPassword)
	message := domain.Message{
		To:      to,
		Subject: "Prueba de conexión — BoletaMailer",
		Body:    "Este es un correo de prueba enviado desde BoletaMailer vía Gmail.",
	}
	if err := adapter.Send(context.Background(), message); err != nil {
		a.log.Error("gmail test send failed", "error", err)
		return err
	}
	a.log.Info("gmail test send succeeded", "to", to)
	return nil
}

// gmailAppPasswordsURL is Google's own page for generating an App Password
// — the credential this app's Gmail connection needs instead of the
// account's real password (docs/specs/POC_Gmail_Desktop.md §3).
const gmailAppPasswordsURL = "https://myaccount.google.com/apppasswords"

// OpenGmailAppPasswordsPage opens Google's App Passwords page in the user's
// default system browser. This is presentation-only convenience — it does
// not detect installed browsers or drive any login flow; the OS decides
// which browser handles the URL, same as double-clicking a link anywhere
// else on the system.
func (a *App) OpenGmailAppPasswordsPage() {
	wailsruntime.BrowserOpenURL(a.ctx, gmailAppPasswordsURL)
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

// StartBatch sends the batch produced by the last ValidateBatch call, using
// whichever provider was last selected (SelectOutlookAccount or
// SelectGmailAccount). It returns immediately — the actual send runs on a
// dedicated goroutine (runBatchWorker) and reports progress via the
// "batch:progress" event. Only one batch can run at a time; calling
// StartBatch again while one is in flight returns an error.
func (a *App) StartBatch() error {
	a.sendMu.Lock()
	if a.sending {
		a.sendMu.Unlock()
		return errBatchAlreadyRunning
	}
	if a.activeProvider == providerNone {
		a.sendMu.Unlock()
		return errNoAccountSelected
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelSend = cancel
	a.sending = true
	provider, account, email, appPassword := a.activeProvider, a.selectedAccount, a.gmailEmail, a.gmailAppPassword
	a.sendMu.Unlock()

	go a.runBatchWorker(ctx, provider, account, email, appPassword)
	return nil
}

// runBatchWorker builds the domain.MailSender for the active provider and
// sends every item sequentially on this same goroutine. Only the Outlook
// path needs runtime.LockOSThread (see the comment inside the switch below);
// Gmail's SMTP path is plain net/smtp and does not.
func (a *App) runBatchWorker(ctx context.Context, provider mailProvider, account, gmailEmail, gmailAppPassword string) {
	defer func() {
		a.sendMu.Lock()
		a.sending = false
		a.cancelSend = nil
		a.sendMu.Unlock()
	}()

	a.log.Info("batch send starting", "total", len(a.currentBatch.Items), "provider", provider)

	var sender domain.MailSender
	switch provider {
	case providerOutlook:
		// LockOSThread is required, not optional, for the Outlook COM path
		// specifically: COM objects are apartment-threaded — CoInitialize
		// binds the connection to whatever OS thread calls it, but Go's
		// scheduler is free to migrate an unlocked goroutine to a different
		// OS thread between calls. Without the lock, a later Send() can run
		// on a different OS thread than Connect() did, and Outlook rejects
		// it with RPC_E_WRONG_THREAD ("La aplicación llamó a una interfaz
		// que se aplanó para un diferente subproceso") — observed in
		// practice before this fix, logged in logs/app.log. Gmail's SMTP
		// path below has no such constraint, so it must not pay this cost.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		adapter, err := outlook.Connect(account)
		if err != nil {
			a.log.Error("outlook connect failed, batch aborted", "error", err)
			wailsruntime.EventsEmit(a.ctx, "batch:fatal-error", err.Error())
			return
		}
		defer adapter.Close()
		sender = adapter
	case providerGmail:
		sender = gmail.NewAdapter(gmailEmail, gmailAppPassword)
	}

	var final application.Progress
	application.RunBatch(ctx, &a.currentBatch, sender, func(p application.Progress) {
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
	if a.activeProvider == providerNone {
		a.sendMu.Unlock()
		return errNoAccountSelected
	}
	retried := application.PrepareRetry(&a.currentBatch)
	if retried == 0 {
		a.sendMu.Unlock()
		return errNothingToRetry
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelSend = cancel
	a.sending = true
	provider, account, email, appPassword := a.activeProvider, a.selectedAccount, a.gmailEmail, a.gmailAppPassword
	a.sendMu.Unlock()

	a.log.Info("retrying failed items", "count", retried)
	go a.runBatchWorker(ctx, provider, account, email, appPassword)
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
var errNoAccountSelected = fmt.Errorf("selecciona y conecta una cuenta de Outlook o Gmail antes de enviar")
var errInvalidRecipient = fmt.Errorf("la dirección de correo del destinatario no es válida")
