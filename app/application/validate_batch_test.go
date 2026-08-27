package application

import (
	"path/filepath"
	"testing"

	"boleta-mailer/app/domain"
)

func dataPaths(t *testing.T) (excelPath, folderPath string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("resolving data dir: %v", err)
	}
	return filepath.Join(root, "Plantilla_Envios.xlsx"), filepath.Join(root, "Boletas")
}

func TestValidateBatch_RealData(t *testing.T) {
	excelPath, folderPath := dataPaths(t)

	batch, err := ValidateBatch(excelPath, folderPath)
	if err != nil {
		t.Fatalf("ValidateBatch returned error: %v", err)
	}

	if len(batch.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(batch.Items))
	}

	summary := batch.Summary()
	if summary.Valid != 2 || summary.Invalid != 0 {
		t.Errorf("expected 2 valid / 0 invalid, got %+v", summary)
	}

	for _, item := range batch.Items {
		if item.Status != domain.StatusReady {
			t.Errorf("expected READY for DNI %s, got %s (%s)", item.Employee.DNI, item.Status, item.ValidationError)
		}
		if item.Document.Path == "" {
			t.Errorf("expected resolved document path for DNI %s", item.Employee.DNI)
		}
	}

	if batch.Template.Subject != "Envio de Boletas" {
		t.Errorf("unexpected subject: %q", batch.Template.Subject)
	}
}

func TestValidateBatch_MissingDocument(t *testing.T) {
	excelPath, _ := dataPaths(t)
	emptyFolder := t.TempDir()

	batch, err := ValidateBatch(excelPath, emptyFolder)
	if err != nil {
		t.Fatalf("ValidateBatch returned error: %v", err)
	}

	summary := batch.Summary()
	if summary.Invalid != 2 {
		t.Fatalf("expected all 2 items invalid (no documents), got %+v", summary)
	}
	for _, item := range batch.Items {
		if item.Status != domain.StatusInvalid {
			t.Errorf("expected INVALID for DNI %s, got %s", item.Employee.DNI, item.Status)
		}
	}
}
