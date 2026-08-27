# POC — Integración Gmail Desktop

## 1. Objetivo

Implementar una Proof of Concept (POC) que permita a la aplicación desktop enviar correos utilizando una cuenta de Gmail del usuario mediante:

- Gmail API
- OAuth 2.0
- Navegador local del usuario
- Selección explícita de cuenta Google
- Persistencia segura de credenciales/token

La POC debe demostrar que la aplicación puede:

1. Detectar navegadores disponibles.
2. Permitir al usuario seleccionar el navegador que desea utilizar.
3. Abrir el flujo OAuth de Google en dicho navegador.
4. Permitir al usuario seleccionar la cuenta de Google correspondiente.
5. Obtener autorización para enviar correos.
6. Enviar un correo mediante Gmail API.
7. Mantener la sesión/autorización para posteriores ejecuciones.
8. Revocar/desconectar la cuenta desde la aplicación.

---

# 2. Alcance

## Incluido

- Gmail API.
- OAuth 2.0 para aplicación desktop.
- Scope mínimo requerido para envío.
- Selección de navegador.
- Selección de cuenta Google durante OAuth.
- Envío de correo.
- Envío de archivos adjuntos.
- Persistencia del refresh token.
- Logout/desconexión.
- Manejo básico de errores.
- Tests de integración.

## Fuera de alcance

- Lectura masiva de correos.
- Gestión de bandeja de entrada.
- Eliminación de correos.
- Gestión de etiquetas.
- Automatización mediante Selenium.
- Extracción de cookies del navegador.
- Extracción de tokens desde perfiles Chrome/Edge.
- Automatización de Gmail Web.
- Uso de Google Apps Script como backend.

---

# 3. Principio arquitectónico

La aplicación NO debe utilizar las sesiones existentes del navegador como mecanismo de autenticación.

No se debe implementar:

```text
Aplicación
   ↓
Perfil Chrome
   ↓
Cookies
   ↓
Sesión Gmail
```

Ni:

```text
Aplicación
   ↓
Cookies del navegador
   ↓
Gmail Web
```

La arquitectura correcta es:

```text
Desktop App
    │
    │ Selección de navegador
    ▼
Navegador del usuario
    │
    │ Google OAuth 2.0
    ▼
Google
    │
    │ Authorization Code
    ▼
Desktop App
    │
    │ Access / Refresh Token
    ▼
Gmail API
```

El navegador únicamente participa en la experiencia de autenticación.

Las credenciales OAuth deben ser gestionadas por la aplicación.

---

# 4. Multi-browser

La aplicación debe detectar navegadores instalados en el equipo.

Ejemplos:

- Google Chrome
- Microsoft Edge
- Mozilla Firefox
- Brave
- Otros navegadores compatibles

La aplicación debe representar cada navegador mediante:

```text
Browser
├── name
├── executable_path
└── available
```

Ejemplo:

```json
{
  "name": "Google Chrome",
  "executable_path": "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
  "available": true
}
```

La detección debe ser específica para cada sistema operativo.

---

# 5. Selección del navegador

Antes de iniciar OAuth, mostrar:

```text
Selecciona el navegador

( ) Google Chrome
( ) Microsoft Edge
( ) Firefox
( ) Brave

[Continuar]
```

La selección determina únicamente dónde se abrirá el flujo de autenticación.

No significa que la aplicación accederá al perfil del navegador.

---

# 6. Selección de cuenta Gmail

Una persona puede tener varias cuentas Google autenticadas en el navegador.

Ejemplo:

```text
Chrome
├── usuario.personal@gmail.com
├── usuario.trabajo@gmail.com
└── usuario.empresa@gmail.com
```

La aplicación debe permitir que Google gestione la selección de cuenta.

Durante OAuth, utilizar un flujo que permita mostrar el selector de cuentas cuando corresponda.

UX esperada:

```text
Aplicación
    ↓
Seleccionar navegador
    ↓
Abrir Google OAuth
    ↓
Google muestra cuentas disponibles
    ↓
Usuario selecciona cuenta
    ↓
Usuario concede permisos
    ↓
Google devuelve autorización
    ↓
Aplicación recibe token
```

No intentar determinar la cuenta mediante scraping de Gmail.

---

# 7. Cuenta autorizada

Después del OAuth, la aplicación debe asociar el token con la cuenta Google autorizada.

Ejemplo:

```text
Cuenta conectada

usuario.empresa@gmail.com

Estado:
● Conectada

[Enviar correo]
[Desconectar]
```

La aplicación debe obtener la identidad de la cuenta mediante una API/autorización válida, no mediante información extraída del navegador.

---

# 8. Scopes

La POC debe utilizar el scope mínimo necesario.

Para envío de correo:

```text
https://www.googleapis.com/auth/gmail.send
```

Evitar inicialmente scopes amplios como:

```text
gmail.modify
gmail.readonly
gmail.full_access
```

si no son necesarios.

Principio:

> Solicitar únicamente los permisos requeridos por la funcionalidad actual.

---

# 9. Persistencia de autenticación

Después del primer login:

```text
OAuth
  ↓
Access Token
  +
Refresh Token
```

El refresh token debe almacenarse de forma segura.

No almacenar:

- tokens en archivos públicos del proyecto;
- tokens en Git;
- tokens en logs;
- tokens dentro del frontend;
- tokens dentro de configuración distribuida con la aplicación.

El access token puede renovarse utilizando el refresh token.

---

# 10. Modelo de sesión

Conceptualmente:

```text
GmailAccount
├── email
├── provider = gmail
├── access_token
├── refresh_token
├── expires_at
└── connected_at
```

La implementación concreta debe evitar exponer tokens al frontend innecesariamente.

---

# 11. Envío de correo

La aplicación debe construir el mensaje MIME y enviarlo utilizando Gmail API.

Ejemplo conceptual:

```text
MailService
    │
    ├── recipient
    ├── subject
    ├── body
    └── attachments
            │
            ▼
        MIME Message
            │
            ▼
        Gmail API
            │
            ▼
         Sent Mail
```

La POC debe soportar como mínimo:

- destinatario;
- asunto;
- cuerpo;
- un archivo adjunto.

---

# 12. Arquitectura sugerida

Separar el proveedor Gmail de la lógica general de envío.

```text
MailAutomation
│
├── MailService
│
├── providers
│   ├── gmail
│   │   ├── GmailAuth
│   │   ├── GmailClient
│   │   └── GmailSender
│   │
│   └── outlook
│       └── ...
│
├── attachments
├── templates
├── recipients
└── logging
```

La lógica de negocio no debería depender directamente de Gmail API.

Ejemplo:

```text
MailService.send(message)
```

en lugar de:

```text
GmailApi.send(...)
```

---

# 13. Estados de conexión

La UI debería contemplar:

```text
DISCONNECTED
AUTHENTICATING
CONNECTED
TOKEN_EXPIRED
ERROR
DISCONNECTING
```

Ejemplo:

```text
Gmail

● Connected
usuario.empresa@gmail.com

[Enviar prueba]
[Desconectar]
```

---

# 14. Desconexión

El usuario debe poder desconectar la cuenta.

La operación debe:

1. Revocar/eliminar la autorización cuando corresponda.
2. Eliminar los tokens almacenados localmente.
3. Eliminar el estado de sesión.
4. Mostrar nuevamente:

```text
Gmail

○ No conectado

[Conectar Gmail]
```

---

# 15. Manejo de errores

La POC debe manejar como mínimo:

### OAuth cancelado

```text
El usuario canceló la autenticación.
```

### Permisos insuficientes

```text
La cuenta no concedió los permisos necesarios.
```

### Token inválido

```text
La sesión de Gmail requiere autenticación nuevamente.
```

### Cuenta incorrecta

```text
Se conectó una cuenta diferente a la esperada.
```

### Error Gmail API

Registrar:

- HTTP status;
- error code;
- mensaje técnico;
- contexto de operación.

No registrar tokens.

---

# 16. Seguridad

## Prohibido

No implementar:

- extracción de cookies;
- lectura de bases de datos de Chrome;
- extracción de tokens de navegador;
- reutilización de cookies Gmail;
- scraping de Gmail;
- almacenamiento de passwords;
- automatización de login mediante Selenium.

## Permitido

- OAuth 2.0;
- navegador del sistema;
- Authorization Code;
- access token;
- refresh token;
- Gmail API.

---

# 17. Test Plan

## T01 — Un navegador / una cuenta

Precondición:

```text
Chrome instalado
Cuenta Gmail autenticada
```

Esperado:

```text
Chrome
→ Google OAuth
→ selección de cuenta
→ autorización
→ aplicación conectada
```

---

## T02 — Varios navegadores

Instalar:

- Chrome
- Edge
- Firefox

Esperado:

```text
Aplicación
→ muestra navegadores disponibles
→ usuario selecciona uno
→ OAuth se abre en navegador seleccionado
```

---

## T03 — Varias cuentas Google

Precondición:

```text
Chrome
├── accountA@gmail.com
└── accountB@gmail.com
```

Esperado:

```text
OAuth
→ Google muestra cuentas
→ usuario selecciona accountB
→ aplicación queda conectada a accountB
```

---

## T04 — Cuenta incorrecta

Seleccionar una cuenta diferente a la esperada.

Esperado:

```text
Aplicación identifica la cuenta autorizada
y muestra claramente qué cuenta quedó conectada.
```

---

## T05 — Reinicio de aplicación

1. Conectar Gmail.
2. Cerrar aplicación.
3. Abrir aplicación nuevamente.

Esperado:

```text
Cuenta sigue conectada
```

sin solicitar login nuevamente, siempre que la autorización siga siendo válida.

---

## T06 — Envío simple

Enviar:

```text
To: test@example.com
Subject: Gmail POC
Body: Test
```

Esperado:

```text
HTTP 200 / operación exitosa
Correo recibido
```

---

## T07 — Envío con adjunto

Enviar:

```text
document.pdf
```

Esperado:

```text
Correo recibido
+
PDF correctamente adjuntado
```

---

## T08 — Revocación

1. Desconectar cuenta.
2. Eliminar credenciales locales.
3. Intentar enviar.

Esperado:

```text
Aplicación solicita autenticación nuevamente.
```

---

# 18. Criterios de aceptación

La POC se considera exitosa si:

- [ ] Puede detectar al menos dos navegadores.
- [ ] Permite seleccionar navegador.
- [ ] Puede iniciar OAuth.
- [ ] Puede seleccionar entre múltiples cuentas Google.
- [ ] Puede identificar la cuenta conectada.
- [ ] Puede enviar correo.
- [ ] Puede enviar adjuntos.
- [ ] Puede persistir autorización.
- [ ] Puede recuperar sesión después de reiniciar.
- [ ] Puede desconectar la cuenta.
- [ ] No utiliza Selenium.
- [ ] No extrae cookies.
- [ ] No almacena contraseñas.
- [ ] No expone tokens al frontend innecesariamente.
