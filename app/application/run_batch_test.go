package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"despacho/app/domain"
)

// fakeSender lets batch logic be tested without any real transport — the
// reason domain.MailSender is an interface in the first place.
type fakeSender struct {
	failFor map[string]bool
	sent    []domain.Message
	onSend  func()
}

func (f *fakeSender) Send(ctx context.Context, m domain.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.sent = append(f.sent, m)
	if f.onSend != nil {
		f.onSend()
	}
	if f.failFor[m.To] {
		return errors.New("550 mailbox unavailable")
	}
	return nil
}

func readyBatch(emails ...string) domain.Batch {
	items := make([]domain.BatchItem, len(emails))
	for i, e := range emails {
		items[i] = domain.BatchItem{
			Recipient: domain.Recipient{Email: e, Fields: map[string]string{"nombre": e}},
			Status:    domain.StatusReady,
		}
	}
	return domain.Batch{Items: items, Template: domain.Template{Subject: "S", Body: "Hola {{nombre}}"}}
}

func TestRunBatch_SendsReadySkipsInvalidAndReportsFailures(t *testing.T) {
	batch := readyBatch("a@x.com", "b@x.com", "c@x.com")
	batch.Items[1].Status = domain.StatusInvalid
	sender := &fakeSender{failFor: map[string]bool{"c@x.com": true}}
	var events []Event

	res := RunBatch(context.Background(), &batch, sender, RunOptions{OnEvent: func(e Event) { events = append(events, e) }})

	if res != (Result{Sent: 1, Failed: 1}) {
		t.Errorf("result = %+v", res)
	}
	if batch.Items[0].Status != domain.StatusSent || batch.Items[1].Status != domain.StatusInvalid || batch.Items[2].Status != domain.StatusFailed {
		t.Errorf("statuses = %v %v %v", batch.Items[0].Status, batch.Items[1].Status, batch.Items[2].Status)
	}
	if sender.sent[0].Body != "Hola a@x.com" {
		t.Errorf("template not rendered: %q", sender.sent[0].Body)
	}
	// 2 items × (SENDING + final) events.
	if len(events) != 4 || events[0].Item.Status != domain.StatusSending {
		t.Fatalf("events = %+v", events)
	}
	last := events[len(events)-1]
	if last.Total != 2 || last.Processed != 2 || last.Failed != 1 || last.Index != 2 {
		t.Errorf("last event = %+v", last)
	}
}

func TestRunBatch_CancelMarksRemainingCancelled(t *testing.T) {
	batch := readyBatch("a@x.com", "b@x.com", "c@x.com")
	ctx, cancel := context.WithCancel(context.Background())
	sender := &fakeSender{onSend: cancel} // cancel right after the first send

	res := RunBatch(ctx, &batch, sender, RunOptions{})

	if res != (Result{Sent: 1, Cancelled: 2}) {
		t.Errorf("result = %+v", res)
	}
	if batch.Items[2].Status != domain.StatusCancelled {
		t.Errorf("status = %v", batch.Items[2].Status)
	}
}

func TestRunBatch_CancelInterruptsDelay(t *testing.T) {
	batch := readyBatch("a@x.com", "b@x.com")
	ctx, cancel := context.WithCancel(context.Background())
	sender := &fakeSender{onSend: cancel}

	start := time.Now()
	RunBatch(ctx, &batch, sender, RunOptions{Delay: time.Minute})
	if time.Since(start) > 5*time.Second {
		t.Error("cancellation did not interrupt the pause between sends")
	}
}

func TestPrepareRetry(t *testing.T) {
	batch := readyBatch("a", "b", "c", "d")
	batch.Items[0].Status = domain.StatusSent
	batch.Items[1].Status = domain.StatusFailed
	batch.Items[1].SendError = "boom"
	batch.Items[2].Status = domain.StatusCancelled
	batch.Items[3].Status = domain.StatusInvalid

	if n := PrepareRetry(&batch); n != 2 {
		t.Errorf("PrepareRetry = %d", n)
	}
	if batch.Items[0].Status != domain.StatusSent || batch.Items[1].Status != domain.StatusReady || batch.Items[1].SendError != "" || batch.Items[3].Status != domain.StatusInvalid {
		t.Errorf("unexpected statuses: %+v", batch.Items)
	}
}
