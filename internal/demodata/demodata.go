// Package demodata generates a fully fictitious dataset for demos, screen
// recordings and end-to-end tests: an Excel template with 12 recipients (10 valid, 2 with
// deliberately seeded errors) plus the "scanned" PDFs the app renames and
// attaches. No real person is involved. By default every address uses the
// reserved example.com domain (RFC 2606), so a misconfigured real send can
// never reach anyone; Options.Inbox redirects them to your own mailbox for a
// real-send demo.
package demodata

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type recipient struct {
	Email  string
	Name   string
	ID     string
	Course string
	CC     string
}

const issueDate = "08/11/2026"

// baseRecipients is the fixed part of the demo dataset. Two rows are broken on purpose so the
// validation step has something to show on camera:
//   - index 10: email without a valid domain
//   - index 11: its scan PDF is never generated (see missingScan)
var baseRecipients = []recipient{
	{"ana.torres@example.com", "Ana Torres", "1001", "Go para backend", ""},
	{"bruno.diaz@example.com", "Bruno Díaz", "1002", "Go para backend", ""},
	{"carla.mendoza@example.com", "Carla Mendoza", "1003", "Wails + React", "coordinacion@example.com"},
	{"diego.ramirez@example.com", "Diego Ramírez", "1004", "Wails + React", ""},
	{"elena.castro@example.com", "Elena Castro", "1005", "Desktop apps con Go", ""},
	{"fernando.rojas@example.com", "Fernando Rojas", "1006", "Go para backend", ""},
	{"gabriela.ortiz@example.com", "Gabriela Ortiz", "1007", "Wails + React", "coordinacion@example.com"},
	{"hugo.salazar@example.com", "Hugo Salazar", "1008", "Desktop apps con Go", ""},
	{"isabel.vargas@example.com", "Isabel Vargas", "1009", "Go para backend", ""},
	{"jorge.paredes@example.com", "Jorge Paredes", "1010", "Desktop apps con Go", ""},
	{"karen.luna@example", "Karen Luna", "1011", "Wails + React", ""},
	{"luis.herrera@example.com", "Luis Herrera", "1012", "Go para backend", ""},
}

// missingScan is the recipient index whose PDF is intentionally not written,
// so the attachment step reports "archivo no encontrado".
const missingScan = 11

// The subject carries the name so Gmail does not thread all demo messages
// into one conversation (it groups by subject).
const templateSubject = "{{nombre}}, aquí está tu certificado de {{curso}}"

// templateBody is a designed HTML email: table layout and inline styles,
// because that is what email clients (Gmail included) actually render.
// Values inserted into it are HTML-escaped by domain.Fill.
const templateBody = `<div style="background:#f4f5f7;padding:24px;font-family:Segoe UI,Arial,sans-serif;color:#1b1a17">
  <table width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;overflow:hidden;border:1px solid #e3e5ea">
    <tr><td style="background:#007d9c;padding:18px 28px;color:#ffffff;font-size:12px;letter-spacing:2px;font-weight:bold">DESPACHO · CERTIFICADOS</td></tr>
    <tr><td style="padding:28px">
      <h1 style="margin:0 0 12px;font-size:22px">¡Felicidades, {{nombre}}!</h1>
      <p style="margin:0 0 18px;line-height:1.6">Completaste el curso <b>{{curso}}</b>. Adjuntamos tu certificado en PDF.</p>
      <table width="100%" cellpadding="0" cellspacing="0" style="background:#eef7fa;border-radius:8px">
        <tr>
          <td style="padding:14px 18px;font-size:13px;color:#5c6170">N.º de certificado<br><b style="font-size:18px;color:#1b1a17">{{documento_id}}</b></td>
          <td style="padding:14px 18px;font-size:13px;color:#5c6170">Fecha de emisión<br><b style="font-size:18px;color:#1b1a17">{{fecha}}</b></td>
        </tr>
      </table>
      <p style="margin:22px 0 0;color:#5c6170;font-size:12px">Enviado con Despacho · Go + Wails + React</p>
    </td></tr>
  </table>
</div>`

// Options tunes the generated dataset.
type Options struct {
	// Count is the total number of recipients (minimum 12, see Generate).
	Count int
	// Inbox, when set, turns every address into a sub-address of this real
	// mailbox: ana.torres@example.com becomes you+ana.torres@gmail.com.
	// Gmail and Outlook.com deliver "+tag" addresses to the base inbox, so a
	// real send lands only in your own mailbox — never in a stranger's.
	// The seeded invalid address is left broken on purpose.
	Inbox string
}

// Result describes what Generate wrote.
type Result struct {
	Workbook   string
	ScansDir   string
	Recipients int
	Scans      int
	Missing    string
}

// Generate writes plantilla_envio.xlsx and documentos_origen/ under out.
// opt.Count is the total number of recipients: the first 12 are always the
// fixed dataset (with its two seeded errors); anything above that is filled
// with deterministic, valid extra rows — handy for "500 emails" demos.
func Generate(out string, opt Options) (Result, error) {
	list := buildRecipients(opt.Count)
	if opt.Inbox != "" {
		var err error
		if list, err = redirectTo(opt.Inbox, list); err != nil {
			return Result{}, err
		}
	}
	scansDir := filepath.Join(out, "documentos_origen")
	if err := os.MkdirAll(scansDir, 0o755); err != nil {
		return Result{}, err
	}

	xlsxPath := filepath.Join(out, "plantilla_envio.xlsx")
	if err := writeWorkbook(xlsxPath, list); err != nil {
		return Result{}, fmt.Errorf("generar Excel: %w", err)
	}

	written := 0
	for i, r := range list {
		if i == missingScan {
			continue
		}
		path := filepath.Join(scansDir, scanName(i))
		if err := os.WriteFile(path, certificatePDF(r), 0o644); err != nil {
			return Result{}, fmt.Errorf("generar %s: %w", path, err)
		}
		written++
	}

	return Result{Workbook: xlsxPath, ScansDir: scansDir, Recipients: len(list), Scans: written, Missing: scanName(missingScan)}, nil
}

var (
	extraFirst   = []string{"Andrea", "Benjamín", "Camila", "Daniel", "Florencia", "Gonzalo", "Inés", "Joaquín", "Lucía", "Mateo", "Natalia", "Óscar", "Paula", "Rodrigo", "Sofía", "Tomás", "Valeria", "Ximena"}
	extraLast    = []string{"Acosta", "Benítez", "Cárdenas", "Delgado", "Espinoza", "Fuentes", "Guzmán", "Herrera", "Ibáñez", "Medina", "Navarro", "Ochoa", "Peña", "Quispe", "Ríos", "Salinas", "Valdivia", "Zamora"}
	extraCourses = []string{"Go para backend", "Wails + React", "Desktop apps con Go"}
)

// buildRecipients returns the fixed dataset followed by count-12 generated
// rows. Generation is deterministic (no randomness), so two runs with the
// same count produce identical files — easier to diff, easier to re-record.
func buildRecipients(count int) []recipient {
	list := append([]recipient(nil), baseRecipients...)
	for i := len(list); i < count; i++ {
		n := i - len(baseRecipients)
		first := extraFirst[n%len(extraFirst)]
		last := extraLast[(n*7+3)%len(extraLast)]
		list = append(list, recipient{
			Email:  fmt.Sprintf("%s.%s%d@example.com", asciiLower(first), asciiLower(last), i+1),
			Name:   first + " " + last,
			ID:     fmt.Sprintf("%d", 1001+i),
			Course: extraCourses[n%len(extraCourses)],
		})
	}
	return list
}

// redirectTo rewrites every well-formed address in list as a "+tag"
// sub-address of inbox, keeping the original local part as the tag.
func redirectTo(inbox string, list []recipient) ([]recipient, error) {
	user, domain, ok := strings.Cut(strings.TrimSpace(inbox), "@")
	if !ok || user == "" || !strings.Contains(domain, ".") || strings.ContainsAny(user, "+ ") {
		return nil, fmt.Errorf("bandeja %q inválida: usa una dirección simple, p. ej. tucorreo@gmail.com", inbox)
	}
	alias := func(addr string) string {
		local, d, ok := strings.Cut(addr, "@")
		if !ok || !strings.Contains(d, ".") {
			return addr // the seeded invalid address must stay invalid
		}
		return user + "+" + local + "@" + domain
	}
	out := make([]recipient, len(list))
	for i, r := range list {
		r.Email = alias(r.Email)
		if r.CC != "" {
			r.CC = alias(r.CC)
		}
		out[i] = r
	}
	return out, nil
}

// asciiLower strips Spanish accents so generated addresses stay plain ASCII.
func asciiLower(s string) string {
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u")
	return strings.ToLower(r.Replace(s))
}

// scanName mimics the meaningless names a scanner produces — the reason the
// rename step exists in the first place.
func scanName(i int) string {
	return fmt.Sprintf("scan_%04d.pdf", i+1)
}

func writeWorkbook(path string, list []recipient) error {
	f := excelize.NewFile()
	defer f.Close()

	const data = "Destinatarios"
	if err := f.SetSheetName("Sheet1", data); err != nil {
		return err
	}

	header := []any{"correo", "nombre", "documento_id", "curso", "fecha", "archivo_origen", "cc"}
	if err := f.SetSheetRow(data, "A1", &header); err != nil {
		return err
	}
	for i, r := range list {
		// documento_id stays a string: it is an identifier, not a quantity,
		// and must keep any leading zeros a real dataset might have.
		row := []any{r.Email, r.Name, r.ID, r.Course, issueDate, scanName(i), r.CC}
		if err := f.SetSheetRow(data, fmt.Sprintf("A%d", i+2), &row); err != nil {
			return err
		}
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"007D9C"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(data, "A1", "G1", headerStyle); err != nil {
		return err
	}
	if err := f.SetRowHeight(data, 1, 22); err != nil {
		return err
	}
	widths := map[string]float64{"A": 30, "B": 18, "C": 14, "D": 22, "E": 12, "F": 18, "G": 28}
	for col, w := range widths {
		if err := f.SetColWidth(data, col, col, w); err != nil {
			return err
		}
	}
	if err := f.SetPanes(data, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	lastRow := len(list) + 1
	if err := f.AutoFilter(data, fmt.Sprintf("A1:G%d", lastRow), nil); err != nil {
		return err
	}

	if err := writeMessageSheet(f, headerStyle); err != nil {
		return err
	}
	if err := writeInstructionsSheet(f); err != nil {
		return err
	}

	f.SetActiveSheet(0)
	return f.SaveAs(path)
}

func writeMessageSheet(f *excelize.File, headerStyle int) error {
	const sheet = "Mensaje"
	if _, err := f.NewSheet(sheet); err != nil {
		return err
	}
	header := []any{"asunto", "cuerpo", "formato"}
	if err := f.SetSheetRow(sheet, "A1", &header); err != nil {
		return err
	}
	values := []any{templateSubject, templateBody, "html"}
	if err := f.SetSheetRow(sheet, "A2", &values); err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A1", "C1", headerStyle); err != nil {
		return err
	}
	wrap, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A2", "C2", wrap); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, "A", "A", 34); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, "B", "B", 80); err != nil {
		return err
	}
	return f.SetRowHeight(sheet, 2, 300)
}

func writeInstructionsSheet(f *excelize.File) error {
	const sheet = "Instrucciones"
	if _, err := f.NewSheet(sheet); err != nil {
		return err
	}
	lines := []string{
		"Plantilla de envío masivo — datos de DEMOSTRACIÓN (personas ficticias)",
		"",
		"Hoja «Destinatarios»: una fila por correo.",
		"  correo          obligatoria. Dirección del destinatario.",
		"  nombre          recomendada. Se usa en el saludo: {{nombre}}.",
		"  documento_id    clave para encontrar el adjunto, p. ej. CERT-{{documento_id}}_*.pdf",
		"  archivo_origen  nombre actual del archivo escaneado, usado por el paso «Renombrar».",
		"  cc              opcional. Copia a otra dirección.",
		"  Cualquier otra columna (curso, fecha, …) queda disponible como variable {{columna}}.",
		"",
		"Hoja «Mensaje» (opcional): asunto, cuerpo y formato (html | texto) iniciales; se editan luego en la app.",
		"",
		"Errores sembrados a propósito para la demo:",
		"  fila 12  karen.luna@example  → correo inválido",
		"  fila 13  scan_0012.pdf       → el archivo no existe en documentos_origen",
	}
	for i, line := range lines {
		if err := f.SetCellStr(sheet, fmt.Sprintf("A%d", i+1), line); err != nil {
			return err
		}
	}
	title, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 13}})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A1", "A1", title); err != nil {
		return err
	}
	return f.SetColWidth(sheet, "A", "A", 100)
}

// certificatePDF builds a one-page landscape A4 certificate by hand. A
// minimal PDF is just numbered objects plus a byte-offset table (xref) —
// small enough to write with the standard library instead of pulling in a
// PDF dependency for demo data.
func certificatePDF(r recipient) []byte {
	var c bytes.Buffer
	text := func(font string, size, x, y float64, s string) {
		fmt.Fprintf(&c, "BT /%s %.0f Tf %.0f %.0f Td (%s) Tj ET\n", font, size, x, y, pdfString(s))
	}

	// Frame and top band in the app's accent color (#007D9C).
	c.WriteString("0 0.49 0.61 RG 2 w 30 30 782 535 re S\n")
	c.WriteString("0 0.49 0.61 rg 30 505 782 60 re f\n")
	c.WriteString("1 1 1 rg\n")
	text("F2", 14, 60, 528, "DESPACHO  ·  DEMO")

	c.WriteString("0.09 0.09 0.11 rg\n")
	text("F2", 30, 80, 420, "Certificado de participación")
	text("F1", 14, 80, 380, "Se otorga el presente certificado a")
	text("F2", 34, 80, 325, r.Name)
	text("F1", 14, 80, 280, "por completar el curso")
	text("F2", 20, 80, 250, r.Course)
	text("F1", 12, 80, 150, "Fecha de emisión: "+issueDate)
	text("F1", 12, 80, 130, "ID: "+r.ID)

	c.WriteString("0.36 0.38 0.43 rg\n")
	text("F1", 10, 80, 60, "Documento de demostración generado con datos ficticios.")

	return buildPDF(c.Bytes())
}

func buildPDF(content []byte) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 842 595] /Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
	}

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}

	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

// pdfString encodes s for a PDF literal string using the font's
// WinAnsiEncoding: Spanish accented letters fall in Latin-1, which maps
// byte-for-byte. Runes outside that range become '?', and the PDF string
// delimiters are escaped.
func pdfString(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch {
		case r == '\\' || r == '(' || r == ')':
			b.WriteByte('\\')
			b.WriteByte(byte(r))
		case r < 256:
			b.WriteByte(byte(r))
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}
