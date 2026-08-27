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
	app           *ole.IDispatch
	namespace     *ole.IDispatch
	senderAccount string // SMTP address; empty means Outlook's own default
}

var _ domain.MailSender = (*Adapter)(nil)

// Connect initializes COM on the calling goroutine's OS thread and opens
// Outlook.Application. The caller must call Close when done, and must keep
// calling Send from the same goroutine that called Connect (COM apartment
// threading requirement).
//
// senderAccount is the SMTP address of the configured Outlook account to
// send from (as returned by ListAccounts). Pass "" to use whichever account
// Outlook treats as its own default — the application must never assume
// that silently; callers should always let the user pick explicitly
// (docs/specs/POC_Outlook_Hotmail_Desktop.md §6).
func Connect(senderAccount string) (*Adapter, error) {
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

	namespaceRaw, err := oleutil.CallMethod(app, "GetNamespace", "MAPI")
	if err != nil {
		app.Release()
		ole.CoUninitialize()
		return nil, fmt.Errorf("no se pudo obtener el namespace MAPI: %w", err)
	}

	return &Adapter{app: app, namespace: namespaceRaw.ToIDispatch(), senderAccount: senderAccount}, nil
}

// Close releases the Outlook.Application reference and uninitializes COM on
// this thread. Must be called from the same goroutine that called Connect.
func (a *Adapter) Close() {
	if a.namespace != nil {
		a.namespace.Release()
		a.namespace = nil
	}
	if a.app != nil {
		a.app.Release()
		a.app = nil
	}
	ole.CoUninitialize()
}

// resolveAccount finds the Namespace.Accounts entry matching smtpAddress.
// The caller must Release the returned *ole.IDispatch.
func (a *Adapter) resolveAccount(smtpAddress string) (*ole.IDispatch, error) {
	accountsRaw, err := oleutil.GetProperty(a.namespace, "Accounts")
	if err != nil {
		return nil, fmt.Errorf("Namespace.Accounts: %w", err)
	}
	accounts := accountsRaw.ToIDispatch()
	defer accounts.Release()

	countRaw, err := oleutil.GetProperty(accounts, "Count")
	if err != nil {
		return nil, fmt.Errorf("Accounts.Count: %w", err)
	}
	count := int(countRaw.Val)

	for i := 1; i <= count; i++ {
		accountRaw, err := oleutil.CallMethod(accounts, "Item", i)
		if err != nil {
			return nil, fmt.Errorf("Accounts.Item(%d): %w", i, err)
		}
		account := accountRaw.ToIDispatch()

		smtpRaw, err := oleutil.GetProperty(account, "SmtpAddress")
		if err != nil {
			account.Release()
			return nil, fmt.Errorf("Account.SmtpAddress (item %d): %w", i, err)
		}
		if smtpRaw.ToString() == smtpAddress {
			return account, nil
		}
		account.Release()
	}

	return nil, fmt.Errorf("la cuenta %q ya no está configurada en Outlook", smtpAddress)
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

	if a.senderAccount != "" {
		account, err := a.resolveAccount(a.senderAccount)
		if err != nil {
			return err
		}
		defer account.Release()

		if _, err := oleutil.CallMethod(mailItem, "SendUsingAccount", account); err != nil {
			return fmt.Errorf("no se pudo establecer la cuenta remitente %q: %w", a.senderAccount, err)
		}
	}

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
