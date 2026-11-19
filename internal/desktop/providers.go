package desktop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"despacho/app/application"
	"despacho/app/domain"
	"despacho/app/infrastructure/demomail"
	"despacho/app/infrastructure/outlook"
	"despacho/app/infrastructure/smtpmail"
)

// ProviderKind identifies the active transport.
type ProviderKind string

const (
	ProviderNone    ProviderKind = ""
	ProviderOutlook ProviderKind = "outlook"
	ProviderSMTP    ProviderKind = "smtp"
	ProviderDemo    ProviderKind = "demo"
)

// Limits are the safety settings applied to every batch.
type Limits struct {
	DelaySeconds int  `json:"delaySeconds"`
	MaxPerBatch  int  `json:"maxPerBatch"`
	KeepAwake    bool `json:"keepAwake"`
}

// ProviderState is what the UI shows about the current connection.
type ProviderState struct {
	Kind       string `json:"kind"`
	Account    string `json:"account"`
	DailyLimit int    `json:"dailyLimit"`
	Limits     Limits `json:"limits"`
}

type OutlookAccount struct {
	DisplayName string `json:"displayName"`
	SmtpAddress string `json:"smtpAddress"`
}

type OutlookStatus struct {
	Available bool             `json:"available"`
	Detail    string           `json:"detail"`
	Accounts  []OutlookAccount `json:"accounts"`
}

type SMTPSettings struct {
	Preset   string `json:"preset"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Security string `json:"security"`
	Username string `json:"username"`
	Password string `json:"password"`
	FromName string `json:"fromName"`
}

type SMTPPreset struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Security   string `json:"security"`
	DailyLimit int    `json:"dailyLimit"`
	HelpURL    string `json:"helpUrl"`
	Help       string `json:"help"`
}

type ConnectionResult struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// ProviderService owns which account mail is sent from. Credentials live in
// memory only, for the current session — never written to disk or logged.
type ProviderService struct {
	log *slog.Logger

	mu             sync.Mutex
	kind           ProviderKind
	outlookAccount string
	smtp           smtpmail.Config
	smtpPreset     string
	demoDir        string
	limits         Limits
}

func NewProviderService(log *slog.Logger) *ProviderService {
	return &ProviderService{
		log:    log,
		limits: Limits{DelaySeconds: 2, MaxPerBatch: 500, KeepAwake: true},
	}
}

// State returns the current connection and limits.
func (p *ProviderService) State() ProviderState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stateLocked()
}

func (p *ProviderService) stateLocked() ProviderState {
	st := ProviderState{Kind: string(p.kind), Limits: p.limits}
	switch p.kind {
	case ProviderOutlook:
		st.Account = p.outlookAccount
	case ProviderSMTP:
		st.Account = p.smtp.Username
		if preset, ok := findPreset(p.smtpPreset); ok {
			st.DailyLimit = preset.DailyLimit
		}
	case ProviderDemo:
		st.Account = p.demoDir
	}
	return st
}

// CheckOutlook probes Outlook Classic over COM and lists its accounts. It
// sends nothing and is safe to call repeatedly.
func (p *ProviderService) CheckOutlook() OutlookStatus {
	result := outlook.TestConnection()
	status := OutlookStatus{Available: result.Available, Detail: result.Detail}
	if result.Available {
		accounts, err := outlook.ListAccounts()
		if err != nil {
			status.Detail = err.Error()
		}
		for _, a := range accounts {
			status.Accounts = append(status.Accounts, OutlookAccount{DisplayName: a.DisplayName, SmtpAddress: a.SmtpAddress})
		}
	}
	p.log.Info("outlook check", "available", status.Available, "accounts", len(status.Accounts))
	return status
}

// UseOutlook selects an Outlook account as the sender. The account must be
// chosen explicitly — the app never falls back to Outlook's default.
func (p *ProviderService) UseOutlook(account string) (ProviderState, error) {
	if account == "" {
		return ProviderState{}, errors.New("elige la cuenta de Outlook desde la que se enviará")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.kind, p.outlookAccount = ProviderOutlook, account
	p.log.Info("provider selected", "kind", p.kind, "account", account)
	return p.stateLocked(), nil
}

func (p *ProviderService) SMTPPresets() []SMTPPreset {
	out := make([]SMTPPreset, len(smtpmail.Presets))
	for i, pr := range smtpmail.Presets {
		out[i] = SMTPPreset{ID: pr.ID, Label: pr.Label, Host: pr.Host, Port: pr.Port, Security: string(pr.Security), DailyLimit: pr.DailyLimit, HelpURL: pr.HelpURL, Help: pr.Help}
	}
	return out
}

// TestSMTP authenticates against the server without sending anything.
func (p *ProviderService) TestSMTP(s SMTPSettings) ConnectionResult {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := smtpmail.TestConnection(ctx, s.config()); err != nil {
		p.log.Info("smtp test failed", "host", s.Host, "error", err)
		return ConnectionResult{Detail: err.Error()}
	}
	p.log.Info("smtp test ok", "host", s.Host)
	return ConnectionResult{OK: true, Detail: "Conexión correcta con " + s.Host}
}

func (p *ProviderService) UseSMTP(s SMTPSettings) (ProviderState, error) {
	cfg := s.config()
	if err := cfg.Validate(); err != nil {
		return ProviderState{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.kind, p.smtp, p.smtpPreset = ProviderSMTP, cfg, s.Preset
	p.log.Info("provider selected", "kind", p.kind, "host", cfg.Host)
	return p.stateLocked(), nil
}

// UseDemo makes every "send" write an .eml file into dir instead. An empty
// dir means Documents\Despacho\salida_demo.
func (p *ProviderService) UseDemo(dir string) (ProviderState, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ProviderState{}, err
		}
		dir = filepath.Join(home, "Documents", "Despacho", "salida_demo")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.kind, p.demoDir = ProviderDemo, dir
	p.log.Info("provider selected", "kind", p.kind, "dir", dir)
	return p.stateLocked(), nil
}

func (p *ProviderService) Disconnect() ProviderState {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.kind, p.smtp, p.outlookAccount = ProviderNone, smtpmail.Config{}, ""
	return p.stateLocked()
}

func (p *ProviderService) SetLimits(l Limits) (ProviderState, error) {
	if l.DelaySeconds < 0 || l.DelaySeconds > 120 {
		return ProviderState{}, errors.New("la pausa debe estar entre 0 y 120 segundos")
	}
	if l.MaxPerBatch < 1 || l.MaxPerBatch > 10000 {
		return ProviderState{}, errors.New("el tope por lote debe estar entre 1 y 10000")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.limits = l
	return p.stateLocked(), nil
}

// SendTest sends one fixed test message through the active provider.
func (p *ProviderService) SendTest(to string) error {
	if !application.IsValidEmail(to) {
		return errors.New("la dirección de prueba no es válida")
	}
	s, err := p.session()
	if err != nil {
		return err
	}
	err = s.withSender(func(sender domain.MailSender) error {
		return sender.Send(context.Background(), domain.Message{
			To:      to,
			Subject: "Prueba de conexión — Despacho",
			Body:    "Este es un correo de prueba enviado desde Despacho.\n\nSi lo recibiste, la cuenta está lista para envíos masivos.",
		})
	})
	if err != nil {
		p.log.Error("test send failed", "kind", s.kind, "error", err)
		return err
	}
	p.log.Info("test send ok", "kind", s.kind)
	return nil
}

// session is an immutable snapshot of the provider configuration, taken
// under the lock when a batch starts. The worker uses it without holding
// any lock, and later UI changes cannot affect a batch already running.
type session struct {
	kind      ProviderKind
	account   string
	limits    Limits
	pinThread bool
	open      func() (domain.MailSender, func(), error)
}

func (p *ProviderService) session() (session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := session{kind: p.kind, limits: p.limits}
	switch p.kind {
	case ProviderOutlook:
		account := p.outlookAccount
		s.account = account
		// COM is apartment-threaded: Connect and every Send must happen on
		// the same OS thread, so the caller pins its goroutine
		// (runtime.LockOSThread) before calling open.
		s.pinThread = true
		s.open = func() (domain.MailSender, func(), error) {
			a, err := outlook.Connect(account)
			if err != nil {
				return nil, nil, err
			}
			return a, a.Close, nil
		}
	case ProviderSMTP:
		cfg := p.smtp
		s.account = cfg.Username
		s.open = func() (domain.MailSender, func(), error) {
			return smtpmail.New(cfg), func() {}, nil
		}
	case ProviderDemo:
		dir := p.demoDir
		s.account = dir
		s.open = func() (domain.MailSender, func(), error) {
			a, err := demomail.New(dir, mail.Address{Name: "Despacho (demo)", Address: "demo@example.com"})
			if err != nil {
				return nil, nil, err
			}
			a.Delay = 600 * time.Millisecond // looks like real network latency on camera
			return a, func() {}, nil
		}
	default:
		return session{}, errors.New("conecta una cuenta de envío antes de continuar")
	}
	return s, nil
}

// withSender opens the sender, runs fn and closes it, pinning the OS thread
// when the transport requires it.
func (s session) withSender(fn func(domain.MailSender) error) error {
	if s.pinThread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	sender, closeFn, err := s.open()
	if err != nil {
		return err
	}
	defer closeFn()
	return fn(sender)
}

func (s SMTPSettings) config() smtpmail.Config {
	return smtpmail.Config{
		Host:     s.Host,
		Port:     s.Port,
		Security: smtpmail.Security(s.Security),
		Username: s.Username,
		Password: s.Password,
		FromName: s.FromName,
	}
}

func findPreset(id string) (smtpmail.Preset, bool) {
	for _, pr := range smtpmail.Presets {
		if pr.ID == id {
			return pr, true
		}
	}
	return smtpmail.Preset{}, false
}

func (s session) label() string {
	switch s.kind {
	case ProviderOutlook:
		return "Outlook (" + s.account + ")"
	case ProviderSMTP:
		return s.account
	case ProviderDemo:
		return "modo demo"
	}
	return fmt.Sprint(s.kind)
}
