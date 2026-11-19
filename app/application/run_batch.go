package application

import (
	"context"
	"time"

	"despacho/app/domain"
)

// Event is reported for every state change of an item: once when it starts
// SENDING and once when it ends (SENT, FAILED or CANCELLED). Item is a copy,
// so the receiver can keep it without racing the worker.
type Event struct {
	Index     int
	Item      domain.BatchItem
	Total     int // items that were READY when the run started
	Processed int
	Sent      int
	Failed    int
}

// RunOptions tunes a run.
type RunOptions struct {
	// Delay is the pause between two sends. Providers throttle or flag
	// accounts that burst hundreds of messages; a few seconds apart looks
	// like a human and keeps the account healthy.
	Delay time.Duration
	// OnEvent is called synchronously on the worker goroutine.
	OnEvent func(Event)
}

// Result summarizes a finished run.
type Result struct {
	Sent, Failed, Cancelled int
}

// RunBatch sends every READY item of batch, in order, on the caller's
// goroutine — it never starts goroutines itself, so whoever calls it decides
// which thread the sender runs on (Outlook COM needs that control).
//
// Cancellation is cooperative: ctx is checked before each item and during
// the pause, never in the middle of a send. Items not reached are marked
// CANCELLED so the final state accounts for every item.
func RunBatch(ctx context.Context, batch *domain.Batch, sender domain.MailSender, opts RunOptions) Result {
	emit := opts.OnEvent
	if emit == nil {
		emit = func(Event) {}
	}

	total := 0
	for _, item := range batch.Items {
		if item.Status == domain.StatusReady {
			total++
		}
	}

	var res Result
	first := true
	for i := range batch.Items {
		item := &batch.Items[i]
		if item.Status != domain.StatusReady {
			continue
		}

		if !first && opts.Delay > 0 && ctx.Err() == nil {
			wait(ctx, opts.Delay)
		}
		first = false

		if ctx.Err() != nil {
			item.Status = domain.StatusCancelled
			res.Cancelled++
			emit(event(i, *item, total, res))
			continue
		}

		item.Status = domain.StatusSending
		emit(event(i, *item, total, res))

		msg := batch.Template.Render(item.Recipient, item.Attachments)
		if err := sender.Send(ctx, msg); err != nil {
			item.Status = domain.StatusFailed
			item.SendError = err.Error()
			res.Failed++
		} else {
			item.Status = domain.StatusSent
			item.SendError = ""
			res.Sent++
		}
		emit(event(i, *item, total, res))
	}
	return res
}

// wait sleeps for d or until ctx is cancelled, whichever comes first. A
// plain time.Sleep would make "Cancelar" feel broken during long pauses.
func wait(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func event(i int, item domain.BatchItem, total int, r Result) Event {
	return Event{Index: i, Item: item, Total: total, Processed: r.Sent + r.Failed, Sent: r.Sent, Failed: r.Failed}
}

// PrepareRetry puts FAILED and CANCELLED items back to READY so the next
// RunBatch attempts only those — items already SENT are never re-sent.
// Returns how many items were reset.
func PrepareRetry(batch *domain.Batch) int {
	n := 0
	for i := range batch.Items {
		switch batch.Items[i].Status {
		case domain.StatusFailed, domain.StatusCancelled:
			batch.Items[i].Status = domain.StatusReady
			batch.Items[i].SendError = ""
			n++
		}
	}
	return n
}
