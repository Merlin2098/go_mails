package excel

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"boleta-mailer/app/domain"
)

func TestWriteReport(t *testing.T) {
	batch := domain.Batch{
		Items: []domain.BatchItem{
			{
				Employee: domain.Employee{Name: "Juan Pérez", DNI: "12345678", Email: "juan@x.com"},
				Document: domain.Document{Path: `C:\Boletas\12345678.pdf`},
				Status:   domain.StatusSent,
			},
			{
				Employee:  domain.Employee{Name: "Ana López", DNI: "87654321", Email: "ana@x.com"},
				Document:  domain.Document{Path: `C:\Boletas\87654321.pdf`},
				Status:    domain.StatusFailed,
				SendError: "Outlook rechazó el envío",
			},
			{
				Employee:        domain.Employee{Name: "Pedro Ruiz", DNI: "11111111"},
				Status:          domain.StatusInvalid,
				ValidationField: "correo",
				ValidationError: "Formato de correo inválido",
			},
		},
	}

	path := filepath.Join(t.TempDir(), "reporte.xlsx")
	if err := WriteReport(batch, path); err != nil {
		t.Fatalf("WriteReport returned error: %v", err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("could not reopen written report: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows(reportSheetName)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 4 { // header + 3 items
		t.Fatalf("expected 4 rows, got %d: %+v", len(rows), rows)
	}

	if rows[0][0] != "Nombre" || rows[0][4] != "Estado" {
		t.Errorf("unexpected header: %v", rows[0])
	}

	sent := rows[1]
	if sent[0] != "Juan Pérez" || sent[3] != "12345678.pdf" || sent[4] != "SENT" || cellAt(sent, 5) != "" {
		t.Errorf("unexpected SENT row: %v", sent)
	}

	failed := rows[2]
	if failed[4] != "FAILED" || failed[5] != "Outlook rechazó el envío" {
		t.Errorf("unexpected FAILED row: %v", failed)
	}

	invalid := rows[3]
	if invalid[4] != "INVALID" || invalid[5] != "Formato de correo inválido" {
		t.Errorf("unexpected INVALID row: %v", invalid)
	}
}
