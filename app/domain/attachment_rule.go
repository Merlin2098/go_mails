package domain

// AttachmentMode selects how each recipient's files are found.
type AttachmentMode string

const (
	// AttachNone sends the email without attachments.
	AttachNone AttachmentMode = "none"
	// AttachPattern renders Pattern with the recipient's fields and attaches
	// every file in Folder whose name matches the resulting glob, e.g.
	// "CERT-{{documento_id}}_*.pdf" -> "CERT-1001_*.pdf".
	AttachPattern AttachmentMode = "pattern"
	// AttachColumn reads file names (or absolute paths) from a column of the
	// table; several files can be separated with ";".
	AttachColumn AttachmentMode = "column"
)

// AttachmentRule describes where a batch's documents live and how each one
// is associated to its recipient.
type AttachmentRule struct {
	Mode      AttachmentMode
	Folder    string
	Pattern   string
	Column    string
	Recursive bool
}
