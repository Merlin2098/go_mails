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

	accounts, err := allAccountSMTPAddresses(namespace)
	if err != nil {
		return ConnectionResult{
			Available: true,
			Email:     "",
			Detail:    fmt.Sprintf("Outlook está disponible pero no se pudo leer las cuentas configuradas: %v", err),
		}
	}

	email := ""
	if len(accounts) > 0 {
		email = accounts[0]
	}

	return ConnectionResult{
		Available: true,
		Email:     email,
		Detail:    "Outlook Classic detectado y disponible.",
	}
}

// Account is one Microsoft account configured in the user's Outlook Classic
// profile, as reported by Namespace.Accounts. It is never used to read
// credentials — only the display name Outlook already has for its own
// configured accounts (docs/specs/POC_Outlook_Hotmail_Desktop.md §6).
type Account struct {
	// DisplayName is the name shown by Outlook Namespace.Accounts.Item(i).DisplayName.
	DisplayName string
	// SmtpAddress is the account's SMTP address, used to select it as the
	// sender via MailItem.SendUsingAccount.
	SmtpAddress string
}

// ListAccounts enumerates every account configured in the local Outlook
// Classic profile (Namespace.Accounts), so the application never assumes
// the first/default account is the one the user wants to send from — the
// user must pick explicitly (docs/specs/POC_Outlook_Hotmail_Desktop.md §3).
func ListAccounts() ([]Account, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return nil, fmt.Errorf("no se pudo inicializar COM: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Outlook.Application")
	if err != nil {
		return nil, fmt.Errorf("Outlook Classic no está disponible: %w", err)
	}
	defer unknown.Release()

	app, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a Outlook.Application: %w", err)
	}
	defer app.Release()

	namespaceRaw, err := oleutil.CallMethod(app, "GetNamespace", "MAPI")
	if err != nil {
		return nil, fmt.Errorf("no se pudo obtener el namespace MAPI: %w", err)
	}
	namespace := namespaceRaw.ToIDispatch()
	defer namespace.Release()

	return listAccounts(namespace)
}

// listAccounts reads Namespace.Accounts.Item(1..Count), returning each
// account's DisplayName and SmtpAddress.
func listAccounts(namespace *ole.IDispatch) ([]Account, error) {
	accountsRaw, err := oleutil.GetProperty(namespace, "Accounts")
	if err != nil {
		return nil, fmt.Errorf("Namespace.Accounts: %w", err)
	}
	accountsCollection := accountsRaw.ToIDispatch()
	defer accountsCollection.Release()

	countRaw, err := oleutil.GetProperty(accountsCollection, "Count")
	if err != nil {
		return nil, fmt.Errorf("Accounts.Count: %w", err)
	}
	count := int(countRaw.Val)
	if count == 0 {
		return nil, fmt.Errorf("no hay cuentas configuradas en este perfil de Outlook")
	}

	accounts := make([]Account, 0, count)
	for i := 1; i <= count; i++ {
		accountRaw, err := oleutil.CallMethod(accountsCollection, "Item", i)
		if err != nil {
			return nil, fmt.Errorf("Accounts.Item(%d): %w", i, err)
		}
		account := accountRaw.ToIDispatch()

		displayNameRaw, err := oleutil.GetProperty(account, "DisplayName")
		if err != nil {
			account.Release()
			return nil, fmt.Errorf("Account.DisplayName (item %d): %w", i, err)
		}
		smtpRaw, err := oleutil.GetProperty(account, "SmtpAddress")
		if err != nil {
			account.Release()
			return nil, fmt.Errorf("Account.SmtpAddress (item %d): %w", i, err)
		}
		account.Release()

		accounts = append(accounts, Account{
			DisplayName: displayNameRaw.ToString(),
			SmtpAddress: smtpRaw.ToString(),
		})
	}
	return accounts, nil
}

// allAccountSMTPAddresses returns just the SMTP addresses, used by
// TestConnection to report the first configured account without requiring
// callers that only need a single status string to depend on Account.
func allAccountSMTPAddresses(namespace *ole.IDispatch) ([]string, error) {
	accounts, err := listAccounts(namespace)
	if err != nil {
		return nil, err
	}
	addresses := make([]string, len(accounts))
	for i, a := range accounts {
		addresses[i] = a.SmtpAddress
	}
	return addresses, nil
}
