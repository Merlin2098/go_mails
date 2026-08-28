// Package gmail implements domain.MailSender against a personal Gmail
// account via SMTP, authenticated with a Google "App Password" instead of
// OAuth — the client explicitly does not want to manage a Google Cloud
// project (docs/specs/POC_Gmail_Desktop.md, revisión). Nothing outside this
// package should know about SMTP, MIME, or App Passwords.
package gmail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"

	"boleta-mailer/app/domain"
)

// buildMessage renders message as a complete RFC 5322 email (headers plus
// body), ready to hand to an SMTP DATA command. from is the authenticated
// Gmail address — Gmail rejects a mismatched From header on submission, so
// it must always be the account that owns the App Password, not something
// the caller can override per message.
func buildMessage(from string, message domain.Message) ([]byte, error) {
	var buf bytes.Buffer

	if message.AttachmentPath == "" {
		writeHeaders(&buf, from, message.To, message.Subject, "text/plain; charset=utf-8")
		buf.WriteString("\r\n")
		buf.WriteString(message.Body)
		return buf.Bytes(), nil
	}

	writer := multipart.NewWriter(&buf)
	writeHeaders(&buf, from, message.To, message.Subject, fmt.Sprintf("multipart/mixed; boundary=%q", writer.Boundary()))
	buf.WriteString("\r\n")

	bodyPart, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type": {"text/plain; charset=utf-8"},
	})
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear la parte del cuerpo: %w", err)
	}
	if _, err := bodyPart.Write([]byte(message.Body)); err != nil {
		return nil, fmt.Errorf("no se pudo escribir el cuerpo: %w", err)
	}

	if err := attachFile(writer, message.AttachmentPath); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("no se pudo cerrar el mensaje multipart: %w", err)
	}

	return buf.Bytes(), nil
}

func writeHeaders(buf *bytes.Buffer, from, to, subject, contentType string) {
	fmt.Fprintf(buf, "From: %s\r\n", from)
	fmt.Fprintf(buf, "To: %s\r\n", to)
	fmt.Fprintf(buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(buf, "Content-Type: %s\r\n", contentType)
}

// attachFile reads path and writes it as a base64 part, matching how
// outlook.Adapter.Send treats domain.Message.AttachmentPath: a single
// optional attachment, resolved relative to nothing (the caller already
// resolved it to an absolute path during batch validation).
func attachFile(writer *multipart.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no se pudo leer el archivo adjunto %q: %w", path, err)
	}

	filename := filepath.Base(path)
	header := textproto.MIMEHeader{
		"Content-Type":              {mimeTypeFor(filename)},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {fmt.Sprintf("attachment; filename=%q", filename)},
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("no se pudo crear la parte del adjunto: %w", err)
	}

	encoder := base64.NewEncoder(base64.StdEncoding, part)
	if _, err := io.Copy(encoder, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("no se pudo codificar el adjunto: %w", err)
	}
	return encoder.Close()
}

func mimeTypeFor(filename string) string {
	if t := mime.TypeByExtension(filepath.Ext(filename)); t != "" {
		return t
	}
	return "application/octet-stream"
}
