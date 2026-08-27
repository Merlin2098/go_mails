// Package outlook aísla toda interacción COM con Outlook Classic para
// Windows. Nada fuera de este paquete debe conocer go-ole, CLSIDs de Outlook,
// o el modelo de objetos de Outlook (Application, Namespace, MailItem).
package outlook

import (
	"fmt"
	"runtime"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// ConnectionResult is the outcome of probing for a running/installable
// Outlook Classic instance and reading its configured account.
type ConnectionResult struct {
	Available bool
	Email     string
	Detail    string
}

// TestConnection attempts to reach Outlook Classic via COM automation and
// read the SMTP address of its default account. It does not send anything
// and does not require Outlook to already be running — COM will launch it
// if needed, same as double-clicking a .msg file would.
func TestConnection() ConnectionResult {
	// LockOSThread pins this goroutine to its current OS thread for the
	// duration of the call: COM is apartment-threaded, so CoInitialize binds
	// to whichever OS thread calls it, and Go's scheduler is otherwise free
	// to migrate the goroutine to a different thread before CoUninitialize
	// runs. See the longer explanation on runBatchWorker in app.go, where
	// the same issue caused every send to fail with RPC_E_WRONG_THREAD.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return ConnectionResult{
			Available: false,
			Detail:    fmt.Sprintf("No se pudo inicializar COM: %v", err),
		}
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Outlook.Application")
	if err != nil {
		return ConnectionResult{
			Available: false,
			Detail:    "Outlook Classic no está instalado o no se pudo iniciar. Se requiere Outlook Classic para Windows configurado en este equipo.",
		}
	}
	defer unknown.Release()

	app, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return ConnectionResult{
			Available: false,
			Detail:    fmt.Sprintf("No se pudo conectar a Outlook.Application: %v", err),
		}
	}
	defer app.Release()

	namespaceRaw, err := oleutil.CallMethod(app, "GetNamespace", "MAPI")
	if err != nil {
		return ConnectionResult{
			Available: false,
			Detail:    fmt.Sprintf("No se pudo obtener el namespace MAPI: %v", err),
		}
	}
	namespace := namespaceRaw.ToIDispatch()
	defer namespace.Release()

	email, err := defaultAccountSMTPAddress(namespace)
	if err != nil {
		return ConnectionResult{
			Available: true,
			Email:     "",
			Detail:    fmt.Sprintf("Outlook está disponible pero no se pudo leer la cuenta configurada: %v", err),
		}
	}

	return ConnectionResult{
		Available: true,
		Email:     email,
		Detail:    "Outlook Classic detectado y disponible.",
	}
}

// defaultAccountSMTPAddress reads namespace.Session.Accounts.Item(1).SmtpAddress.
func defaultAccountSMTPAddress(namespace *ole.IDispatch) (string, error) {
	accountsRaw, err := oleutil.GetProperty(namespace, "Accounts")
	if err != nil {
		return "", fmt.Errorf("Namespace.Accounts: %w", err)
	}
	accounts := accountsRaw.ToIDispatch()
	defer accounts.Release()

	countRaw, err := oleutil.GetProperty(accounts, "Count")
	if err != nil {
		return "", fmt.Errorf("Accounts.Count: %w", err)
	}
	if countRaw.Val == 0 {
		return "", fmt.Errorf("no hay cuentas configuradas en este perfil de Outlook")
	}

	accountRaw, err := oleutil.CallMethod(accounts, "Item", 1)
	if err != nil {
		return "", fmt.Errorf("Accounts.Item(1): %w", err)
	}
	account := accountRaw.ToIDispatch()
	defer account.Release()

	smtpRaw, err := oleutil.GetProperty(account, "SmtpAddress")
	if err != nil {
		return "", fmt.Errorf("Account.SmtpAddress: %w", err)
	}
	return smtpRaw.ToString(), nil
}
