package demomail

import (
	"context"
	"net/mail"
	"os"
	"path/filepath"
	"testing"

	"despacho/app/domain"
)

func TestSend_WritesNumberedEML(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	a, err := New(dir, mail.Address{Address: "envios@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{"ana@example.com", "bruno@example.com"} {
		if err := a.Send(context.Background(), domain.Message{To: to, Subject: "S", Body: "B"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"001_ana@example.com.eml", "002_bruno@example.com.eml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestSend_CancelledDuringDelay(t *testing.T) {
	a, _ := New(t.TempDir(), mail.Address{Address: "e@example.com"})
	a.Delay = 1e9
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Send(ctx, domain.Message{To: "x@example.com"}); err == nil {
		t.Error("expected cancellation error")
	}
}
