# POC — Selección de cuenta Outlook Desktop

> **Nota de alcance (revisión 2026-08-27):** la versión original de este
> documento proponía autenticación vía Microsoft Graph + OAuth 2.0 + MSAL,
> tratando a Outlook Desktop como algo ajeno a la identidad de la app. El
> cliente pidió explícitamente lo contrario: que la aplicación detecte las
> cuentas ya configuradas en el perfil local de Outlook Desktop y permita
> elegir con cuál enviar. Esta revisión reemplaza el enfoque Graph/MSAL por
> ese requerimiento real, mucho más simple y sin dependencias externas
> (Azure AD, tenant, consentimiento de administrador). El histórico Graph/MSAL
> queda documentado al final como opción descartada, por si en el futuro se
> retoma.

## 1. Objetivo

Permitir que la aplicación desktop detecte las cuentas configuradas en el
perfil local de Outlook Classic y que el usuario elija explícitamente con
cuál de ellas enviar el lote de boletas.

La solución debe contemplar que el usuario puede tener múltiples cuentas
configuradas en Outlook Desktop (personales y/o corporativas).

---

## 2. Alcance

## Incluido

- Enumeración de cuentas configuradas en Outlook Classic vía COM
  (`Namespace.Accounts`).
- Selector explícito de cuenta en la UI antes de habilitar el envío.
- Envío usando la cuenta elegida (`MailItem.SendUsingAccount`).
- Persistencia de la elección solo durante la sesión de la app (no hay
  requerimiento de recordarla entre reinicios en esta POC).

## Fuera de alcance

- Microsoft Graph API.
- OAuth 2.0 / Microsoft identity platform / MSAL.
- Login interactivo por navegador.
- Azure AD / Entra ID app registration.
- Lectura de archivos PST/OST.
- Extracción de credenciales o tokens desde Outlook.
- Automatización de Outlook Desktop mediante UI o Selenium.
- Envío para cuentas que no estén configuradas en el Outlook Classic local.

---

## 3. Caso multi-cuenta

```text
Outlook Desktop

├── personal@hotmail.com
├── usuario@empresa.com
└── usuario@otraempresa.com
```

La aplicación NO debe asumir que la primera cuenta configurada en Outlook
(el "default account" de Outlook) es la cuenta que debe utilizar para
enviar. El usuario debe elegir explícitamente antes de poder enviar.

---

## 4. Principio arquitectónico

A diferencia del enfoque Graph/MSAL descartado, aquí Outlook Desktop **sí**
es la fuente de verdad de qué cuentas existen — porque es exactamente lo que
el cliente pidió: aprovechar la configuración que el usuario ya tiene en su
Outlook, sin pedirle un login adicional.

```text
Desktop App
    │
    ▼
Outlook.Application (COM)
    │
    ▼
Namespace.Accounts
    │
    ▼
Lista de cuentas configuradas
    │
    ▼
Usuario elige una explícitamente
    │
    ▼
MailItem.SendUsingAccount
    │
    ▼
Outlook Classic envía
```

Lo que sigue prohibido, y no cambia respecto al documento original:

- Leer contraseñas o tokens de Outlook.
- Leer archivos PST/OST para obtener credenciales.
- Asumir silenciosamente cuál es "la cuenta correcta" sin que el usuario la
  confirme.

---

## 5. UX propuesta

La aplicación debe mostrar, una vez confirmada la conexión con Outlook:

```text
Cuenta remitente

○ Juan Pérez (personal@hotmail.com)
○ Juan Pérez (usuario@empresa.com)
○ Juan Pérez (usuario@otraempresa.com)
```

El botón "Enviar" permanece deshabilitado hasta que:

1. Outlook esté disponible (`TestOutlookConnection`).
2. El usuario haya seleccionado una cuenta.
3. El lote esté validado con al menos un ítem válido.

---

## 6. Modelo de datos

```go
// app/infrastructure/outlook/connection.go
type Account struct {
    DisplayName string
    SmtpAddress string
}

func ListAccounts() ([]Account, error)
```

```go
// app/infrastructure/outlook/adapter.go
func Connect(senderAccount string) (*Adapter, error)
```

`senderAccount` es el `SmtpAddress` elegido por el usuario. Si se pasa
vacío, `Adapter.Send` no llama `SendUsingAccount` y Outlook usa su propio
comportamiento por defecto — pero los bindings de la app (`app.go`) nunca
permiten que `StartBatch`/`RetryFailed` corran sin una cuenta explícitamente
seleccionada (`errNoAccountSelected`).

---

## 7. Bindings Wails

```text
ListOutlookAccounts() -> []OutlookAccount
SelectOutlookAccount(smtpAddress string)
```

`SelectOutlookAccount` solo guarda la elección en memoria (`App.selectedAccount`)
para la sesión actual — no hay persistencia en disco en esta POC.

---

## 8. Casos de error

### Sin cuentas configuradas

```text
No se encontraron cuentas configuradas en Outlook.
```

El usuario debe configurar una cuenta en Outlook Classic antes de continuar.

### Envío sin cuenta seleccionada

```text
Selecciona una cuenta de Outlook antes de enviar.
```

`StartBatch`/`RetryFailed` devuelven este error inmediatamente, sin lanzar
el worker de envío.

### Cuenta eliminada de Outlook entre la selección y el envío

```text
La cuenta "<smtp>" ya no está configurada en Outlook.
```

Caso borde: el usuario selecciona una cuenta y luego la quita de Outlook
Classic antes de que termine el envío. `resolveAccount` en el adapter
detecta esto y falla ese envío con un error explícito, sin abortar el resto
del lote de forma distinta al manejo de errores por ítem ya existente.

---

## 9. Criterios de aceptación

- [ ] Puede enumerar todas las cuentas configuradas en Outlook Classic.
- [ ] Puede mostrar la lista en la UI y dejar elegir una explícitamente.
- [ ] No permite iniciar el envío sin una cuenta seleccionada.
- [ ] El envío usa la cuenta elegida, no la cuenta por defecto de Outlook.
- [ ] Funciona con una sola cuenta configurada.
- [ ] Funciona con múltiples cuentas configuradas (personales y/o
      corporativas).
- [ ] No extrae credenciales ni tokens de Outlook.
- [ ] No requiere Azure AD, OAuth, ni login adicional.

---

## Apéndice — Enfoque descartado: Microsoft Graph + MSAL

Se evaluó originalmente un flujo de autenticación independiente de Outlook
Desktop, vía Microsoft Graph API + OAuth 2.0 + MSAL (Authorization Code +
PKCE, login interactivo por navegador, cache de tokens cifrado, envío vía
`POST /me/sendMail`). Ese enfoque queda descartado para esta POC porque:

- Requiere una Azure AD / Entra ID app registration, que el cliente no
  tiene ni quiere gestionar en este momento.
- Introduce fricción operativa (consentimiento de administrador para
  cuentas corporativas, Conditional Access) no deseada para este caso de
  uso.
- El cliente prefiere reutilizar la configuración de cuentas que el usuario
  ya mantiene en Outlook Desktop, en vez de una identidad separada gestionada
  por la app.

Si en el futuro se requiere enviar sin depender de que Outlook Classic esté
instalado y configurado en la máquina (por ejemplo, para un despliegue sin
Outlook), este apéndice es el punto de partida para retomar el enfoque
Graph/MSAL como un segundo `domain.MailSender` — la interfaz ya está
preparada para admitir ambos transportes sin cambios en `RunBatch`.
