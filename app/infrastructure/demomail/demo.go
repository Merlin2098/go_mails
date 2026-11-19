// Package demomail is a domain.MailSender that never sends anything: each
// message is written to a folder as a standard .eml file, which Outlook,
// Thunderbird or any mail client can open. It exists for demos, screen
// recordings and end-to-end checks without spamming real inboxes.
package demomail

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"despacho/app/domain"
	"despacho/app/infrastructure/mailmime"
)

// Adapter writes one .eml per Send. Not safe for concurrent use — like every
// sender here, it is owned by a single batch worker goroutine.
type Adapter struct {
	dir   string
	from  mail.Address
	count int
	// Delay simulates network latency so a demo "feels" like a real send.
	Delay time.Duration
}

var _ domain.MailSender = (*Adapter)(nil)

// New creates dir if needed.
func New(dir string, from mail.Address) (*Adapter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("no se pudo crear la carpeta de salida: %w", err)
	}
	return &Adapter{dir: dir, from: from}, nil
}

func (a *Adapter) Send(ctx context.Context, message domain.Message) error {
	if a.Delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(a.Delay):
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := mailmime.Build(a.from, message)
	if err != nil {
		return err
	}
	a.count++
	name := fmt.Sprintf("%03d_%s.eml", a.count, safeName(message.To))
	return os.WriteFile(filepath.Join(a.dir, name), raw, 0o644)
}

// safeName keeps an email address usable as a Windows file name.
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
}
