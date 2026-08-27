# POC — Integración Outlook / Hotmail Desktop

## 1. Objetivo

Implementar una Proof of Concept que permita a la aplicación desktop enviar correos utilizando una cuenta Microsoft mediante:

- Microsoft Graph API
- OAuth 2.0 / Microsoft identity platform
- Microsoft Authentication Library (MSAL)
- Navegador para autenticación
- Selección de cuenta
- Persistencia segura de tokens

La solución debe contemplar que el usuario puede tener múltiples cuentas configuradas en Outlook Desktop.

---

# 2. Alcance

## Incluido

- Microsoft Graph.
- OAuth 2.0.
- MSAL.
- Login Microsoft.
- Selección de cuenta.
- Identificación de cuenta autenticada.
- Envío de correos.
- Archivos adjuntos.
- Persistencia de sesión.
- Desconexión.
- Tests de integración.

## Fuera de alcance

- Automatización de Outlook Desktop mediante UI.
- Automatización con Selenium.
- Lectura de archivos PST/OST.
- Extracción de credenciales desde Outlook.
- Extracción de tokens/cookies.
- Automatización mediante Outlook COM como mecanismo principal.
- Microsoft Power Automate.

---

# 3. Caso multi-cuenta

Un usuario puede tener:

```text
Outlook Desktop

├── personal@hotmail.com
├── usuario@empresa.com
└── usuario@otraempresa.com
```

La aplicación NO debe asumir que la primera cuenta configurada en Outlook es la cuenta que debe utilizar.

Tampoco debe intentar leer las credenciales directamente desde Outlook.

---

# 4. Principio arquitectónico

Outlook Desktop no debe utilizarse como almacén de credenciales.

Arquitectura:

```text
Desktop App
    │
    ▼
Microsoft OAuth
    │
    ▼
Microsoft Account Selector
    │
    ▼
Authorization
    │
    ▼
Access Token
    │
    ▼
Microsoft Graph
    │
    ▼
Exchange Online
```

Outlook Desktop puede coexistir con la integración, pero la autenticación de la aplicación debe realizarse mediante Microsoft identity platform.

---

# 5. UX propuesta

La aplicación debe mostrar:

```text
Selecciona cuenta de Outlook

○ personal@hotmail.com
○ usuario@empresa.com
○ usuario@otraempresa.com

[Conectar]
```

Importante:

La lista de cuentas puede provenir del flujo de autenticación de Microsoft.

La aplicación no debe depender de leer directamente el perfil interno de Outlook.

---

# 6. Relación con Outlook Desktop

Si Outlook Desktop tiene:

```text
Account A
Account B
Account C
```

la aplicación debe permitir utilizar cualquiera de ellas siempre que:

1. La cuenta pueda autenticarse mediante Microsoft identity platform.
2. El tenant/políticas de la organización permitan la aplicación.
3. La cuenta tenga permisos suficientes para enviar correo.

No asumir:

```text
Outlook account = currently selected account
```

La aplicación debe mantener explícitamente:

```text
AuthenticatedAccount
```

---

# 7. Microsoft Graph

El envío debe realizarse mediante Microsoft Graph.

Conceptualmente:

```text
MailService
    │
    ▼
OutlookProvider
    │
    ▼
Microsoft Graph
    │
    ▼
sendMail
```

La lógica de negocio debe permanecer independiente del proveedor.

---

# 8. Modelo de proveedor

Propuesta:

```text
MailProvider
│
├── GmailProvider
│
└── OutlookProvider
```

Interfaz conceptual:

```text
connect()
disconnect()
get_account()
send(message)
```

De esta manera:

```text
UI
 │
 ▼
MailService
 │
 ├── GmailProvider
 │
 └── OutlookProvider
```

---

# 9. Cuenta autenticada

Después del login mostrar:

```text
Outlook

● Connected

usuario@empresa.com

[Enviar prueba]
[Desconectar]
```

La aplicación debe utilizar la identidad devuelta por Microsoft, no inferirla desde el perfil local de Outlook.

---

# 10. Scopes

La POC debe solicitar únicamente los permisos necesarios.

Para envío de correo, evaluar inicialmente:

```text
Mail.Send
```

Evitar solicitar permisos de lectura o administración si no forman parte de la POC.

Principio:

> Least privilege.

---

# 11. Tokens

Utilizar MSAL para gestionar:

- access tokens;
- refresh/session handling;
- cache de autenticación.

No implementar manualmente almacenamiento inseguro de tokens si MSAL puede gestionar el ciclo de autenticación.

Nunca almacenar:

- password;
- tokens en texto plano;
- secretos en Git;
- tokens en logs.

---

# 12. Cuenta personal vs cuenta corporativa

La POC debe probar:

### Cuenta personal

```text
@hotmail.com
@outlook.com
```

### Cuenta corporativa

```text
usuario@empresa.com
```

La segunda puede estar sujeta a políticas del tenant corporativo.

Posibles restricciones:

- consentimiento de administrador;
- aplicaciones no autorizadas;
- Conditional Access;
- políticas de seguridad;
- MFA.

Estas condiciones deben documentarse como dependencias externas de la POC.

---

# 13. Envío

La aplicación debe soportar:

```text
To
Subject
Body
Attachments
```

Ejemplo:

```text
OutlookProvider.send(
    recipient,
    subject,
    body,
    attachments
)
```

El proveedor transforma el mensaje al formato requerido por Microsoft Graph.

---

# 14. Multi-account testing

## T01 — Una cuenta

Precondición:

```text
Outlook
└── user@outlook.com
```

Esperado:

```text
Login
→ autorización
→ user@outlook.com conectado
```

---

## T02 — Dos cuentas personales

Precondición:

```text
Outlook
├── accountA@hotmail.com
└── accountB@hotmail.com
```

Esperado:

```text
Microsoft Login
→ selección accountA
→ aplicación identifica accountA
```

Repetir:

```text
Microsoft Login
→ selección accountB
→ aplicación identifica accountB
```

---

## T03 — Cuenta personal + corporativa

Precondición:

```text
Outlook
├── personal@hotmail.com
└── employee@company.com
```

Validar que ambas cuentas puedan autenticarse cuando las políticas de Microsoft lo permitan.

---

## T04 — Cuenta incorrecta

Autenticar una cuenta distinta de la deseada.

Esperado:

```text
Aplicación muestra:

Cuenta conectada:
wrong-account@example.com
```

El usuario debe poder desconectarla y seleccionar otra.

---

## T05 — Reinicio

1. Autenticar cuenta.
2. Cerrar aplicación.
3. Abrir aplicación.

Esperado:

```text
Cuenta permanece autenticada
```

sin login innecesario mientras el token cache siga siendo válido.

---

## T06 — Envío

Enviar correo de prueba.

Esperado:

```text
Correo enviado mediante Microsoft Graph.
```

---

## T07 — Adjunto

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

## T08 — Desconexión

Desconectar cuenta.

Esperado:

```text
Estado:
○ Not connected
```

La aplicación debe requerir autenticación nuevamente antes de enviar.

---

# 15. Casos de error

### Login cancelado

```text
El usuario canceló la autenticación.
```

### MFA

```text
Microsoft solicita MFA.
```

La aplicación debe permitir que el usuario complete el flujo normalmente.

### Consentimiento

```text
La organización requiere consentimiento administrativo.
```

Mostrar un error entendible.

### Token expirado

MSAL debe intentar obtener silenciosamente un token nuevo.

Si no es posible:

```text
La sesión requiere autenticación nuevamente.
```

### Permiso Mail.Send denegado

```text
La cuenta no tiene autorización para enviar correo
mediante esta aplicación.
```

---

# 16. Outlook Desktop vs Microsoft Graph

No considerar Outlook Desktop como dependencia técnica obligatoria.

La solución debe funcionar conceptualmente así:

```text
Aplicación
    │
    └── Microsoft Graph
             │
             ▼
        Exchange Online
```

aunque el usuario también tenga Outlook Desktop instalado.

Esto permite que la aplicación funcione incluso cuando:

- Outlook no está abierto;
- Outlook está minimizado;
- el usuario utiliza otro cliente;
- existen varias cuentas configuradas.

---

# 17. Seguridad

## Prohibido

- Extraer credenciales de Outlook.
- Extraer tokens.
- Leer archivos OST/PST para obtener credenciales.
- Automatizar la interfaz de Outlook como mecanismo principal.
- Utilizar Selenium para Microsoft login.
- Almacenar passwords.
- Copiar cookies.

## Permitido

- Microsoft identity platform.
- OAuth 2.0.
- MSAL.
- Microsoft Graph.
- Browser-based authentication.
- Token cache seguro.

---

# 18. Criterios de aceptación

La POC se considera exitosa si:

- [ ] Puede autenticar una cuenta Microsoft.
- [ ] Puede trabajar con cuentas personales.
- [ ] Puede trabajar con cuentas corporativas cuando la política del tenant lo permita.
- [ ] Puede manejar múltiples cuentas.
- [ ] Puede identificar inequívocamente la cuenta conectada.
- [ ] Puede enviar correo.
- [ ] Puede enviar adjuntos.
- [ ] Puede persistir la sesión mediante MSAL.
- [ ] Puede desconectar una cuenta.
- [ ] No depende de Outlook Desktop.
- [ ] No extrae credenciales de Outlook.
- [ ] No utiliza Selenium.
- [ ] No almacena contraseñas.
