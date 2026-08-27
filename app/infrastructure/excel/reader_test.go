package excel

import (
	"path/filepath"
	"testing"
)

// Uses the real template shipped in data/ — an integration test against the
// actual file shape the user produces, not a synthetic fixture.
func templatePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "data", "Plantilla_Envios.xlsx"))
	if err != nil {
		t.Fatalf("resolving template path: %v", err)
	}
	return path
}

func TestReadWorkbook_RealTemplate(t *testing.T) {
	wb, err := ReadWorkbook(templatePath(t))
	if err != nil {
		t.Fatalf("ReadWorkbook returned error: %v", err)
	}

	if len(wb.Employees) != 2 {
		t.Fatalf("expected 2 employees, got %d: %+v", len(wb.Employees), wb.Employees)
	}

	first := wb.Employees[0]
	if first.DNI != "44315604" || first.Name != "Lesley" || first.Email == "" {
		t.Errorf("unexpected first employee: %+v", first)
	}

	if wb.Template.Subject != "Envio de Boletas" {
		t.Errorf("unexpected subject: %q", wb.Template.Subject)
	}
	if wb.Template.Body == "" {
		t.Error("expected non-empty message body")
	}
}

func TestReadWorkbook_MissingFile(t *testing.T) {
	_, err := ReadWorkbook("no_existe.xlsx")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
