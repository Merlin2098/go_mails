package desktop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"despacho/app/application"
	"despacho/app/domain"
	"despacho/app/infrastructure/filesystem"
	"despacho/app/infrastructure/power"
	"despacho/app/infrastructure/spreadsheet"
)

// Events emitted while a batch runs.
const (
	eventItem  = "batch:item"
	eventLog   = "batch:log"
	eventDone  = "batch:done"
	eventFatal = "batch:fatal"
)

type Mapping struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	CC    string `json:"cc"`
	Key   string `json:"key"`
}

type TemplateDTO struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	HTML    bool   `json:"html"`
}

type AttachmentRuleDTO struct {
	Mode      string `json:"mode"`
	Folder    string `json:"folder"`
	Pattern   string `json:"pattern"`
	Column    string `json:"column"`
	Recursive bool   `json:"recursive"`
}

type CampaignRequest struct {
	Mapping     Mapping           `json:"mapping"`
	Template    TemplateDTO       `json:"template"`
	Attachments AttachmentRuleDTO `json:"attachments"`
}

type TableData struct {
	Path      string       `json:"path"`
	FileName  string       `json:"fileName"`
	Headers   []string     `json:"headers"`
	Rows      [][]string   `json:"rows"`
	Suggested Mapping      `json:"suggested"`
	Template  *TemplateDTO `json:"template"` // from the optional "Mensaje" sheet
}

type ItemDTO struct {
	Index       int      `json:"index"`
	Row         int      `json:"row"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Attachments []string `json:"attachments"`
	Status      string   `json:"status"`
	Field       string   `json:"field"`
	Error       string   `json:"error"`
}

type ValidationResult struct {
	Error           string    `json:"error"` // table/mapping problem; items are empty when set
	TemplateError   string    `json:"templateError"`
	AttachmentError string    `json:"attachmentError"`
	Total           int       `json:"total"`
	Ready           int       `json:"ready"`
	Invalid         int       `json:"invalid"`
	Sent            int       `json:"sent"`
	Failed          int       `json:"failed"`
	Items           []ItemDTO `json:"items"`
	Running         bool      `json:"running"`
}

type MessagePreview struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	CC          string   `json:"cc"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	HTML        bool     `json:"html"`
	Attachments []string `json:"attachments"`
	Error       string   `json:"error"`
}

type ItemEvent struct {
	Item      ItemDTO `json:"item"`
	Total     int     `json:"total"`
	Processed int     `json:"processed"`
	Sent      int     `json:"sent"`
	Failed    int     `json:"failed"`
}

type LogLine struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type DoneEvent struct {
	Sent      int `json:"sent"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

// CampaignService loads the recipient table, validates the campaign and runs
// the send. All mutable state is guarded by mu.
//
// The original single-App version had data races: validation could replace
// the batch while the worker was writing to it, and the report exporter read
// it mid-send. Here the worker sends a private clone (domain.Batch.Clone) and
// copies each item's new state back under the lock, while Validate refuses
// to run during a send.
type CampaignService struct {
	ctx       context.Context
	log       *slog.Logger
	providers *ProviderService

	mu       sync.Mutex
	table    domain.Table
	path     string
	batch    domain.Batch
	sendable bool // the last validation had no template/attachment errors
	running  bool
	cancel   context.CancelFunc
}

func NewCampaignService(log *slog.Logger, providers *ProviderService) *CampaignService {
	return &CampaignService{log: log, providers: providers}
}

func (c *CampaignService) startup(ctx context.Context) { c.ctx = ctx }

// LoadData reads an .xlsx/.csv and suggests which columns play each role.
func (c *CampaignService) LoadData(path string) (TableData, error) {
	table, err := spreadsheet.ReadTable(path)
	if err != nil {
		return TableData{}, err
	}
	data := TableData{
		Path:      path,
		FileName:  filepath.Base(path),
		Headers:   table.Headers,
		Rows:      table.Rows,
		Suggested: suggestMapping(table),
	}
	if tpl, found, err := spreadsheet.ReadMessageTemplate(path); err == nil && found {
		data.Template = &TemplateDTO{Subject: tpl.Subject, Body: tpl.Body, HTML: tpl.HTML}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return TableData{}, errBatchRunning
	}
	c.table, c.path, c.batch, c.sendable = table, path, domain.Batch{}, false
	c.log.Info("data loaded", "file", data.FileName, "rows", len(table.Rows), "columns", len(table.Headers))
	return data, nil
}

// Validate builds the batch from the current table and settings. Campaign
// problems come back in ValidationResult.Error (not as a Go error) because
// they are an expected, displayable state while the user is still typing.
func (c *CampaignService) Validate(req CampaignRequest) ValidationResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return c.resultLocked()
	}
	if len(c.table.Headers) == 0 {
		return ValidationResult{Error: "Carga primero el archivo de destinatarios."}
	}

	// Rows, template and attachments are validated independently so each
	// step of the wizard can show its own problems. A template or rule error
	// does not hide row errors: rows are then checked with a neutral
	// template / no attachments, and the batch is marked not sendable.
	var tplErr, ruleErr string
	tpl := req.Template.template()
	if err := application.CheckTemplate(c.table.Headers, tpl); err != nil {
		tplErr = capitalize(err.Error())
		tpl = domain.Template{Subject: "-"}
	}

	rule := req.Attachments.rule()
	var docs application.DocumentIndex
	if rule.Mode != domain.AttachNone && rule.Folder != "" {
		idx, err := filesystem.BuildIndex(rule.Folder, rule.Recursive)
		if err != nil {
			ruleErr = "No se pudo leer la carpeta de documentos: " + err.Error()
		} else {
			docs = idx
		}
	}
	if ruleErr == "" {
		if err := application.CheckRule(c.table.Headers, rule, docs); err != nil {
			ruleErr = capitalize(err.Error())
		}
	}
	if ruleErr != "" {
		rule, docs = domain.AttachmentRule{Mode: domain.AttachNone}, nil
	}

	batch, err := application.BuildBatch(c.table, req.Mapping.columns(), tpl, rule, docs)
	if err != nil {
		c.batch, c.sendable = domain.Batch{}, false
		return ValidationResult{Error: capitalize(err.Error())}
	}
	keepDelivered(&batch, c.batch)
	c.batch = batch
	c.sendable = tplErr == "" && ruleErr == ""
	res := c.resultLocked()
	res.TemplateError, res.AttachmentError = tplErr, ruleErr
	return res
}

// keepDelivered carries SENT status over from the previous validation of the
// same table: fixing a typo in the template after a send must never turn
// into re-sending to everyone who already got the message. Loading a new
// file (LoadData) is the explicit way to start from scratch.
func keepDelivered(next *domain.Batch, prev domain.Batch) {
	sent := map[int]string{}
	for _, item := range prev.Items {
		if item.Status == domain.StatusSent {
			sent[item.Recipient.Row] = item.Recipient.Email
		}
	}
	for i := range next.Items {
		item := &next.Items[i]
		if email, ok := sent[item.Recipient.Row]; ok && email == item.Recipient.Email && item.Status == domain.StatusReady {
			item.Status = domain.StatusSent
		}
	}
}

// Snapshot returns the current batch state, e.g. after the UI remounts.
func (c *CampaignService) Snapshot() ValidationResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resultLocked()
}

func (c *CampaignService) resultLocked() ValidationResult {
	s := c.batch.Summary()
	res := ValidationResult{Total: s.Total, Ready: s.Ready, Invalid: s.Invalid, Sent: s.Sent, Failed: s.Failed + s.Cancelled, Running: c.running}
	res.Items = make([]ItemDTO, len(c.batch.Items))
	for i, item := range c.batch.Items {
		res.Items[i] = toItemDTO(i, item)
	}
	return res
}

// PreviewMessage renders the message for table row index (0-based) exactly
// as it would be sent, attachments included.
func (c *CampaignService) PreviewMessage(req CampaignRequest, index int) MessagePreview {
	c.mu.Lock()
	table := c.table
	c.mu.Unlock()

	if index < 0 || index >= len(table.Rows) {
		return MessagePreview{Error: "No hay filas para previsualizar."}
	}
	one := domain.Table{Headers: table.Headers, Rows: [][]string{table.Rows[index]}}
	rule := req.Attachments.rule()
	var docs application.DocumentIndex
	if rule.Mode != domain.AttachNone && rule.Folder != "" {
		if idx, err := filesystem.BuildIndex(rule.Folder, rule.Recursive); err == nil {
			docs = idx
		}
	}
	if docs == nil || application.CheckRule(table.Headers, rule, docs) != nil {
		rule, docs = domain.AttachmentRule{Mode: domain.AttachNone}, nil
	}

	// The row is resolved with a neutral template so the preview keeps
	// rendering while the user is still typing (empty subject, a half-written
	// {{variable}}); the real template is rendered right after.
	batch, err := application.BuildBatch(one, req.Mapping.columns(), domain.Template{Subject: "-"}, rule, docs)
	if err != nil {
		return MessagePreview{Error: capitalize(err.Error())}
	}
	item := batch.Items[0]
	tpl := req.Template.template()
	msg := tpl.Render(item.Recipient, item.Attachments)

	from := ""
	if st := c.providers.State(); st.Account != "" && st.Kind != string(ProviderDemo) {
		from = st.Account
	}
	preview := MessagePreview{
		From: from, To: msg.To, CC: msg.CC, Subject: strings.TrimSpace(msg.Subject),
		Body: msg.Body, HTML: msg.HTML, Attachments: baseNames(msg.Attachments),
	}
	if item.Status == domain.StatusInvalid {
		preview.Error = item.ValidationError
	}
	return preview
}

// StartBatch asks for native confirmation and starts sending every READY
// item on a background goroutine. Returns false if the user declined.
func (c *CampaignService) StartBatch() (bool, error) {
	return c.start(nil)
}

// RetryFailed re-sends only FAILED and CANCELLED items — never the ones
// already delivered.
func (c *CampaignService) RetryFailed() (bool, error) {
	return c.start(application.PrepareRetry)
}

func (c *CampaignService) start(prepare func(*domain.Batch) int) (bool, error) {
	s, err := c.providers.session()
	if err != nil {
		return false, err
	}

	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return false, errBatchRunning
	}
	if !c.sendable {
		c.mu.Unlock()
		return false, errors.New("revisa la plantilla y los adjuntos: la campaña tiene errores")
	}
	pending := 0
	for _, item := range c.batch.Items {
		if item.Status == domain.StatusReady || (prepare != nil && (item.Status == domain.StatusFailed || item.Status == domain.StatusCancelled)) {
			pending++
		}
	}
	c.mu.Unlock()

	if pending == 0 {
		return false, errors.New("no hay correos pendientes de enviar")
	}
	if pending > s.limits.MaxPerBatch {
		return false, fmt.Errorf("el lote tiene %d correos y el tope configurado es %d", pending, s.limits.MaxPerBatch)
	}

	// The confirmation is a native dialog owned by Go, so a bug in the
	// frontend can never skip it. Demo mode sends nothing, so it has nothing
	// to confirm.
	if s.kind != ProviderDemo {
		answer, err := wailsruntime.MessageDialog(c.ctx, wailsruntime.MessageDialogOptions{
			Type:    wailsruntime.QuestionDialog,
			Title:   "Confirmar envío",
			Message: fmt.Sprintf("Se enviarán %d correos desde %s.\n\n¿Continuar?", pending, s.label()),
		})
		if err != nil {
			return false, err
		}
		if answer != "Yes" && answer != "Sí" && answer != "Ok" {
			return false, nil
		}
	}

	c.mu.Lock()
	if c.running { // someone else started meanwhile
		c.mu.Unlock()
		return false, errBatchRunning
	}
	if prepare != nil {
		prepare(&c.batch)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.running, c.cancel = true, cancel
	work := c.batch.Clone()
	c.mu.Unlock()

	go c.worker(ctx, work, s)
	return true, nil
}

// worker runs on its own goroutine for the whole batch.
func (c *CampaignService) worker(ctx context.Context, work domain.Batch, s session) {
	defer func() {
		c.mu.Lock()
		c.running, c.cancel = false, nil
		c.mu.Unlock()
	}()

	if s.pinThread {
		// See session.pinThread: Outlook COM requires every call on the
		// same OS thread, and Go would otherwise migrate this goroutine.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	if s.limits.KeepAwake {
		release := power.KeepAwake()
		defer release()
	}

	sender, closeFn, err := s.open()
	if err != nil {
		c.emitLog("ERROR", "no se pudo abrir el proveedor: "+err.Error())
		wailsruntime.EventsEmit(c.ctx, eventFatal, err.Error())
		return
	}
	defer closeFn()

	c.emitLog("INFO", fmt.Sprintf("batch starting provider=%s delay=%ds", s.kind, s.limits.DelaySeconds))

	res := application.RunBatch(ctx, &work, sender, application.RunOptions{
		Delay: time.Duration(s.limits.DelaySeconds) * time.Second,
		OnEvent: func(e application.Event) {
			c.mu.Lock()
			c.batch.Items[e.Index] = e.Item
			c.mu.Unlock()

			wailsruntime.EventsEmit(c.ctx, eventItem, ItemEvent{
				Item: toItemDTO(e.Index, e.Item), Total: e.Total, Processed: e.Processed, Sent: e.Sent, Failed: e.Failed,
			})
			c.logItem(e.Item)
		},
	})

	c.emitLog("INFO", fmt.Sprintf("batch finished sent=%d failed=%d cancelled=%d", res.Sent, res.Failed, res.Cancelled))
	wailsruntime.EventsEmit(c.ctx, eventDone, DoneEvent{Sent: res.Sent, Failed: res.Failed, Cancelled: res.Cancelled})
}

// logItem writes operational metadata only — row, key and file count, never
// the message body.
func (c *CampaignService) logItem(item domain.BatchItem) {
	id := item.Recipient.Key
	if id == "" {
		id = fmt.Sprintf("fila%d", item.Recipient.Row)
	}
	switch item.Status {
	case domain.StatusSent:
		c.emitLog("INFO", fmt.Sprintf("item sent id=%s files=%d", id, len(item.Attachments)))
	case domain.StatusFailed:
		c.emitLog("ERROR", fmt.Sprintf("item failed id=%s err=%q", id, item.SendError))
	case domain.StatusCancelled:
		c.emitLog("WARN", fmt.Sprintf("item cancelled id=%s", id))
	}
}

func (c *CampaignService) emitLog(level, msg string) {
	switch level {
	case "ERROR":
		c.log.Error(msg)
	case "WARN":
		c.log.Warn(msg)
	default:
		c.log.Info(msg)
	}
	wailsruntime.EventsEmit(c.ctx, eventLog, LogLine{Time: time.Now().Format("15:04:05"), Level: level, Message: msg})
}

// CancelBatch requests cooperative cancellation: the message being sent
// finishes, the rest are marked CANCELLED.
func (c *CampaignService) CancelBatch() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

// ExportReport saves the current batch result as .xlsx. Returns "" if the
// user cancels the dialog.
func (c *CampaignService) ExportReport() (string, error) {
	c.mu.Lock()
	snapshot := c.batch.Clone()
	c.mu.Unlock()
	if len(snapshot.Items) == 0 {
		return "", errors.New("no hay un lote para exportar")
	}

	path, err := wailsruntime.SaveFileDialog(c.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Guardar reporte de envío",
		DefaultFilename: "reporte_" + time.Now().Format("2006-01-02_1504") + ".xlsx",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := spreadsheet.WriteReport(snapshot, path); err != nil {
		c.log.Error("export report failed", "error", err)
		return "", err
	}
	c.log.Info("report exported", "file", filepath.Base(path))
	return path, nil
}

// table returns a copy of the loaded table for other services.
func (c *CampaignService) currentTable() domain.Table {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.table
}

var errBatchRunning = errors.New("hay un envío en curso")

func (m Mapping) columns() application.ColumnMapping {
	return application.ColumnMapping{Email: m.Email, Name: m.Name, CC: m.CC, Key: m.Key}
}

func (t TemplateDTO) template() domain.Template {
	return domain.Template{Subject: t.Subject, Body: t.Body, HTML: t.HTML}
}

func (r AttachmentRuleDTO) rule() domain.AttachmentRule {
	mode := domain.AttachmentMode(r.Mode)
	if mode == "" {
		mode = domain.AttachNone
	}
	return domain.AttachmentRule{Mode: mode, Folder: r.Folder, Pattern: r.Pattern, Column: r.Column, Recursive: r.Recursive}
}

func toItemDTO(i int, item domain.BatchItem) ItemDTO {
	return ItemDTO{
		Index:       i,
		Row:         item.Recipient.Row,
		Email:       item.Recipient.Email,
		Name:        item.Recipient.Name,
		Key:         item.Recipient.Key,
		Attachments: baseNames(item.Attachments),
		Status:      string(item.Status),
		Field:       item.ValidationField,
		Error:       firstNonEmpty(item.ValidationError, item.SendError),
	}
}

func baseNames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// suggestMapping guesses column roles from common header names, falling
// back to the first column whose values look like email addresses.
func suggestMapping(t domain.Table) Mapping {
	pick := func(candidates ...string) string {
		for _, cand := range candidates {
			for _, h := range t.Headers {
				if domain.NormalizeKey(h) == cand {
					return h
				}
			}
		}
		return ""
	}
	m := Mapping{
		Email: pick("correo", "email", "e-mail", "mail", "correo electrónico", "correo electronico"),
		Name:  pick("nombre", "nombres", "name", "nombre completo"),
		CC:    pick("cc", "copia"),
		Key:   pick("documento_id", "id", "dni", "codigo", "código", "clave"),
	}
	if m.Email == "" && len(t.Rows) > 0 {
		for i, h := range t.Headers {
			if i < len(t.Rows[0]) && strings.Contains(t.Rows[0][i], "@") {
				m.Email = h
				break
			}
		}
	}
	return m
}
