package application

import (
	"context"
	"errors"
	"testing"

	"boleta-mailer/app/domain"
)

// fakeMailSender lets batch-processing logic be tested without Outlook —
// this is the reason MailSender exists as an interface (agents.md §23).
type fakeMailSender struct {
	failFor map[string]bool // keyed by message.To
	sent    []domain.Message
}

func (f *fakeMailSender) Send(ctx context.Context, message domain.Message) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.sent = append(f.sent, message)
	if f.failFor[message.To] {
		return errors.New("simulated Outlook error")
	}
	return nil
}

func makeBatch(n int) domain.Batch {
	items := make([]domain.BatchItem, n)
	for i := range items {
		items[i] = domain.BatchItem{
			Employee: domain.Employee{Name: "Empleado", DNI: "1", Email: "e@example.com"},
			Status:   domain.StatusReady,
		}
	}
	return domain.Batch{Items: items, Template: domain.MessageTemplate{Subject: "S", Body: "B {NOMBRE}"}}
}

func TestRunBatch_AllSucceed(t *testing.T) {
	batch := makeBatch(3)
	sender := &fakeMailSender{}
	var progresses []Progress

	RunBatch(context.Background(), &batch, sender, func(p Progress) {
		progresses = append(progresses, p)
	})

	for _, item := range batch.Items {
		if item.Status != domain.StatusSent {
			t.Errorf("expected SENT, got %s", item.Status)
		}
	}
	if len(sender.sent) != 3 {
		t.Errorf("expected 3 sends, got %d", len(sender.sent))
	}
	last := progresses[len(progresses)-1]
	if last.Sent != 3 || last.Failed != 0 || last.Processed != 3 {
		t.Errorf("unexpected final progress: %+v", last)
	}
}

func TestRunBatch_ItemFailureDoesNotStopBatch(t *testing.T) {
	batch := domain.Batch{
		Items: []domain.BatchItem{
			{Employee: domain.Employee{Name: "A", Email: "a@x.com"}, Status: domain.StatusReady},
			{Employee: domain.Employee{Name: "B", Email: "b@x.com"}, Status: domain.StatusReady},
			{Employee: domain.Employee{Name: "C", Email: "c@x.com"}, Status: domain.StatusReady},
		},
		Template: domain.MessageTemplate{Subject: "S", Body: "B"},
	}
	sender := &fakeMailSender{failFor: map[string]bool{"b@x.com": true}}

	RunBatch(context.Background(), &batch, sender, func(Progress) {})

	if batch.Items[0].Status != domain.StatusSent {
		t.Errorf("item A: expected SENT, got %s", batch.Items[0].Status)
	}
	if batch.Items[1].Status != domain.StatusFailed {
		t.Errorf("item B: expected FAILED, got %s", batch.Items[1].Status)
	}
	if batch.Items[1].SendError == "" {
		t.Error("item B: expected SendError to be set")
	}
	if batch.Items[2].Status != domain.StatusSent {
		t.Errorf("item C: expected SENT (batch must continue after item failure), got %s", batch.Items[2].Status)
	}
}

func TestRunBatch_InvalidItemsAreSkipped(t *testing.T) {
	batch := domain.Batch{
		Items: []domain.BatchItem{
			{Employee: domain.Employee{Name: "A", Email: "a@x.com"}, Status: domain.StatusReady},
			{Employee: domain.Employee{Name: "B"}, Status: domain.StatusInvalid, ValidationError: "correo vacío"},
		},
		Template: domain.MessageTemplate{Subject: "S", Body: "B"},
	}
	sender := &fakeMailSender{}

	var lastProgress Progress
	RunBatch(context.Background(), &batch, sender, func(p Progress) { lastProgress = p })

	if len(sender.sent) != 1 {
		t.Fatalf("expected only the READY item to be sent, sender got %d calls", len(sender.sent))
	}
	if batch.Items[1].Status != domain.StatusInvalid {
		t.Errorf("INVALID item must remain untouched, got %s", batch.Items[1].Status)
	}
	if lastProgress.Total != 1 {
		t.Errorf("Total must count only READY items (1), not the whole batch (2), got %d", lastProgress.Total)
	}
}

func TestPrepareRetry_ResetsOnlyFailedItems(t *testing.T) {
	batch := domain.Batch{
		Items: []domain.BatchItem{
			{Employee: domain.Employee{Name: "A"}, Status: domain.StatusSent},
			{Employee: domain.Employee{Name: "B"}, Status: domain.StatusFailed, SendError: "boom"},
			{Employee: domain.Employee{Name: "C"}, Status: domain.StatusInvalid, ValidationError: "correo vacío"},
			{Employee: domain.Employee{Name: "D"}, Status: domain.StatusFailed, SendError: "timeout"},
		},
	}

	count := PrepareRetry(&batch)

	if count != 2 {
		t.Errorf("expected 2 items reset, got %d", count)
	}
	if batch.Items[0].Status != domain.StatusSent {
		t.Errorf("SENT item must remain untouched, got %s", batch.Items[0].Status)
	}
	if batch.Items[1].Status != domain.StatusReady || batch.Items[1].SendError != "" {
		t.Errorf("item B must be reset to READY with cleared SendError, got status=%s error=%q", batch.Items[1].Status, batch.Items[1].SendError)
	}
	if batch.Items[2].Status != domain.StatusInvalid {
		t.Errorf("INVALID item must remain untouched, got %s", batch.Items[2].Status)
	}
	if batch.Items[3].Status != domain.StatusReady || batch.Items[3].SendError != "" {
		t.Errorf("item D must be reset to READY with cleared SendError, got status=%s error=%q", batch.Items[3].Status, batch.Items[3].SendError)
	}
}

func TestRunBatch_AfterPrepareRetry_OnlyRetriesFailedItems(t *testing.T) {
	batch := domain.Batch{
		Items: []domain.BatchItem{
			{Employee: domain.Employee{Name: "A", Email: "a@x.com"}, Status: domain.StatusSent},
			{Employee: domain.Employee{Name: "B", Email: "b@x.com"}, Status: domain.StatusFailed, SendError: "boom"},
		},
		Template: domain.MessageTemplate{Subject: "S", Body: "B"},
	}
	sender := &fakeMailSender{}

	count := PrepareRetry(&batch)
	if count != 1 {
		t.Fatalf("expected 1 item reset, got %d", count)
	}

	var lastProgress Progress
	RunBatch(context.Background(), &batch, sender, func(p Progress) { lastProgress = p })

	if len(sender.sent) != 1 || sender.sent[0].To != "b@x.com" {
		t.Fatalf("expected only item B to be retried, sender got: %+v", sender.sent)
	}
	if batch.Items[1].Status != domain.StatusSent {
		t.Errorf("retried item expected SENT, got %s", batch.Items[1].Status)
	}
	if lastProgress.Total != 1 {
		t.Errorf("Total must count only the retried item (1), got %d", lastProgress.Total)
	}
}

func TestRunBatch_CancellationLeavesRemainingItemsCancelled(t *testing.T) {
	batch := makeBatch(5)
	sender := &fakeMailSender{}
	ctx, cancel := context.WithCancel(context.Background())

	processed := 0
	RunBatch(ctx, &batch, sender, func(p Progress) {
		processed++
		if processed == 2 {
			cancel()
		}
	})

	sentCount, cancelledCount := 0, 0
	for _, item := range batch.Items {
		switch item.Status {
		case domain.StatusSent:
			sentCount++
		case domain.StatusCancelled:
			cancelledCount++
		}
	}
	if sentCount != 2 {
		t.Errorf("expected 2 items sent before cancellation, got %d", sentCount)
	}
	if cancelledCount != 3 {
		t.Errorf("expected 3 items left CANCELLED, got %d", cancelledCount)
	}
}
