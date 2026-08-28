package gmail

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"boleta-mailer/app/domain"
)

func TestBuildMessageWithoutAttachment(t *testing.T) {
	message := domain.Message{
		To:      "destinatario@example.com",
		Subject: "Asunto de prueba",
		Body:    "Cuerpo del correo",
	}

	raw, err := buildMessage("remitente@gmail.com", message)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("mail.ReadMessage: %v", err)
	}

	if got := parsed.Header.Get("From"); got != "remitente@gmail.com" {
		t.Errorf("From = %q, want remitente@gmail.com", got)
	}
	if got := parsed.Header.Get("To"); got != message.To {
		t.Errorf("To = %q, want %q", got, message.To)
	}
	if ct := parsed.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", ct)
	}

	body, err := io.ReadAll(parsed.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != message.Body {
		t.Errorf("body = %q, want %q", body, message.Body)
	}
}

func TestBuildMessageWithAttachment(t *testing.T) {
	dir := t.TempDir()
	attachmentPath := filepath.Join(dir, "boleta.pdf")
	attachmentContent := []byte("%PDF-1.4 contenido de prueba")
	if err := os.WriteFile(attachmentPath, attachmentContent, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	message := domain.Message{
		To:             "destinatario@example.com",
		Subject:        "Boleta de pago",
		Body:           "Adjunto tu boleta",
		AttachmentPath: attachmentPath,
	}

	raw, err := buildMessage("remitente@gmail.com", message)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("mail.ReadMessage: %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("mediaType = %q, want multipart/mixed", mediaType)
	}

	reader := multipart.NewReader(parsed.Body, params["boundary"])

	bodyPart, err := reader.NextPart()
	if err != nil {
		t.Fatalf("first part: %v", err)
	}
	bodyBytes, err := io.ReadAll(bodyPart)
	if err != nil {
		t.Fatalf("read body part: %v", err)
	}
	if string(bodyBytes) != message.Body {
		t.Errorf("body part = %q, want %q", bodyBytes, message.Body)
	}

	attachmentPart, err := reader.NextPart()
	if err != nil {
		t.Fatalf("second part: %v", err)
	}
	if enc := attachmentPart.Header.Get("Content-Transfer-Encoding"); enc != "base64" {
		t.Errorf("Content-Transfer-Encoding = %q, want base64", enc)
	}
	_, dispParams, err := mime.ParseMediaType(attachmentPart.Header.Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("ParseMediaType (disposition): %v", err)
	}
	if dispParams["filename"] != "boleta.pdf" {
		t.Errorf("filename = %q, want boleta.pdf", dispParams["filename"])
	}

	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, attachmentPart))
	if err != nil {
		t.Fatalf("decode attachment: %v", err)
	}
	if !bytes.Equal(decoded, attachmentContent) {
		t.Errorf("decoded attachment = %q, want %q", decoded, attachmentContent)
	}

	if _, err := reader.NextPart(); err != io.EOF {
		t.Errorf("expected exactly 2 parts, got a third: err=%v", err)
	}
}

func TestBuildMessageMissingAttachment(t *testing.T) {
	message := domain.Message{
		To:             "destinatario@example.com",
		Subject:        "Asunto",
		Body:           "Cuerpo",
		AttachmentPath: filepath.Join(t.TempDir(), "no-existe.pdf"),
	}

	if _, err := buildMessage("remitente@gmail.com", message); err == nil {
		t.Fatal("esperaba error por archivo adjunto inexistente, obtuve nil")
	}
}
