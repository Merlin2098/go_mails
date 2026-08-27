package outlook

import (
	"context"
	"fmt"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	"boleta-mailer/app/domain"
	"boleta-mailer/app/infrastructure/winpath"
)

// olMailItem is the Outlook object type constant for a new email
// (Outlook's OlItemType enumeration, value 0).
const olMailItem = 0

// Adapter implements domain.MailSender against Outlook Classic via COM. It
// holds one open connection to Outlook.Application for the lifetime of a
// batch, instead of reconnecting per message — reconnecting per send would
// be wasteful and is not how a human using Outlook would operate either.
//
// Not safe for concurrent use: the batch worker that owns an Adapter must
// call Send from a single goroutine, matching the "one Outlook worker"
// decision in docs/boleta_mailer_mvp_context.md §13.
type Adapter struct {
	app *ole.IDispatch
}

var _ domain.MailSender = (*Adapter)(nil)

// Connect initializes COM on the calling goroutine's OS thread and opens
// Outlook.Application. The caller must call Close when done, and must keep
// calling Send from the same goroutine that called Connect (COM apartment
// threading requirement).
func Connect() (*Adapter, error) {
	if err := ole.CoInitialize(0); err != nil {
		return nil, fmt.Errorf("no se pudo inicializar COM: %w", err)
	}

	unknown, err := oleutil.CreateObject("Outlook.Application")
	if err != nil {
		ole.CoUninitialize()
		return nil, fmt.Errorf("Outlook Classic no está disponible: %w", err)
	}
	defer unknown.Release()

	app, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		ole.CoUninitialize()
		return nil, fmt.Errorf("no se pudo conectar a Outlook.Application: %w", err)
	}

	return &Adapter{app: app}, nil
}

// Close releases the Outlook.Application reference and uninitializes COM on
// this thread. Must be called from the same goroutine that called Connect.
func (a *Adapter) Close() {
	if a.app != nil {
		a.app.Release()
		a.app = nil
	}
	ole.CoUninitialize()
}

// Send creates a MailItem, sets recipient/subject/body, attaches the
// document if any, and sends it immediately. ctx is checked before starting
// — cancellation takes effect between items, not mid-send (context.md §22:
// cooperative cancellation, never kill a send in progress).
func (a *Adapter) Send(ctx context.Context, message domain.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mailItemRaw, err := oleutil.CallMethod(a.app, "CreateItem", olMailItem)
	if err != nil {
		return fmt.Errorf("no se pudo crear el mensaje: %w", err)
	}
	mailItem := mailItemRaw.ToIDispatch()
	defer mailItem.Release()

	if _, err := oleutil.PutProperty(mailItem, "To", message.To); err != nil {
		return fmt.Errorf("no se pudo establecer el destinatario: %w", err)
	}
	if _, err := oleutil.PutProperty(mailItem, "Subject", message.Subject); err != nil {
		return fmt.Errorf("no se pudo establecer el asunto: %w", err)
	}
	if _, err := oleutil.PutProperty(mailItem, "Body", message.Body); err != nil {
		return fmt.Errorf("no se pudo establecer el cuerpo: %w", err)
	}

	if message.AttachmentPath != "" {
		attachmentsRaw, err := oleutil.GetProperty(mailItem, "Attachments")
		if err != nil {
			return fmt.Errorf("no se pudo acceder a los adjuntos: %w", err)
		}
		attachments := attachmentsRaw.ToIDispatch()
		defer attachments.Release()

		attachmentPath := winpath.ExtendedLengthPrefix(message.AttachmentPath)
		if _, err := oleutil.CallMethod(attachments, "Add", attachmentPath); err != nil {
			return fmt.Errorf("no se pudo adjuntar el archivo %q: %w", message.AttachmentPath, err)
		}
	}

	if _, err := oleutil.CallMethod(mailItem, "Send"); err != nil {
		return fmt.Errorf("Outlook rechazó el envío: %w", err)
	}

	return nil
}
