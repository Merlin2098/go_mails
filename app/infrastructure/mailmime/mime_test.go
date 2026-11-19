package mailmime

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"despacho/app/domain"
)

var sender = mail.Address{Name: "Equipo Despacho", Address: "envios@example.com"}

func TestBuild_PlainNoAttachments(t *testing.T) {
	raw, err := Build(sender, domain.Message{
		To: "ana@example.com", CC: "a@x.com; b@y.com", Subject: "Tu certificado — Go", Body: "Hola Ana,\nañadimos esto.",
	})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	dec := new(mime.WordDecoder)
	subject, _ := dec.DecodeHeader(msg.Header.Get("Subject"))
	if subject != "Tu certificado — Go" {
		t.Errorf("subject = %q", subject)
	}
	if got := msg.Header.Get("Cc"); got != "a@x.com, b@y.com" {
		t.Errorf("cc = %q", got)
	}
	if !strings.HasPrefix(msg.Header.Get("Content-Type"), "text/plain") {
		t.Errorf("content-type = %q", msg.Header.Get("Content-Type"))
	}
	if msg.Header.Get("Message-Id") == "" || msg.Header.Get("Date") == "" {
		t.Error("missing Message-Id or Date")
	}
}

func TestBuild_HTMLWithAttachments(t *testing.T) {
	dir := t.TempDir()
	big := bytes.Repeat([]byte("0123456789"), 100) // forces several base64 lines
	a := filepath.Join(dir, "Diego Ramírez.pdf")
	b := filepath.Join(dir, "anexo.txt")
	_ = os.WriteFile(a, big, 0o644)
	_ = os.WriteFile(b, []byte("hola"), 0o644)

	raw, err := Build(sender, domain.Message{
		To: "d@example.com", Subject: "S", Body: "<p>Hola</p>", HTML: true, Attachments: []string{a, b},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("line longer than RFC 5322 limit: %d", len(line))
		}
	}

	msg, _ := mail.ReadMessage(bytes.NewReader(raw))
	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])

	body, _ := mr.NextPart()
	if !strings.HasPrefix(body.Header.Get("Content-Type"), "text/html") {
		t.Errorf("first part = %q", body.Header.Get("Content-Type"))
	}

	att, _ := mr.NextPart()
	if att.FileName() != "Diego Ramírez.pdf" {
		t.Errorf("filename = %q", att.FileName())
	}
	// multipart.Part decodes quoted-printable automatically but not base64.
	encoded, _ := io.ReadAll(att)
	if !bytes.Contains(encoded, []byte("\r\n")) {
		t.Error("base64 should be wrapped")
	}

	second, _ := mr.NextPart()
	if second.FileName() != "anexo.txt" {
		t.Errorf("second filename = %q", second.FileName())
	}
}

func TestBuild_MissingAttachment(t *testing.T) {
	_, err := Build(sender, domain.Message{To: "x@example.com", Attachments: []string{"nope.pdf"}})
	if err == nil {
		t.Error("expected error")
	}
}
