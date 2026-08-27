package excel

import (
	"fmt"
	"path/filepath"

	"github.com/xuri/excelize/v2"

	"boleta-mailer/app/domain"
)

const reportSheetName = "Reporte"

var reportHeader = []string{"Nombre", "DNI", "Correo", "Archivo", "Estado", "Error"}

// WriteReport writes one row per batch item to path — the send result the
// user reviews after a batch finishes (docs/specs/ui_spec_mvp.md §3). Only
// operational metadata is included: no boleta contents, no salary data
// (context.md §23 applies to the report the same way it applies to logs).
func WriteReport(batch domain.Batch, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName(f.GetSheetName(0), reportSheetName)

	for col, title := range reportHeader {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		if err := f.SetCellValue(reportSheetName, cell, title); err != nil {
			return fmt.Errorf("no se pudo escribir el encabezado: %w", err)
		}
	}

	for i, item := range batch.Items {
		row := i + 2 // 1-based, plus header row
		values := []any{
			item.Employee.Name,
			item.Employee.DNI,
			item.Employee.Email,
			filepath.Base(item.Document.Path),
			string(item.Status),
			errorMessage(item),
		}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			if err := f.SetCellValue(reportSheetName, cell, value); err != nil {
				return fmt.Errorf("no se pudo escribir la fila %d: %w", row, err)
			}
		}
	}

	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("no se pudo guardar el reporte: %w", err)
	}
	return nil
}

// errorMessage returns whichever error explains why the item did not send
// successfully — validation error if it never reached the send step, send
// error if it did and failed. Empty for SENT items.
func errorMessage(item domain.BatchItem) string {
	if item.ValidationError != "" {
		return item.ValidationError
	}
	return item.SendError
}
