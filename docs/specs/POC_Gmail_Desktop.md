# POC — Integración Gmail Desktop (SMTP + contraseña de aplicación)

> **Nota de alcance (revisión 2026-08-27):** la versión original de este
> documento proponía OAuth 2.0 completo con selector de navegador, login
> interactivo de Google, y envío vía Gmail API — lo que requiere crear un
> proyecto en Google Cloud Console (client_id/secret, OAuth consent screen,
> habilitar Gmail API). El cliente confirmó explícitamente que no cuenta con
> Google Cloud habilitado y que el envío debe resolverse "mediante conexión
> con el navegador web con la cuenta de Gmail regular" — es decir, sin
> gestionar infraestructura OAuth propia. Como Gmail API siempre exige una
> app registrada para hacer OAuth (a diferencia de Outlook, donde existía el
> atajo de leer cuentas ya configuradas vía COM), la alternativa viable es
> **SMTP con una "contraseña de aplicación"** generada por el propio usuario
> en su cuenta de Google — sin ningún proyecto GCP, sin navegador embebido
> en la app, sin dependencias nuevas (usa `net/smtp` de la stdlib de Go).
> El enfoque OAuth2/Gmail API original queda documentado al final como
> opción descartada, por si en el futuro se retoma.

## 1. Objetivo

Permitir que la aplicación desktop envíe correos usando una cuenta de Gmail
personal, autenticando por SMTP con una contraseña de aplicación generada
por el usuario — sin flujo OAuth, sin selector de navegador, sin backend de
Google Cloud.

## 2. Alcance

## Incluido

- Envío vía SMTP (`smtp.gmail.com:587`, STARTTLS) usando `net/smtp` de la
  stdlib de Go — sin dependencias nuevas.
- Validación de credenciales (correo + contraseña de aplicación) antes de
  habilitar el envío, sin enviar correo de prueba en ese paso.
- Envío de correo con un adjunto (mismo modelo que `domain.Message`: un
  `AttachmentPath` por mensaje).
- Envío de correo de prueba manual desde la UI.
- Selector de proveedor en la UI (Outlook / Gmail) para el envío de lotes.

## Fuera de alcance

- OAuth 2.0 / Microsoft-identity-platform-style login por navegador.
- Google Cloud Console / app registration / Gmail API.
- Selector de navegador.
- Selección de cuenta múltiple vía Google (solo se soporta una cuenta Gmail
  configurada por sesión, igual que Outlook soporta una cuenta seleccionada
  a la vez).
- Persistencia de la contraseña de aplicación en disco (se mantiene solo en
  memoria durante la sesión, igual que la cuenta Outlook seleccionada).
- Revocación remota de credenciales (el usuario revoca la contraseña de
  aplicación directamente desde su cuenta de Google si lo necesita).

---

## 3. Qué es una "contraseña de aplicación"

Es un código de 16 caracteres que Google genera para permitir que apps que
no soportan el login interactivo completo de Google (como esta) se
autentiquen sin usar la contraseña real de la cuenta. Requiere que la cuenta
tenga la verificación en dos pasos activada.

```text
Usuario
   │
   ▼
myaccount.google.com/apppasswords
   │
   ▼
Genera contraseña de 16 caracteres
   │
   ▼
Pega la contraseña en BoletaMailer
```

La aplicación nunca ve ni almacena la contraseña real de la cuenta de
Google — solo esta contraseña de aplicación, y solo en memoria durante la
sesión actual.

---

## 4. Arquitectura

```text
Desktop App
    │
    ▼
net/smtp (STARTTLS)
    │
    ▼
smtp.gmail.com:587
    │
    ▼
Autenticación con contraseña de aplicación
    │
    ▼
Envío del mensaje
```

`domain.MailSender` sigue siendo el boundary de transporte — `gmail.Adapter`
lo implementa exactamente igual que `outlook.Adapter`, sin que el dominio ni
`RunBatch` conozcan SMTP.

---

## 5. UX propuesta

```text
Proveedor de correo
○ Outlook   ● Gmail

Cuenta de Gmail

[correo@gmail.com________]  [contraseña de aplicación__]

Necesitas una contraseña de aplicación (no tu contraseña normal de Gmail).
Requiere verificación en dos pasos activada. [Generar contraseña de aplicación]

[Probar conexión]  ✓ Conexión con Gmail establecida

[Conectar]
```

Después de conectar:

```text
● Conectado (correo@gmail.com)

[Enviar prueba]  [Desconectar]
```

El botón "Enviar" del lote de boletas permanece deshabilitado hasta que:

1. El proveedor seleccionado sea Gmail y la cuenta esté conectada
   (`SelectGmailAccount` llamado tras una prueba de conexión exitosa), o el
   proveedor sea Outlook con una cuenta seleccionada.
2. El lote esté validado con al menos un ítem válido.

---

## 6. Modelo de datos

```go
// app/infrastructure/gmail/adapter.go
type Adapter struct { email, appPassword string }
func NewAdapter(email, appPassword string) *Adapter
func (a *Adapter) Send(ctx context.Context, message domain.Message) error

// app/infrastructure/gmail/connection.go
type ConnectionResult struct { Available bool; Email, Detail string }
func TestConnection(email, appPassword string) ConnectionResult
```

`Adapter.Send` construye el mensaje MIME (con o sin adjunto,
`app/infrastructure/gmail/mime.go`) y lo entrega vía
`net/smtp.SendMail` autenticado con `smtp.PlainAuth`.

---

## 7. Bindings Wails

```text
TestGmailConnection(email, appPassword string) -> GmailStatus
SelectGmailAccount(email, appPassword string)
DisconnectGmailAccount()
SendTestGmail(to string) error
OpenGmailAppPasswordsPage()
```

`OpenGmailAppPasswordsPage` abre `myaccount.google.com/apppasswords` en el
navegador predeterminado del sistema (`wailsruntime.BrowserOpenURL`) — es
únicamente conveniencia de presentación: no detecta navegadores instalados
ni participa en ningún flujo de login: el sistema operativo decide qué
navegador maneja la URL, igual que al hacer doble clic en cualquier enlace.

`SelectGmailAccount` guarda las credenciales en memoria y hace de Gmail el
proveedor activo para `StartBatch`/`RetryFailed` — análogo a
`SelectOutlookAccount`, que hace lo mismo para Outlook. Solo un proveedor
puede estar activo a la vez; seleccionar uno no borra las credenciales del
otro, pero sí determina cuál usa el siguiente envío de lote.

---

## 8. Casos de error

### Credenciales rechazadas

```text
Gmail rechazó las credenciales. Verifica el correo y la contraseña de
aplicación (16 caracteres, generada en myaccount.google.com/apppasswords).
```

### Envío sin proveedor conectado

```text
Selecciona y conecta una cuenta de Outlook o Gmail antes de enviar.
```

### Error de red/TLS al conectar

Se reporta el detalle técnico devuelto por `net/smtp`/`crypto/tls`, sin
tokens ni contraseñas en el mensaje ni en los logs.

---

## 9. Límite de envíos diarios (restricción externa, no técnica)

Gmail impone un límite de **destinatarios por día** sobre la cuenta misma,
independiente del mecanismo usado para enviar (SMTP, como implementa esta
POC, o la API REST de Gmail). No es un límite que este proyecto pueda
evitar con código — es una política del lado de Google atada a la cuenta.

| Tipo de cuenta                  | Límite aproximado          |
|----------------------------------|-----------------------------|
| Gmail personal (gratuita)        | 500 destinatarios/día       |
| Google Workspace (de pago)       | 2 000 destinatarios/día     |

Notas relevantes para BoletaMailer:

- El límite cuenta **destinatarios**, no "correos enviados" — un lote de
  500 boletas a 500 destinatarios distintos consume el límite diario
  completo de una cuenta gratuita, incluso si no se envía nada más ese día
  desde esa cuenta.
- Cambiar de SMTP a la API de Gmail (ver Apéndice) **no evita este límite**
  — la cuota de la API mide otra cosa (unidades de solicitud, no
  destinatarios) y es mucho más alta, pero el límite real de envío sigue
  siendo el de la cuenta subyacente.
- No existe una versión "Gmail Desktop" con un límite distinto — a
  diferencia de Outlook Classic (una aplicación real con su propio motor de
  envío vía Exchange/MAPI), Gmail no tiene un cliente de escritorio nativo;
  usar el navegador, SMTP o la API es, a estos efectos, el mismo canal con
  el mismo límite de cuenta.
- Al superar el límite, Gmail rechaza los envíos siguientes con un error
  SMTP como `454 4.7.0 ... Daily user sending limit exceeded` hasta que el
  límite se reinicia (~24 horas) — `gmail.Adapter.Send` propaga ese error
  tal cual lo recibe de `net/smtp`, sin un mensaje especial para este caso
  todavía.

Opciones si el volumen de boletas se acerca o supera el límite:

1. **Google Workspace** — sube el límite a 2 000/día sin cambiar nada del
   código (mismo `gmail.Adapter` vía SMTP).
2. **Repartir el lote entre varias cuentas Gmail** — operativo, no
   automatizado hoy: la app permite conectar una cuenta a la vez
   (`SelectGmailAccount`), pero no divide un lote entre cuentas por sí sola.
3. **Un proveedor SMTP transaccional dedicado** (SendGrid, Amazon SES,
   Mailgun, etc.) — pensado para este caso de uso, límites mucho más altos
   y mejor entregabilidad. Requeriría un nuevo `MailSender` adicional, mismo
   patrón que `outlook.Adapter`/`gmail.Adapter`.
4. **Usar Outlook para volumen alto** — el límite de envío de Outlook
   Classic (impuesto por el servidor Exchange/Outlook.com de la cuenta, no
   por este proyecto) suele ser más generoso que el de Gmail gratuito;
   puede ser el canal más apto si el volumen de boletas es alto y
   constante.

---

## 10. Criterios de aceptación

- [ ] Puede validar una cuenta de Gmail con contraseña de aplicación sin
      enviar correo.
- [ ] Puede conectar la cuenta tras una validación exitosa.
- [ ] Puede enviar un correo de prueba.
- [ ] Puede enviar boletas con adjunto vía el flujo de lote existente.
- [ ] No permite iniciar el envío de lote sin un proveedor conectado
      (Outlook o Gmail).
- [ ] Puede desconectar la cuenta de Gmail y volver a estado "No conectado".
- [ ] No requiere Google Cloud Console, OAuth, ni navegador embebido.
- [ ] No almacena la contraseña de aplicación en disco ni en logs.

---

## Apéndice — Enfoque descartado: OAuth 2.0 + Gmail API + selector de navegador

Se evaluó originalmente un flujo de autenticación completo vía OAuth 2.0
(Authorization Code + PKCE, login interactivo con selector de navegador
explícito, cache de refresh token cifrado con DPAPI, envío vía
`POST /gmail/v1/users/me/messages/send`). Ese enfoque queda descartado para
esta POC porque:

- Requiere crear y mantener un proyecto en Google Cloud Console
  (credenciales OAuth, consent screen, habilitar Gmail API), que el cliente
  no tiene ni quiere gestionar.
- El cliente prefiere una experiencia más directa: pegar una contraseña de
  aplicación generada una sola vez, sin flujo de navegador dentro de la app.

Si en el futuro se requiere soporte multi-cuenta Gmail con selección de
cuenta durante el login, o eliminar la dependencia de que el usuario
gestione manualmente la contraseña de aplicación, este apéndice es el punto
de partida para retomar OAuth2 + Gmail API — `gmail.Adapter` seguiría
implementando `domain.MailSender` sin cambios en `RunBatch`.
