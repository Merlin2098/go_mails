// Package smtpmail implements domain.MailSender over plain SMTP, with
// presets for common providers. Gmail is just one preset: an App Password
// (myaccount.google.com/apppasswords) authenticates without OAuth or a
// Google Cloud project.
package smtpmail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"despacho/app/domain"
	"despacho/app/infrastructure/mailmime"
)

// Security is how the TCP connection is protected.
type Security string

const (
	// StartTLS connects in plain text and upgrades (usually port 587).
	StartTLS Security = "starttls"
	// ImplicitTLS speaks TLS from the first byte (usually port 465).
	ImplicitTLS Security = "tls"
)

// Config is everything needed to submit mail to one SMTP account.
type Config struct {
	Host     string
	Port     int
	Security Security
	Username string
	Password string
	FromName string
}

// Preset is a known provider's fixed server settings.
type Preset struct {
	ID         string
	Label      string
	Host       string
	Port       int
	Security   Security
	DailyLimit int    // 0 = unknown / not published
	HelpURL    string // where to create the credential the server expects
	Help       string
}

// Presets lists the providers offered in the UI. "custom" leaves every field
// to the user.
var Presets = []Preset{
	{
		ID: "gmail", Label: "Gmail", Host: "smtp.gmail.com", Port: 587, Security: StartTLS,
		DailyLimit: 500,
		HelpURL:    "https://myaccount.google.com/apppasswords",
		Help:       "Usa una contraseña de aplicación de 16 caracteres (requiere verificación en 2 pasos).",
	},
	{
		ID: "zoho", Label: "Zoho Mail", Host: "smtp.zoho.com", Port: 587, Security: StartTLS,
		HelpURL: "https://accounts.zoho.com/home#security/app_password",
		Help:    "Si tienes 2FA activo, genera una contraseña específica de aplicación.",
	},
	{
		ID: "custom", Label: "Otro servidor SMTP", Port: 587, Security: StartTLS,
		Help: "Datos que te entrega tu proveedor de correo o hosting.",
	},
}

const (
	dialTimeout    = 15 * time.Second
	sessionTimeout = 2 * time.Minute
)

// Validate reports configuration mistakes before any network I/O.
func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.Host) == "":
		return errors.New("falta el servidor SMTP")
	case c.Port <= 0 || c.Port > 65535:
		return errors.New("el puerto SMTP no es válido")
	case c.Security != StartTLS && c.Security != ImplicitTLS:
		return errors.New("elige STARTTLS o TLS")
	}
	if _, err := mail.ParseAddress(c.Username); err != nil {
		return errors.New("el usuario debe ser una dirección de correo")
	}
	if c.Password == "" {
		return errors.New("falta la contraseña")
	}
	return nil
}

// From is the sender address. Most providers reject a From that does not
// match the authenticated account, so it is not configurable separately.
func (c Config) From() mail.Address {
	return mail.Address{Name: c.FromName, Address: c.Username}
}

// Adapter implements domain.MailSender. Each Send opens its own connection:
// with a pause of seconds between messages, a long-lived session would just
// be closed by the server's idle timeout anyway.
type Adapter struct {
	cfg Config
}

var _ domain.MailSender = (*Adapter)(nil)

func New(cfg Config) *Adapter { return &Adapter{cfg: cfg} }

func (a *Adapter) Send(ctx context.Context, message domain.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := mailmime.Build(a.cfg.From(), message)
	if err != nil {
		return err
	}
	rcpts := append([]string{message.To}, domain.SplitAddresses(message.CC)...)

	return withClient(ctx, a.cfg, func(c *smtp.Client) error {
		if err := c.Mail(a.cfg.Username); err != nil {
			return fmt.Errorf("el servidor rechazó el remitente: %w", err)
		}
		for _, r := range rcpts {
			if err := c.Rcpt(r); err != nil {
				return fmt.Errorf("el servidor rechazó el destinatario %s: %w", r, err)
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(raw); err != nil {
			return err
		}
		return w.Close()
	})
}

// TestConnection connects, negotiates TLS and authenticates without sending
// anything (MAIL/RCPT/DATA are never issued).
func TestConnection(ctx context.Context, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	return withClient(ctx, cfg, func(*smtp.Client) error { return nil })
}

// withClient owns the connection lifecycle so Send and TestConnection share
// exactly the same dial/TLS/auth path.
func withClient(ctx context.Context, cfg Config, fn func(*smtp.Client) error) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: dialTimeout}

	var conn net.Conn
	var err error
	if cfg.Security == ImplicitTLS {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("no se pudo conectar a %s: %w", addr, err)
	}
	// A deadline on the whole session: a server that stops answering must
	// fail this item, not freeze the batch worker forever.
	_ = conn.SetDeadline(time.Now().Add(sessionTimeout))

	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("respuesta SMTP inválida de %s: %w", addr, err)
	}
	defer c.Close()

	if cfg.Security == StartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("%s no ofrece STARTTLS; prueba con TLS en el puerto 465", cfg.Host)
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("no se pudo iniciar TLS: %w", err)
		}
	}
	if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
		return fmt.Errorf("credenciales rechazadas por %s: %w", cfg.Host, err)
	}
	if err := fn(c); err != nil {
		return err
	}
	return c.Quit()
}
