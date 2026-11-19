package domain

// BatchItemStatus is the explicit lifecycle state of a single BatchItem.
// It is a named string (not an int/iota) because it crosses the Wails
// boundary as-is and is readable in logs and in the exported report.
type BatchItemStatus string

const (
	StatusInvalid   BatchItemStatus = "INVALID"
	StatusReady     BatchItemStatus = "READY"
	StatusSending   BatchItemStatus = "SENDING"
	StatusSent      BatchItemStatus = "SENT"
	StatusFailed    BatchItemStatus = "FAILED"
	StatusCancelled BatchItemStatus = "CANCELLED"
)

// BatchItem is one recipient tracked through the send pipeline. Status is
// the single source of truth for where the item is in its lifecycle — no
// separate booleans (sent/failed/processing) that could contradict each
// other.
type BatchItem struct {
	Recipient       Recipient
	Attachments     []string
	Status          BatchItemStatus
	ValidationField string
	ValidationError string
	SendError       string
}
