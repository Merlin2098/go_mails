package spreadsheet

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	"despacho/app/domain"
)

const reportSheet = "Reporte"

// WriteReport writes one row per batch item to path — the send result the
// user keeps after a batch finishes. Only operational metadata is included:
// never message bodies or attachment contents.
func WriteReport(batch domain.Batch, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName(f.GetSheetName(0), reportSheet); err != nil {
		return err
	}

	header := []any{"Fila", "Correo", "Nombre", "Adjuntos", "Estado", "Detalle"}
	if err := f.SetSheetRow(reportSheet, "A1", &header); err != nil {
		return fmt.Errorf("no se pudo escribir el encabezado: %w", err)
	}

	for i, item := range batch.Items {
		names := make([]string, len(item.Attachments))
		for j, p := range item.Attachments {
			names[j] = filepath.Base(p)
		}
		row := []any{
			item.Recipient.Row,
			item.Recipient.Email,
			item.Recipient.Name,
			strings.Join(names, "; "),
			statusLabel(item.Status),
			detail(item),
		}
		if err := f.SetSheetRow(reportSheet, fmt.Sprintf("A%d", i+2), &row); err != nil {
			return fmt.Errorf("no se pudo escribir la fila %d: %w", i+2, err)
		}
	}

	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err == nil {
		_ = f.SetCellStyle(reportSheet, "A1", "F1", style)
	}
	_ = f.SetColWidth(reportSheet, "B", "B", 32)
	_ = f.SetColWidth(reportSheet, "C", "C", 24)
	_ = f.SetColWidth(reportSheet, "D", "D", 36)
	_ = f.SetColWidth(reportSheet, "F", "F", 48)

	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("no se pudo guardar el reporte: %w", err)
	}
	return nil
}

func statusLabel(s domain.BatchItemStatus) string {
	switch s {
	case domain.StatusSent:
		return "Enviado"
	case domain.StatusFailed:
		return "Falló"
	case domain.StatusInvalid:
		return "No válido"
	case domain.StatusCancelled:
		return "Cancelado"
	default:
		return "Pendiente"
	}
}

// detail explains why an item did not send: the validation error if it never
// reached the send step, the send error if it did and failed.
func detail(item domain.BatchItem) string {
	if item.ValidationError != "" {
		return item.ValidationError
	}
	return item.SendError
}
