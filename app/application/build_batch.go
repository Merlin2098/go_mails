// Package application holds the use cases: turning a raw table into a
// validated batch, sending it, and renaming documents. It orchestrates the
// domain against small interfaces, so it never imports a concrete adapter.
package application

import (
	"fmt"
	"net/mail"
	"strings"

	"despacho/app/domain"
)

// ColumnMapping says which table column plays each role. Only Email is
// required; the others may be "".
type ColumnMapping struct {
	Email string
	Name  string
	CC    string
	Key   string // identifies the row's document, e.g. documento_id
}

// DocumentIndex is what BuildBatch needs from the filesystem. The concrete
// filesystem.Index satisfies it; tests use an in-memory fake. Declaring the
// interface here, at the consumer, is the idiomatic Go direction: "accept
// interfaces, return structs".
type DocumentIndex interface {
	Glob(pattern string) ([]string, error)
	Resolve(name string) (string, bool)
}

// BuildBatch validates the whole campaign before anything is sent. Problems
// that make the campaign itself unusable (a missing column, an unknown
// {{variable}}) are returned as an error; problems with individual rows mark
// that item INVALID with a reason, so the user can fix the spreadsheet and
// the rest of the batch stays sendable.
func BuildBatch(table domain.Table, m ColumnMapping, tpl domain.Template, rule domain.AttachmentRule, docs DocumentIndex) (domain.Batch, error) {
	cols := columnIndex(table.Headers)

	if m.Email == "" {
		return domain.Batch{}, fmt.Errorf("elige la columna que contiene el correo")
	}
	for role, name := range map[string]string{"correo": m.Email, "nombre": m.Name, "copia": m.CC, "clave": m.Key} {
		if name != "" {
			if _, ok := cols[domain.NormalizeKey(name)]; !ok {
				return domain.Batch{}, fmt.Errorf("la columna %q (%s) no existe en el archivo", name, role)
			}
		}
	}
	if err := CheckTemplate(table.Headers, tpl); err != nil {
		return domain.Batch{}, err
	}
	if err := CheckRule(table.Headers, rule, docs); err != nil {
		return domain.Batch{}, err
	}

	seenKey := map[string]int{}
	items := make([]domain.BatchItem, 0, len(table.Rows))
	for i, row := range table.Rows {
		r := recipientFromRow(i+2, row, table.Headers, m)
		item := domain.BatchItem{Recipient: r, Status: domain.StatusReady}

		field, problem := validateRecipient(r, seenKey)
		if r.Key != "" {
			if _, dup := seenKey[r.Key]; !dup {
				seenKey[r.Key] = r.Row
			}
		}
		if problem == "" {
			attachments, err := resolveAttachments(rule, r, docs)
			if err != nil {
				field, problem = "adjunto", err.Error()
			}
			item.Attachments = attachments
		}
		if problem != "" {
			item.Status = domain.StatusInvalid
			item.ValidationField = field
			item.ValidationError = problem
		}
		items = append(items, item)
	}

	return domain.Batch{Items: items, Template: tpl}, nil
}

func columnIndex(headers []string) map[string]int {
	cols := make(map[string]int, len(headers))
	for i, h := range headers {
		cols[domain.NormalizeKey(h)] = i
	}
	return cols
}

func checkVariables(vars []string, cols map[string]int, where string) error {
	var unknown []string
	for _, v := range vars {
		if _, ok := cols[v]; !ok {
			unknown = append(unknown, "{{"+v+"}}")
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("%s usa variables que no existen como columna: %s", where, strings.Join(unknown, ", "))
	}
	return nil
}

// CheckTemplate reports problems with the message itself. It is exported so
// the UI layer can validate rows and template independently: a user still on
// the data step should see row errors even before writing a subject.
func CheckTemplate(headers []string, tpl domain.Template) error {
	if strings.TrimSpace(tpl.Subject) == "" {
		return fmt.Errorf("el asunto no puede estar vacío")
	}
	return checkVariables(tpl.Variables(), columnIndex(headers), "el mensaje")
}

// CheckRule reports problems with the attachment configuration.
func CheckRule(headers []string, rule domain.AttachmentRule, docs DocumentIndex) error {
	cols := columnIndex(headers)
	switch rule.Mode {
	case domain.AttachNone, "":
		return nil
	case domain.AttachPattern:
		if strings.TrimSpace(rule.Pattern) == "" {
			return fmt.Errorf("escribe el patrón para encontrar los adjuntos")
		}
		if err := checkVariables(domain.Variables(rule.Pattern), cols, "el patrón de adjuntos"); err != nil {
			return err
		}
	case domain.AttachColumn:
		if _, ok := cols[domain.NormalizeKey(rule.Column)]; !ok {
			return fmt.Errorf("la columna de adjuntos %q no existe", rule.Column)
		}
	default:
		return fmt.Errorf("modo de adjuntos desconocido %q", rule.Mode)
	}
	if docs == nil {
		return fmt.Errorf("elige la carpeta de documentos")
	}
	return nil
}

func recipientFromRow(rowNumber int, row, headers []string, m ColumnMapping) domain.Recipient {
	fields := make(map[string]string, len(headers))
	for i, h := range headers {
		if i < len(row) {
			fields[domain.NormalizeKey(h)] = row[i]
		}
	}
	get := func(col string) string {
		if col == "" {
			return ""
		}
		return fields[domain.NormalizeKey(col)]
	}
	return domain.Recipient{
		Row:    rowNumber,
		Email:  get(m.Email),
		Name:   get(m.Name),
		CC:     get(m.CC),
		Key:    get(m.Key),
		Fields: fields,
	}
}

// validateRecipient returns the first problem found, in the order a person
// would fix them: the address first, then copies, then duplicates.
func validateRecipient(r domain.Recipient, seenKey map[string]int) (field, problem string) {
	switch {
	case r.Email == "":
		return "correo", "Correo vacío"
	case !IsValidEmail(r.Email):
		return "correo", "Correo inválido"
	}
	for _, cc := range domain.SplitAddresses(r.CC) {
		if !IsValidEmail(cc) {
			return "cc", "Copia (CC) inválida: " + cc
		}
	}
	if r.Key != "" {
		if first, dup := seenKey[r.Key]; dup {
			return "clave", fmt.Sprintf("Clave duplicada (ya usada en la fila %d)", first)
		}
	}
	return "", ""
}

// IsValidEmail accepts a bare address whose domain has at least one dot.
// net/mail alone accepts "user@localhost"-style addresses, which are legal
// per RFC 5322 but are always a typo in a bulk-mailing spreadsheet.
func IsValidEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != strings.TrimSpace(s) {
		return false
	}
	_, domainPart, _ := strings.Cut(addr.Address, "@")
	dot := strings.Index(domainPart, ".")
	return dot > 0 && dot < len(domainPart)-1 && !strings.HasSuffix(domainPart, ".")
}

func resolveAttachments(rule domain.AttachmentRule, r domain.Recipient, docs DocumentIndex) ([]string, error) {
	switch rule.Mode {
	case domain.AttachPattern:
		for _, v := range domain.Variables(rule.Pattern) {
			if strings.TrimSpace(r.Fields[v]) == "" {
				return nil, fmt.Errorf("Falta {{%s}} para buscar el adjunto", v)
			}
		}
		pattern := domain.Fill(rule.Pattern, r.Fields, false)
		matches, err := docs.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("Patrón inválido %q", pattern)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("Archivo no encontrado (%s)", pattern)
		}
		return matches, nil

	case domain.AttachColumn:
		names := strings.Split(r.Fields[domain.NormalizeKey(rule.Column)], ";")
		var paths []string
		for _, n := range names {
			if n = strings.TrimSpace(n); n == "" {
				continue
			}
			p, ok := docs.Resolve(n)
			if !ok {
				return nil, fmt.Errorf("Archivo no encontrado: %s", n)
			}
			paths = append(paths, p)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("La columna %q está vacía", rule.Column)
		}
		return paths, nil
	}
	return nil, nil
}
