package desktop

import (
	"testing"

	"despacho/app/domain"
)

func item(row int, email string, status domain.BatchItemStatus) domain.BatchItem {
	return domain.BatchItem{Recipient: domain.Recipient{Row: row, Email: email}, Status: status}
}

// keepDelivered is the guard against re-sending: editing the template after
// a batch must not turn already delivered rows back into READY.
func TestKeepDelivered(t *testing.T) {
	prev := domain.Batch{Items: []domain.BatchItem{
		item(2, "a@x.com", domain.StatusSent),
		item(3, "b@x.com", domain.StatusFailed),
		item(4, "c@x.com", domain.StatusSent),
	}}
	next := domain.Batch{Items: []domain.BatchItem{
		item(2, "a@x.com", domain.StatusReady),   // same row and email -> stays SENT
		item(3, "b@x.com", domain.StatusReady),   // failed before -> may be retried
		item(4, "new@x.com", domain.StatusReady), // email changed -> it is a new recipient
	}}

	keepDelivered(&next, prev)

	want := []domain.BatchItemStatus{domain.StatusSent, domain.StatusReady, domain.StatusReady}
	for i, w := range want {
		if got := next.Items[i].Status; got != w {
			t.Errorf("row %d: status = %s, want %s", next.Items[i].Recipient.Row, got, w)
		}
	}
}

func TestKeepDelivered_DoesNotResurrectInvalid(t *testing.T) {
	prev := domain.Batch{Items: []domain.BatchItem{item(2, "a@x.com", domain.StatusSent)}}
	next := domain.Batch{Items: []domain.BatchItem{item(2, "a@x.com", domain.StatusInvalid)}}

	keepDelivered(&next, prev)

	if next.Items[0].Status != domain.StatusInvalid {
		t.Errorf("an item that is now invalid must stay invalid, got %s", next.Items[0].Status)
	}
}

func TestSuggestMapping(t *testing.T) {
	cases := map[string]struct {
		table domain.Table
		want  Mapping
	}{
		"common spanish headers": {
			domain.Table{Headers: []string{"Correo", "NOMBRE", "documento_id", "cc", "curso"}},
			Mapping{Email: "Correo", Name: "NOMBRE", Key: "documento_id", CC: "cc"},
		},
		"english headers": {
			domain.Table{Headers: []string{"name", "email"}},
			Mapping{Email: "email", Name: "name"},
		},
		"no known header: detects @ in first row": {
			domain.Table{Headers: []string{"persona", "contacto"}, Rows: [][]string{{"Ana", "ana@example.com"}}},
			Mapping{Email: "contacto"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := suggestMapping(c.table); got != c.want {
				t.Errorf("suggestMapping = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestBindingsExcludeServicesContainer(t *testing.T) {
	s := New(nil)
	for _, b := range s.Bindings() {
		if _, ok := b.(*Services); ok {
			t.Fatal("Services must not be bound: its OnStartup would become callable from JavaScript")
		}
	}
	if len(s.Bindings()) != 4 {
		t.Errorf("bindings = %d, want 4", len(s.Bindings()))
	}
}
