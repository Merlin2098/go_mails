package application

import (
	"context"

	"boleta-mailer/app/domain"
)

// Progress is emitted after each item finishes (sent, failed, or skipped by
// cancellation), matching the payload shape in docs/boleta_mailer_mvp_context.md §14.
// Item is the BatchItem that was just processed — its final Status and
// SendError are already set, letting the caller log or report per-item
// detail without re-deriving which item this callback refers to.
type Progress struct {
	Total     int
	Processed int
	Sent      int
	Failed    int
	Current   string // employee name of the item just processed
	Item      domain.BatchItem
}

// RunBatch sends every READY item in batch sequentially through sender,
// mutating each item's Status in place as it goes. There is exactly one
// goroutine involved: the caller's — RunBatch does not spawn goroutines
// itself, so the "single Outlook worker" from context.md §13 is whichever
// goroutine the caller runs this on (see app.go for where that is).
//
// Cancellation is cooperative: ctx is checked before starting each item, not
// mid-send. An item that was already sending when cancellation arrives is
// allowed to finish (context.md §22). Items not yet reached are left as
// StatusCancelled so the batch's final state accounts for every item.
//
// onProgress is called synchronously after every item — the caller decides
// how to relay it onward (e.g. a Wails event).
func RunBatch(ctx context.Context, batch *domain.Batch, sender domain.MailSender, onProgress func(Progress)) {
	total := 0
	for _, item := range batch.Items {
		if item.Status == domain.StatusReady {
			total++
		}
	}
	sent := 0
	failed := 0

	for i := range batch.Items {
		item := &batch.Items[i]
		if item.Status != domain.StatusReady {
			continue // already INVALID from validation — never attempted
		}

		if ctx.Err() != nil {
			item.Status = domain.StatusCancelled
			continue
		}

		item.Status = domain.StatusSending
		message := batch.Template.Render(item.Employee, item.Document.Path)

		if err := sender.Send(ctx, message); err != nil {
			item.Status = domain.StatusFailed
			item.SendError = err.Error()
			failed++
		} else {
			item.Status = domain.StatusSent
			sent++
		}

		onProgress(Progress{
			Total:     total,
			Processed: sent + failed,
			Sent:      sent,
			Failed:    failed,
			Current:   item.Employee.Name,
			Item:      *item,
		})
	}
}

// PrepareRetry resets every FAILED item in batch back to READY, in place,
// so a subsequent RunBatch call attempts only those items — items already
// SENT, INVALID, or CANCELLED are left untouched. Returns how many items
// were reset, so the caller can no-op if there is nothing to retry.
func PrepareRetry(batch *domain.Batch) int {
	count := 0
	for i := range batch.Items {
		if batch.Items[i].Status == domain.StatusFailed {
			batch.Items[i].Status = domain.StatusReady
			batch.Items[i].SendError = ""
			count++
		}
	}
	return count
}
