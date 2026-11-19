// Package mailmime renders a domain.Message as a complete RFC 5322 email.
// It is shared by every transport that speaks raw email: the SMTP adapter
// sends these bytes, the demo adapter saves them as .eml files.
package mailmime

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"despacho/app/domain"
)

// Build renders message from the given sender. Bodies use quoted-printable
// (readable in the raw source, safe for accents); attachments use base64
// wrapped at 76 columns, as RFC 2045 requires.
func Build(from mail.Address, message domain.Message) ([]byte, error) {
	var buf bytes.Buffer
	h := textproto.MIMEHeader{}
	h.Set("From", from.String())
	h.Set("To", message.To)
	if cc := domain.SplitAddresses(message.CC); len(cc) > 0 {
		h.Set("Cc", strings.Join(cc, ", "))
	}
	h.Set("Subject", mime.QEncoding.Encode("utf-8", message.Subject))
	h.Set("Date", time.Now().Format(time.RFC1123Z))
	h.Set("Message-Id", messageID(from.Address))
	h.Set("Mime-Version", "1.0")

	bodyType := "text/plain; charset=utf-8"
	if message.HTML {
		bodyType = "text/html; charset=utf-8"
	}

	if len(message.Attachments) == 0 {
		h.Set("Content-Type", bodyType)
		h.Set("Content-Transfer-Encoding", "quoted-printable")
		writeHeader(&buf, h)
		return buf.Bytes(), writeQP(&buf, message.Body)
	}

	mw := multipart.NewWriter(&buf)
	h.Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	writeHeader(&buf, h)

	bodyPart, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {bodyType},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return nil, err
	}
	if err := writeQP(bodyPart, message.Body); err != nil {
		return nil, err
	}

	for _, path := range message.Attachments {
		if err := attach(mw, path); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeHeader emits headers in a stable, conventional order (map iteration
// order is random in Go, and a reproducible .eml is easier to diff).
func writeHeader(buf *bytes.Buffer, h textproto.MIMEHeader) {
	order := []string{"From", "To", "Cc", "Subject", "Date", "Message-Id", "Mime-Version", "Content-Type", "Content-Transfer-Encoding"}
	for _, k := range order {
		if v := h.Get(k); v != "" {
			fmt.Fprintf(buf, "%s: %s\r\n", k, v)
		}
	}
	buf.WriteString("\r\n")
}

type writer interface{ Write([]byte) (int, error) }

func writeQP(w writer, body string) error {
	qp := quotedprintable.NewWriter(w)
	// Normalize to CRLF: email is a CRLF protocol, and Windows editors may
	// already have produced some \r\n.
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if _, err := qp.Write([]byte(strings.ReplaceAll(body, "\n", "\r\n"))); err != nil {
		return err
	}
	return qp.Close()
}

func attach(mw *multipart.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no se pudo leer el adjunto %q: %w", filepath.Base(path), err)
	}
	name := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	part, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {mime.FormatMediaType(contentType, map[string]string{"name": name})},
		"Content-Transfer-Encoding": {"base64"},
		// FormatMediaType applies RFC 2231 encoding for non-ASCII names
		// ("Diego Ramírez.pdf"), which every modern client understands.
		"Content-Disposition": {mime.FormatMediaType("attachment", map[string]string{"filename": name})},
	})
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	for len(encoded) > 76 {
		if _, err := fmt.Fprintf(part, "%s\r\n", encoded[:76]); err != nil {
			return err
		}
		encoded = encoded[76:]
	}
	_, err = fmt.Fprintf(part, "%s\r\n", encoded)
	return err
}

func messageID(from string) string {
	domainPart := "despacho.local"
	if _, d, ok := strings.Cut(from, "@"); ok && d != "" {
		domainPart = d
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b), domainPart)
}
