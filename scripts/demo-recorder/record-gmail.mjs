// Graba un envío REAL con Gmail: la app sale por SMTP con tu contraseña de
// aplicación y los 10 correos llegan a tu propia bandeja (alias tucorreo+nombre@).
// Se graba la pantalla completa, no solo el navegador:
//
//   ┌──────────────────────────────┬───────────────┐
//   │ Despacho (frontend real)     │ Explorador:   │
//   │ backend Go real              │ scan_0001.pdf │
//   │                              │ → CERT-1001_… │
//   └──────────────────────────────┴───────────────┘
//   ...y al final, tu Gmail con los correos recibidos.
//
// Por qué no se controla Despacho.exe directamente: Wails (go-webview2) borra a
// propósito WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS, así que el WebView2 del .exe
// no expone el puerto de depuración que Playwright necesita. En su lugar se usa
// el binario de desarrollo de Wails (mismo backend Go, mismos bindings) y el
// frontend se muestra en una ventana de Chrome en modo app (sin barra de
// direcciones). El envío, el renombrado y el diálogo de confirmación son reales.
//
// Una sola vez:   node record-gmail.mjs --login     (inicias sesión en Gmail)
// Luego:          node record-gmail.mjs [--skip-build]
// Ensayo:         node record-gmail.mjs --dry       (modo demo, sin Gmail)
//
// Credenciales: scripts/demo-recorder/.env.local (ignorado por git)
//   GMAIL_USER=tucorreo@gmail.com
//   GMAIL_APP_PASSWORD=xxxx xxxx xxxx xxxx
//   GMAIL_FROM_NAME=Despacho Demo           (opcional)

import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(HERE, "..", "..");
const args = new Set(process.argv.slice(2));
const DRY = args.has("--dry");
const OUT = join(ROOT, "docs", "demo", DRY ? "gmail-ensayo" : "gmail");
const SHOTS = join(OUT, "screenshots");
const RAW = join(OUT, ".raw");
const DATA = join(ROOT, "demo");
const SCANS = join(DATA, "documentos_origen");
const PROFILE = join(HERE, ".chrome-profile");
const DEV_BIN = join(ROOT, "build", "bin", "Despacho-dev.exe");
const CHROME = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe";
const URL = "http://localhost:34115";

// Distribución en píxeles FÍSICOS de una pantalla 1920×1080. Chrome corre con
// --force-device-scale-factor=1 para que la app tenga 1300 px reales aunque
// Windows esté al 125%; el Explorador (que sí escala) recibe coordenadas
// lógicas, por eso se divide entre SCALE. El alto deja fuera la barra de
// tareas, que luego se recorta del video.
const SCALE = windowsScale();
const SCREEN = { w: 1920, h: 1080, work: 1080 - Math.round(48 * SCALE) };
const APP = { x: 0, y: 0, w: 1300, h: SCREEN.work };
const EXPLORER = { x: APP.w, y: 0, w: SCREEN.w - APP.w, h: SCREEN.work };
const logical = (r) => Object.fromEntries(Object.entries(r).map(([k, v]) => [k, Math.round(v / SCALE)]));

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// --- --login: Chrome normal (sin Playwright) para que Google acepte el inicio de sesión
if (args.has("--login")) {
  console.log("Inicia sesión en Gmail en la ventana que se abrirá y luego ciérrala.");
  spawnSync(CHROME, [`--user-data-dir=${PROFILE}`, "--no-first-run", "https://mail.google.com/"], { stdio: "inherit" });
  console.log("Sesión guardada en", PROFILE);
  process.exit(0);
}

const env = loadEnv();
if (!DRY && (!env.GMAIL_USER || !env.GMAIL_APP_PASSWORD)) {
  console.error(`Faltan credenciales. Crea ${join(HERE, ".env.local")} con GMAIL_USER y GMAIL_APP_PASSWORD (ver .env.local.example).`);
  process.exit(1);
}
if (!DRY && !existsSync(PROFILE)) {
  console.error("Primero ejecuta: node record-gmail.mjs --login");
  process.exit(1);
}

// --- 1. datos (alias de tu bandeja) y build ---------------------------------
rmSync(DATA, { recursive: true, force: true });
run("go", ["run", "./cmd/demodata", "-out", DATA, "-count", "12", ...(DRY ? [] : ["-inbox", env.GMAIL_USER])]);
if (!args.has("--skip-build") || !existsSync(DEV_BIN)) {
  run("npm", ["run", "build"], join(ROOT, "frontend"));
  run("go", ["build", "-tags", "dev", "-o", DEV_BIN, "."]);
}

// --- 2. backend Go + ventanas -----------------------------------------------
const app = spawn(DEV_BIN, ["-assetdir", join(ROOT, "frontend", "dist"), "-devserver", "localhost:34115", "-loglevel", "Warning"], {
  cwd: ROOT,
  stdio: "ignore",
});
let ffmpeg = null;
let ctx = null;
let page = null;
const cleanup = () => {
  if (ffmpeg && ffmpeg.exitCode === null) ffmpeg.kill();
  if (!app.killed) app.kill();
};
process.on("exit", cleanup);

try {
  await waitForServer(URL);
  // La ventana nativa del binario dev no se usa: se oculta. Sigue siendo la
  // dueña del diálogo de confirmación, que aparece centrado en pantalla.
  await winTool("hide", { ProcessId: app.pid });
  const ex = logical(EXPLORER);
  await winTool("explorer", { Path: SCANS, X: ex.x, Y: ex.y, W: ex.w, H: ex.h });
  await record();
} finally {
  if (ctx) await ctx.close().catch(() => {});
  await winTool("close-explorer", { Path: SCANS }).catch(() => {});
  cleanup();
}

// --- 3. guion ---------------------------------------------------------------
async function record() {
  rmSync(OUT, { recursive: true, force: true });
  mkdirSync(SHOTS, { recursive: true });
  mkdirSync(RAW, { recursive: true });
  await openApp();

  // Graba el escritorio sin el cursor real (-draw_mouse 0): el cursor
  // amarillo de la página es el que guía al espectador.
  ffmpeg = spawn(findFfmpeg(), ["-y", "-loglevel", "error", "-f", "gdigrab", "-framerate", "30", "-draw_mouse", "0",
    "-i", "desktop", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "16", join(RAW, "screen.mkv")],
    { stdio: ["pipe", "inherit", "inherit"] });
  await sleep(1500);

  // 01 · Conexión
  if (DRY) {
    await click(page.locator("button.prov", { hasText: "Modo demo" }));
    await click(btn("Elegir carpeta"));
    await click(btn("Usar modo demo"));
  } else {
    await click(page.locator("button.prov", { hasText: "Gmail / SMTP" }));
    await typeInto(page.locator("#smtp-user"), env.GMAIL_USER, 35);
    await click(page.locator("#smtp-pass"));
    await page.locator("#smtp-pass").fill(env.GMAIL_APP_PASSWORD); // se ve como ••••
    await sleep(400);
    await typeInto(page.locator("#smtp-name"), env.GMAIL_FROM_NAME || "Despacho Demo", 45);
    await click(btn("Probar conexión"));
    await btn("Probar conexión").and(page.locator(":enabled")).waitFor({ timeout: 30000 });
    await sleep(1500);
    await shot("01-conexion-gmail.png");
    await click(btn("Usar esta cuenta"));
  }
  await sleep(1000);
  await click(btn("Siguiente: cargar datos"));

  // 02 · Datos
  await click(btn("Seleccionar Excel o CSV"));
  await page.getByText("plantilla_envio.xlsx").waitFor();
  await page.getByText("con errores").waitFor();
  await sleep(2500);
  await shot("02-datos-validados.png");
  await click(btn("Siguiente: plantilla"));

  // 03 · Plantilla HTML: la vista previa la arma el mismo Go que envía
  await page.locator("iframe.mail-frame").waitFor();
  await sleep(2500);
  await shot("03-plantilla-html.png");
  for (let i = 0; i < 2; i++) {
    await click(page.getByRole("button", { name: "Destinatario siguiente" }));
    await sleep(1500);
  }
  await click(btn("Siguiente: adjuntos"));

  // 04 · Renombrado (se ve en vivo en el Explorador) y emparejado
  await click(btn("Elegir carpeta"));
  await sleep(1500); // el Explorador muestra scan_0001.pdf…
  await typeInto(page.locator("#rule"), "CERT-{{documento_id}}_{{nombre}}{{ext}}", 55);
  await btn("Renombrar 11 archivos").waitFor();
  await sleep(2000);
  await shot("04-renombrar-vista-previa.png");
  await click(btn("Renombrar 11 archivos"));
  await page.getByText("11 renombrados").waitFor();
  await sleep(3500); // …y ahora CERT-1001_Ana Torres.pdf
  await typeInto(page.locator("#pattern"), "CERT-{{documento_id}}_*.pdf", 55);
  await btn("Revisar y enviar 10 correos").waitFor();
  await sleep(1500);
  await shot("04-adjuntos-emparejados.png");
  await click(btn("Revisar y enviar 10 correos"));

  // 05 · Envío
  await sleep(1500);
  await shot("05-envio-listo.png");
  // El diálogo nativo de Go se muestra unos segundos y se acepta. En modo
  // demo no hay diálogo (no se envía nada que confirmar).
  const confirm = DRY ? Promise.resolve() : winTool("confirm", { Title: "Confirmar envío", HoldMs: 2200 });
  await click(btn("Enviar 10 correos"));
  await confirm;
  await page.locator(".kpi b", { hasText: /^[3-7] \/ 10$/ }).waitFor({ timeout: 90000 });
  await shot("05-envio-en-curso.png");
  await page.getByRole("heading", { name: "Lote terminado" }).waitFor({ timeout: 180000 });
  await sleep(2000);
  await shot("05-envio-terminado.png");
  await sleep(1000);

  // 06 · La prueba: los correos en Gmail
  if (!DRY) await showGmail();

  ffmpeg.stdin.write("q");
  await new Promise((r) => ffmpeg.on("exit", r));
  const mp4 = join(OUT, "despacho-gmail.mp4");
  // Recorta la barra de tareas y re-encoda con calidad para redes.
  run(findFfmpeg(), ["-y", "-loglevel", "error", "-i", join(RAW, "screen.mkv"),
    "-vf", `crop=${SCREEN.w}:${SCREEN.work - (SCREEN.work % 2)}:0:0`,
    "-c:v", "libx264", "-preset", "slow", "-crf", "20", "-pix_fmt", "yuv420p", "-movflags", "+faststart", mp4]);
  rmSync(RAW, { recursive: true, force: true });
  console.log(`\nListo:\n  ${mp4}\n  ${SHOTS} (${readdirSync(SHOTS).length} imágenes)`);
}

async function openApp() {
  ctx = await chromium.launchPersistentContext(PROFILE, {
    channel: "chrome",
    headless: false,
    viewport: null,
    locale: "es-ES",
    // Sin estos, Chrome muestra barras de aviso que saldrían en el video.
    ignoreDefaultArgs: ["--enable-automation", "--no-sandbox"],
    args: [
      `--app=${URL}`,
      `--window-position=${APP.x},${APP.y}`,
      `--window-size=${APP.w},${APP.h}`,
      "--force-device-scale-factor=1",
      "--disable-blink-features=AutomationControlled",
      "--no-first-run",
    ],
  });
  await ctx.addInitScript(CURSOR);
  page = ctx.pages().find((p) => p.url().startsWith(URL)) ?? (await ctx.waitForEvent("page"));
  for (const p of ctx.pages()) if (p !== page) await p.close();
  page.setDefaultTimeout(30000);
  await page.reload(); // para que corra el script del cursor
  await page.waitForFunction(() => window.go?.desktop?.App && window.go?.desktop?.CampaignService);
  await page.evaluate(
    ({ data, docs, eml }) => {
      // Lo único simulado: los selectores nativos de archivo/carpeta.
      window.go.desktop.App.SelectDataFile = async () => data;
      window.go.desktop.App.SelectFolder = async (title) => (String(title).includes(".eml") ? eml : docs);
    },
    { data: join(DATA, "plantilla_envio.xlsx"), docs: SCANS, eml: join(DATA, "salida_eml") }
  );
  await page.bringToFront();
  await sleep(800);
}

async function showGmail() {
  const gm = await ctx.newPage();
  const cdp = await ctx.newCDPSession(gm);
  const { windowId } = await cdp.send("Browser.getWindowForTarget");
  await cdp.send("Browser.setWindowBounds", { windowId, bounds: { windowState: "normal", left: 0, top: 0, width: SCREEN.w, height: SCREEN.work } });
  await gm.bringToFront();
  // Solo los correos de la demo: el resto de tu bandeja no aparece en el video.
  await gm.goto("https://mail.google.com/mail/u/0/#search/" + encodeURIComponent('subject:"aquí está tu certificado" newer_than:1d'));
  const rows = gm.locator("tr.zA");
  await rows.first().waitFor({ timeout: 60000 });
  await sleep(3000);
  await shot("06-gmail-bandeja.png", gm);
  await click(rows.first(), gm);
  await sleep(3000);
  await shot("06-gmail-correo.png", gm);
  try {
    // Abre el PDF adjunto en el visor de Gmail (selectores de Gmail: frágiles).
    const att = gm.locator("span.aV3, div.aQH span[role='link'], div.aQH a").first();
    await att.waitFor({ timeout: 5000 });
    await click(att, gm);
    await sleep(3500);
    await shot("06-gmail-adjunto.png", gm);
  } catch {
    console.warn("  (no se pudo abrir el adjunto en Gmail; se omite esa toma)");
  }
  await sleep(1500);
}

// --- utilidades -------------------------------------------------------------
function btn(name) {
  return page.getByRole("button", { name, exact: false });
}

async function shot(name, p = page) {
  await sleep(300);
  await p.screenshot({ path: join(SHOTS, name), style: "#demo-cursor{display:none!important}" });
  console.log(`  screenshot ${name}`);
}

// Movimiento de mouse visible antes de cada clic.
async function click(locator, p = page) {
  await locator.waitFor({ state: "visible" });
  await locator.scrollIntoViewIfNeeded();
  const box = await locator.boundingBox();
  await p.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 20 });
  await sleep(200);
  await p.mouse.down();
  await sleep(80);
  await p.mouse.up();
  await sleep(500);
}

async function typeInto(locator, text, delay = 45) {
  await click(locator);
  await locator.press("Control+A");
  await locator.press("Backspace");
  await locator.pressSequentially(text, { delay });
  await sleep(400);
}

function CURSOR() {
  addEventListener("DOMContentLoaded", () => {
    const c = document.createElement("div");
    c.id = "demo-cursor";
    c.style.cssText =
      "position:fixed;left:0;top:0;width:22px;height:22px;margin:-11px 0 0 -11px;border-radius:50%;" +
      "background:rgba(242,194,48,.55);border:2px solid #1B1A17;z-index:2147483647;pointer-events:none;" +
      "transition:transform .12s ease;transform:translate(-100px,-100px)";
    document.body.appendChild(c);
    let x = -100, y = -100;
    addEventListener("mousemove", (e) => { x = e.clientX; y = e.clientY; c.style.transform = `translate(${x}px,${y}px)`; }, true);
    addEventListener("mousedown", () => (c.style.transform = `translate(${x}px,${y}px) scale(.7)`), true);
    addEventListener("mouseup", () => (c.style.transform = `translate(${x}px,${y}px)`), true);
  });
}

function run(cmd, cmdArgs, cwd = ROOT) {
  console.log(`$ ${cmd} ${cmdArgs.join(" ")}`);
  // Solo npm necesita shell en Windows (es un .cmd); nunca se le pasan rutas.
  const shell = process.platform === "win32" && cmd === "npm";
  const res = spawnSync(cmd, cmdArgs, { cwd, stdio: "inherit", shell });
  if (res.status !== 0) throw new Error(`${cmd} falló (código ${res.status})`);
}

function winTool(action, params = {}) {
  const ps = ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", join(HERE, "wintools.ps1"), action];
  for (const [k, v] of Object.entries(params)) ps.push(`-${k}`, String(v));
  return new Promise((ok, fail) =>
    spawn("powershell.exe", ps, { stdio: ["ignore", "inherit", "inherit"] })
      .on("exit", (c) => (c === 0 ? ok() : fail(new Error(`wintools ${action} falló (${c})`)))));
}

async function waitForServer(url, timeoutMs = 30000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      if ((await fetch(url)).ok) return;
    } catch {
      /* todavía no arranca */
    }
    await sleep(300);
  }
  throw new Error(`La app no respondió en ${url}`);
}

// Lee .env.local sin imprimir nada: la contraseña nunca sale a la consola.
function loadEnv() {
  const file = join(HERE, ".env.local");
  if (!existsSync(file)) return {};
  const env = {};
  for (const line of readFileSync(file, "utf8").split(/\r?\n/)) {
    if (line.trimStart().startsWith("#")) continue;
    const m = line.match(/^\s*([A-Z_]+)\s*=\s*(.*?)\s*$/);
    if (m) env[m[1]] = m[2].replace(/^["']|["']$/g, "");
  }
  return env;
}

function windowsScale() {
  const q = spawnSync("reg", ["query", "HKCU\\Control Panel\\Desktop\\WindowMetrics", "/v", "AppliedDPI"], { encoding: "utf8" });
  const m = q.stdout?.match(/0x([0-9a-f]+)/i);
  return m ? parseInt(m[1], 16) / 96 : 1;
}

function findFfmpeg() {
  if (process.env.FFMPEG_PATH && existsSync(process.env.FFMPEG_PATH)) return process.env.FFMPEG_PATH;
  const where = spawnSync("where", ["ffmpeg"], { encoding: "utf8" });
  if (where.status === 0) return where.stdout.split(/\r?\n/)[0].trim();
  // winget lo deja aquí; una terminal abierta antes de instalarlo no lo ve en el PATH.
  const winget = join(process.env.LOCALAPPDATA ?? "", "Microsoft", "WinGet", "Links", "ffmpeg.exe");
  if (existsSync(winget)) return winget;
  throw new Error("No se encontró ffmpeg (winget install Gyan.FFmpeg)");
}
