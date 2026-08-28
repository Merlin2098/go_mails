package gmail

import (
	"context"
	"fmt"
	"net/smtp"

	"boleta-mailer/app/domain"
)

// smtpHost/smtpPort are Gmail's fixed SMTP submission endpoint — Gmail does
// not let a caller point at a different server, so unlike outlook.Adapter
// there is no "which account is this" ambiguity to resolve: the App
// Password itself identifies the sending account.
const (
	smtpHost = "smtp.gmail.com"
	smtpPort = "587"
	smtpAddr = smtpHost + ":" + smtpPort
)

// Adapter implements domain.MailSender by submitting mail over SMTP to
// Gmail, authenticated with a Google "App Password" (a 16-character
// password generated at myaccount.google.com/apppasswords) instead of
// OAuth. This trades away the multi-account/browser-login flow described
// in docs/specs/POC_Gmail_Desktop.md for something that needs no Google
// Cloud project at all — the client explicitly asked to avoid managing one.
//
// Not safe for concurrent use, matching outlook.Adapter: the batch worker
// that owns an Adapter should call Send from a single goroutine. Unlike
// COM, nothing here requires that goroutine to be pinned to one OS
// thread — net/smtp has no apartment-threading equivalent.
type Adapter struct {
	email       string
	appPassword string
}

var _ domain.MailSender = (*Adapter)(nil)

// NewAdapter builds an Adapter for the given Gmail address and App
// Password. It performs no network I/O — call TestConnection first if you
// need to validate the credentials before committing to a batch.
func NewAdapter(email, appPassword string) *Adapter {
	return &Adapter{email: email, appPassword: appPassword}
}

// Send renders message as a MIME email (mime.go) and submits it over
// STARTTLS SMTP. ctx is checked before starting — cancellation takes effect
// between items, not mid-send, matching outlook.Adapter.Send.
func (a *Adapter) Send(ctx context.Context, message domain.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	raw, err := buildMessage(a.email, message)
	if err != nil {
		return err
	}

	auth := smtp.PlainAuth("", a.email, a.appPassword, smtpHost)
	if err := smtp.SendMail(smtpAddr, auth, a.email, []string{message.To}, raw); err != nil {
		return fmt.Errorf("Gmail rechazó el envío: %w", err)
	}
	return nil
}
