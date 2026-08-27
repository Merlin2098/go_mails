package domain

// BatchItemStatus is the explicit lifecycle state of a single BatchItem.
// It is a named string (not an int/iota) because it is serialized as-is
// to the React frontend via Wails bindings and to the local JSON history.
type BatchItemStatus string

const (
	StatusPending    BatchItemStatus = "PENDING"
	StatusValidating BatchItemStatus = "VALIDATING"
	StatusInvalid    BatchItemStatus = "INVALID"
	StatusReady      BatchItemStatus = "READY"
	StatusSending    BatchItemStatus = "SENDING"
	StatusSent       BatchItemStatus = "SENT"
	StatusFailed     BatchItemStatus = "FAILED"
	StatusCancelled  BatchItemStatus = "CANCELLED"
)

// BatchItem is one employee/document pair being tracked through the send
// pipeline. Status is the single source of truth for where the item is in
// its lifecycle — no separate booleans (sent/failed/processing) that could
// contradict each other.
type BatchItem struct {
	Employee        Employee
	Document        Document
	Status          BatchItemStatus
	ValidationField string
	ValidationError string
	SendError       string
}
