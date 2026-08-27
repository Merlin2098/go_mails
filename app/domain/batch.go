package domain

// Batch is the aggregate representing one full send run: the resolved
// items, the message template applied to all of them, and progress counts
// derived from item statuses.
type Batch struct {
	Items    []BatchItem
	Template MessageTemplate
}

// Summary derives aggregate counts from item statuses — there is no
// separate mutable counter to keep in sync, avoiding the scattered-booleans
// problem described in app/domain/batch_item.go.
type Summary struct {
	Total   int
	Valid   int
	Invalid int
}

func (b Batch) Summary() Summary {
	s := Summary{Total: len(b.Items)}
	for _, item := range b.Items {
		switch item.Status {
		case StatusInvalid:
			s.Invalid++
		default:
			s.Valid++
		}
	}
	return s
}
