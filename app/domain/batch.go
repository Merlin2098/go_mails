package domain

// Batch is one full send run: the validated items plus the template applied
// to all of them.
type Batch struct {
	Items    []BatchItem
	Template Template
}

// Summary derives aggregate counts from item statuses — there is no
// separate mutable counter to keep in sync.
type Summary struct {
	Total     int
	Ready     int
	Invalid   int
	Sent      int
	Failed    int
	Cancelled int
}

func (b Batch) Summary() Summary {
	s := Summary{Total: len(b.Items)}
	for _, item := range b.Items {
		switch item.Status {
		case StatusInvalid:
			s.Invalid++
		case StatusReady, StatusSending:
			s.Ready++
		case StatusSent:
			s.Sent++
		case StatusFailed:
			s.Failed++
		case StatusCancelled:
			s.Cancelled++
		}
	}
	return s
}

// Clone returns a deep-enough copy of the batch for a worker goroutine to
// mutate freely: the Items slice is new, so writes by the worker can never
// race with readers of the original (the UI exporting a report, for
// example). Attachments and Fields are never mutated after validation, so
// sharing them is safe.
func (b Batch) Clone() Batch {
	items := make([]BatchItem, len(b.Items))
	copy(items, b.Items)
	return Batch{Items: items, Template: b.Template}
}
