# BoletaMailer — Cómo correr la app en modo developer

Guía práctica para levantar la aplicación localmente durante desarrollo (§1-6) y para
generar el ejecutable de distribución (§7).

---

## 1. Requisitos previos

| Herramienta | Versión usada en este proyecto | Verificar con |
|---|---|---|
| Go | 1.26.7 | `go version` |
| Node.js | 24.x | `node -v` |
| npm | 11.x | `npm -v` |
| Wails CLI | v2.15.0 | `wails version` |
| WebView2 Runtime | Preinstalado en Windows 11 | `wails doctor` lo detecta |

Windows es el único target soportado (Outlook Classic vía COM es Windows-only).

### Instalar el CLI de Wails (una sola vez por máquina)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

El binario queda en `$(go env GOPATH)/bin` — asegúrate que esa ruta esté en tu `PATH`.
En este entorno: `C:\Users\User\go\bin`.

### Verificar el entorno completo

```bash
wails doctor
```

Debe terminar con `SUCCESS: Your system is ready for Wails development!`. Si falta
WebView2, Node o npm, `wails doctor` indica qué instalar.

---

## 2. Instalar dependencias del frontend

Solo la primera vez, o cuando cambie `frontend/package.json`:

```bash
cd frontend
npm install
cd ..
```

`wails dev` también ejecuta esto automáticamente si detecta que faltan paquetes, pero
instalarlo a mano primero hace el primer arranque más rápido y más fácil de diagnosticar
si algo falla.

---

## 3. Levantar la app en modo developer

Desde la raíz del repo:

```bash
wails dev
```

Esto:

1. Corre `go mod tidy`.
2. Genera los bindings TypeScript en `frontend/wailsjs/` a partir de los métodos
   exportados en `app.go` (`App` struct).
3. Levanta Vite (`npm run dev`) sirviendo el frontend con hot-reload en
   `http://localhost:5173`.
4. Compila el binario Go y abre la ventana nativa de la aplicación (WebView2), que
   consume el frontend a través de `http://localhost:34115`.
5. Observa el árbol de archivos del proyecto — un cambio en `.go` recompila y
   relanza la ventana; un cambio en `frontend/src` hace hot-reload sin reiniciar Go.

La ventana se cierra manualmente (botón cerrar); el proceso `wails dev` termina solo
cuando cierras la ventana o con `Ctrl+C` en la terminal.

### 3.1. Ver la UI en el navegador en vez de la ventana nativa

Útil para usar DevTools de Chrome/Edge sin abrir la ventana embebida:

```text
http://localhost:34115
```

Los bindings de Go (`window.go.main.App.*`) funcionan igual desde el navegador siempre
que `wails dev` siga corriendo — no es una copia estática, habla con el mismo backend.

---

## 4. Dev vs Producción — por qué hay localhost solo en desarrollo

`wails dev` levanta dos puertos locales:

- `:5173` — servidor de Vite, sirve el frontend con hot-reload.
- `:34115` — proxy de Wails que la ventana WebView2 consume, y que también puedes
  abrir en el navegador (ver §3.1).

Esto es **exclusivamente una herramienta de desarrollo**, equivalente a `npm run dev`
en cualquier proyecto web. No existe en el `.exe` final.

En el build de producción (`wails build`), el flujo es distinto:

```text
React (compilado a HTML/JS/CSS estático por `npm run build`)
       |
       embebido en el binario vía //go:embed all:frontend/dist  (main.go)
       |
       v
WebView2 carga esos assets directamente desde el binario, sin red
       |
Go <-> React se comunican por el IPC nativo de WebView2
(bindings `window.go.main.App.*`), NO por HTTP a localhost
```

Esto es consistente con el contexto del MVP (§5: "No se utilizará una arquitectura
HTTP local entre frontend y backend") y con la restricción de distribución (§4:
`.exe` autocontenido, sin runtime ni servidor local).

Por qué importa para este proyecto en concreto:

- El usuario final es una MYPE — es razonable esperar antivirus o firewall corporativo
  restrictivo. Si el `.exe` dependiera de un servidor local en un puerto fijo, ese
  puerto podría estar bloqueado o en uso por otro proceso. Al no depender de red local
  en absoluto, ese riesgo desaparece.
- Refuerza que el frontend nunca puede asumir que hay un servidor HTTP detrás — solo
  los bindings de Wails. Cualquier lógica que hoy pruebes contra `localhost:34115` en
  el navegador debe seguir funcionando igual una vez empaquetada, porque el mecanismo
  de transporte cambia pero el contrato de los bindings no.

---

## 5. Errores comunes

| Síntoma | Causa probable | Solución |
|---|---|---|
| `cannot initialise project in non-empty directory` | Solo aplica a `wails init`, no a `wails dev`. No debería aparecer en desarrollo normal. | N/A |
| Ventana en blanco | Frontend no compiló o Vite no llegó a levantar a tiempo | Revisar la terminal de `wails dev`; buscar errores de `npm run dev` o de TypeScript |
| `wails: command not found` | El CLI no está en `PATH` | Agregar `$(go env GOPATH)/bin` (o `%GOPATH%\bin` en Windows) al `PATH` |
| Proxy error `wsarecv` al cerrar la ventana | Vite pierde la conexión del WebSocket de HMR cuando la ventana se cierra | Normal, no indica un bug — aparece justo antes de `Development mode exited` |
| Cambios en `app.go` no aparecen en el frontend | Los bindings TS no se regeneraron | Reiniciar `wails dev`; si persiste, borrar `frontend/wailsjs/` y relanzar |

---

## 6. Notas para este proyecto específico

- `frontend/wailsjs/` es generado, no se versiona (ver `.gitignore`) — nunca editarlo a mano.
- El spec de la UI objetivo (tabs Renombrar/Enviar) está en
  [docs/specs/ui_spec_mvp.md](specs/ui_spec_mvp.md).
- La integración real con Outlook COM ya cubre la prueba de conexión
  (`app/infrastructure/outlook`, botón "Probar conexión con Outlook"). El resto —
  lectura de Excel, envío real de correos, renombrado — sigue mockeado en
  `frontend/src/services/mockApi.ts` mientras avanzan las Fases 1-2 del contexto del MVP.

---

## 7. Generar el ejecutable de distribución (`wails build`)

A diferencia de `wails dev` (§3), esto produce el `.exe` final: frontend compilado y
embebido en el binario, sin servidor local ni dependencia de Node/npm en la máquina
del usuario final (ver §4 para el porqué).

### 7.1. Build estándar

Desde la raíz del repo:

```bash
wails build
```

Esto:

1. Corre `go mod tidy` y regenera los bindings TypeScript.
2. Ejecuta `npm run build` en `frontend/` (compila React a `frontend/dist/`, estático).
3. Embebe `frontend/dist/` en el binario vía `//go:embed all:frontend/dist` (`main.go`).
4. Compila el binario Go en modo `production` (sin devtools, sin consola de debug).
5. Genera:

   ```text
   build\bin\BoletaMailer.exe
   ```

   Nombre definido por `outputfilename` en `wails.json` — coincide con el artefacto
   esperado por el contexto del MVP (§4). Tamaño de referencia en este proyecto:
   ~11 MB.

`build/bin/` no se versiona (ver `.gitignore`) — se regenera en cada build.

### 7.2. Verificar el build

El `.exe` generado corre de forma completamente independiente — no requiere que
`wails dev`, Vite, ni ningún proceso Node siga activo. Para confirmarlo:

```bash
# Sin wails dev corriendo:
./build/bin/BoletaMailer.exe
```

Debe abrir la ventana normalmente. Si la ventana no carga contenido (queda en blanco)
mientras que `wails dev` sí funciona, el problema está en el paso de embed o en el
build de producción de Vite — no en el código Go/React en sí.

La verificación completa en un equipo limpio (sin Go/Node/Python instalados) es la
que exige el contexto del MVP §26 — pendiente de ejecutar cuando haya una máquina
Windows separada disponible para probar. La verificación de este documento (build +
arranque standalone en la máquina de desarrollo) es un primer nivel, no la sustituye.

### 7.3. Variantes útiles del build

| Comando | Cuándo usarlo |
|---|---|
| `wails build -clean` | Limpia `build/bin` antes de compilar — útil si sospechas de un binario obsoleto. |
| `wails build -upx` | Comprime el `.exe` con UPX (`wails doctor` lo lista como dependencia opcional; hay que instalarlo aparte). Reduce tamaño, aumenta tiempo de build. No probado aún en este proyecto. |
| `wails build -nsis` | Genera además un instalador `.exe` (NSIS) en vez de solo el binario suelto. El contexto del MVP (§4) pide un `.exe` autocontenido simple — un instalador es una capacidad adicional, no un requisito, y no se ha adoptado todavía. |
| `wails build -debug` | Build con devtools/consola habilitados — solo para diagnosticar un problema que no reproduce en `wails dev`, nunca para distribuir. |

### 7.4. Errores comunes al buildear

| Síntoma | Causa probable | Solución |
|---|---|---|
| El build falla con `EPERM`/`Permission denied` sobre `frontend/dist/...` | Una instancia anterior de `BoletaMailer.exe` o de `wails dev` sigue viva y tiene archivos bloqueados | Cerrar toda ventana de la app y todo proceso `wails`/`BoletaMailer*` antes de reintentar (`Get-Process | Where-Object ProcessName -match "wails|BoletaMailer"` en PowerShell) |
| El `.exe` funciona con `wails dev` pero el build standalone abre en blanco | El paso `npm run build` fue exitoso pero generó un `frontend/dist/index.html` con rutas rotas, o el embed no incluyó todo | Revisar la salida de "Compiling frontend" en el log del build; probar `cd frontend && npm run build` de forma aislada y servir `dist/` con cualquier servidor estático para descartar el problema antes de involucrar a Wails |
| El título de la ventana muestra `boleta-mailer` en vez de `BoletaMailer` | `wails.json` tiene `name` en minúsculas — solo `outputfilename` controla el nombre del `.exe` | Ajustar `wails.json` si se quiere un título de ventana distinto; no afecta el nombre del artefacto |

### 7.5. "Windows protegió su PC" (SmartScreen) al ejecutar el `.exe` en otra máquina

Al copiar `BoletaMailer.exe` a la máquina de un usuario y ejecutarlo, Windows puede
mostrar:

```text
Windows protegió su PC
SmartScreen de Microsoft Defender evitó que se iniciara una aplicación desconocida.
```

**Esto no es un antivirus deteniendo malware — es SmartScreen evaluando reputación.**
Es importante no confundirlo con el problema que causaba bloqueos en builds
PyInstaller `--onefile` (que se resolvían cambiando a `--onedir`): ese problema era
heurística de comportamiento disparada por la **autoextracción a una carpeta
temporal** que hace un ejecutable OneFile de PyInstaller al arrancar. Un binario Go
(y por lo tanto el `.exe` de `wails build`) no se autoextrae ni descomprime nada en
tiempo de ejecución — ya es código nativo compilado corriendo directamente. El
equivalente Go de "modo OneDir" es, en la práctica, lo que ya se tiene por defecto.
No existe un ajuste de build en Go/Wails que reproduzca el fix de PyInstaller, porque
la causa que lo motivó no aplica aquí.

SmartScreen bloquea binarios nuevos basándose en dos factores, ninguno relacionado con
cómo se empaqueta el ejecutable:

1. **Firma digital** — si el `.exe` no está firmado con un certificado de firma de
   código (Authenticode), SmartScreen lo trata como "no verificado".
2. **Reputación acumulada** — cuántas veces Microsoft ha visto ese hash exacto de
   archivo ejecutarse en su red de telemetría (SmartScreen Application Reputation).
   Un `.exe` recién compilado, sin firmar, tiene reputación cero sin importar qué tan
   benigno sea el código — esto le pasaría a cualquier ejecutable Windows nuevo, no
   solo a los hechos con Go/Wails.

#### Solución inmediata (costo cero): desbloqueo manual

SmartScreen siempre deja una vía de escape manual. En el diálogo de bloqueo:

```text
1. Clic en "Más información" (el enlace queda cortado/oculto en algunos temas de Windows)
2. Aparece el botón "Ejecutar de todas formas"
3. Clic ahí — la app corre normalmente
```

Esto se pide una sola vez por máquina y por versión del `.exe` (un rebuild cambia el
hash y SmartScreen vuelve a evaluarlo desde cero). Es la opción usada actualmente para
este proyecto — sin costo, sin cambios de build, adecuada para distribución interna a
un número pequeño de usuarios conocidos.

#### Alternativa: excepción en Windows Defender

Si SmartScreen sigue interfiriendo (por ejemplo, bloqueando antes de mostrar el
diálogo con la opción de desbloqueo), se puede excluir la carpeta del `.exe` en
Windows Security → Protección antivirus y contra amenazas → Exclusiones. Requiere
permisos de administrador en la máquina del usuario — decisión del usuario/soporte de
TI, no algo que la app pueda automatizar.

#### Solución permanente (con costo): certificado de firma de código

La única forma de que la advertencia deje de aparecer para todos los usuarios sin
intervención manual es firmar el `.exe` con un certificado de firma de código
(Authenticode) antes de distribuirlo, vía `signtool sign`.

| Tipo | Costo aproximado | Reputación SmartScreen |
|---|---|---|
| OV (Organization Validated) | ~$70–250 USD/año | No inmediata — se acumula con el tiempo y volumen de ejecuciones reportadas a Microsoft (semanas/meses) |
| EV (Extended Validation) | ~$300–450 USD/año | Inmediata desde el primer build firmado, pero exige que la clave privada viva en hardware (token USB o HSM en la nube) — no es un archivo `.pfx` que se pueda guardar en el repo |

Dado que BoletaMailer es una herramienta interna para un número reducido de usuarios
de una MYPE (no un producto distribuido masivamente), el costo/complejidad de un
certificado — sobre todo EV, con su requisito de HSM — no se justifica hoy. Queda
como mejora pendiente si el proyecto llegara a distribuirse a una base de usuarios
más amplia o externa a la empresa.
