package spreadsheet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"despacho/app/domain"
)

func TestReadTable_CSVSemicolonWithBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.csv")
	content := "\xEF\xBB\xBFcorreo;Nombre;;ciudad\n a@example.com ;Ana;x;Lima\n;;;\nb@example.com;Bruno\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	table, err := ReadTable(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := table.Headers; len(got) != 4 || got[0] != "correo" || got[2] != "columna_3" {
		t.Errorf("headers = %v", got)
	}
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %v", table.Rows)
	}
	if table.Rows[0][0] != "a@example.com" || table.Rows[1][3] != "" {
		t.Errorf("rows = %v", table.Rows)
	}
}

func TestReadTable_DuplicateHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dup.csv")
	_ = os.WriteFile(path, []byte("correo,Correo\n"), 0o644)
	if _, err := ReadTable(path); err == nil {
		t.Error("expected duplicate header error")
	}
}

func TestReadTable_UnsupportedExtension(t *testing.T) {
	if _, err := ReadTable("data.txt"); err == nil {
		t.Error("expected error")
	}
}

func TestReadTableAndMessageTemplate_XLSX(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plantilla.xlsx")
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "Destinatarios")
	_ = f.SetSheetRow("Destinatarios", "A1", &[]any{"correo", "nombre"})
	_ = f.SetSheetRow("Destinatarios", "A2", &[]any{"ana@example.com", "Ana"})
	_, _ = f.NewSheet("MENSAJE")
	_ = f.SetSheetRow("MENSAJE", "A1", &[]any{"Asunto", "Cuerpo"})
	_ = f.SetSheetRow("MENSAJE", "A2", &[]any{"Hola {{nombre}}", "Cuerpo {{nombre}}"})
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	table, err := ReadTable(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Rows) != 1 || table.Rows[0][1] != "Ana" {
		t.Errorf("table = %+v", table)
	}

	tpl, found, err := ReadMessageTemplate(path)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if tpl.Subject != "Hola {{nombre}}" || tpl.Body != "Cuerpo {{nombre}}" || tpl.HTML {
		t.Errorf("tpl = %+v", tpl)
	}
}

func TestReadMessageTemplate_Format(t *testing.T) {
	cases := map[string]struct {
		header []any
		row    []any
		want   bool
	}{
		"explicit html":           {[]any{"asunto", "cuerpo", "formato"}, []any{"s", "Hola", "HTML"}, true},
		"explicit text wins":      {[]any{"asunto", "cuerpo", "formato"}, []any{"s", "<div>x</div>", "texto"}, false},
		"detected from block tag": {[]any{"asunto", "cuerpo"}, []any{"s", "<table><tr><td>x</td></tr></table>"}, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "p.xlsx")
			f := excelize.NewFile()
			_, _ = f.NewSheet("Mensaje")
			_ = f.SetSheetRow("Mensaje", "A1", &c.header)
			_ = f.SetSheetRow("Mensaje", "A2", &c.row)
			if err := f.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			tpl, found, err := ReadMessageTemplate(path)
			if err != nil || !found {
				t.Fatalf("found=%v err=%v", found, err)
			}
			if tpl.HTML != c.want {
				t.Errorf("HTML = %v, want %v", tpl.HTML, c.want)
			}
		})
	}
}

func TestWriteReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reporte.xlsx")
	batch := domain.Batch{Items: []domain.BatchItem{
		{Recipient: domain.Recipient{Row: 2, Email: "a@example.com", Name: "Ana"}, Attachments: []string{`C:\x\a.pdf`}, Status: domain.StatusSent},
		{Recipient: domain.Recipient{Row: 3, Email: "bad"}, Status: domain.StatusInvalid, ValidationError: "Correo inválido"},
	}}
	if err := WriteReport(batch, path); err != nil {
		t.Fatal(err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, _ := f.GetRows(reportSheet)
	if len(rows) != 3 {
		t.Fatalf("rows = %v", rows)
	}
	if rows[1][4] != "Enviado" || rows[2][5] != "Correo inválido" {
		t.Errorf("rows = %v", rows)
	}
}
