"use strict";

// Cadence developer dashboard helper: a minimal Electron main process that
// automates the Spotify Developer dashboard in a hidden window and reports
// only the resulting public Client ID or Soloist API key over the loopback
// control socket.
//
// The dashboard Bearer token is captured from the page's own requests and kept
// in memory. It is attached to our tagged requests and never logged. The
// Soloist API key and the Client Secret are never logged either.

const { app, BrowserWindow } = require("electron");

const DASHBOARD_URL = "https://developer.spotify.com/dashboard";
const CREATE_URL = "https://developer.spotify.com/dashboard/create";
const SOLOIST_URL = "https://developer.spotify.com/dashboard/soloist";
const REDIRECT_URI = "http://127.0.0.1:8888/callback";
const ACTION_TIMEOUT_MS = 10 * 60 * 1000;
const LOGIN_TIMEOUT_MS = 10 * 60 * 1000;
const CONTROL_TIMEOUT_MS = 10000;
const SEARCH_TIMEOUT_MS = 60000;
const SUBMIT_TIMEOUT_MS = 3 * 60 * 1000;
const LOGIN_WINDOW_TITLE = "Cadence Spotify login";

class LoginRequired extends Error {}

function readSwitch(name) {
  const prefix = "--" + name + "=";
  const argument = process.argv.find((value) => value.startsWith(prefix));
  return argument ? argument.slice(prefix.length) : "";
}

function parseLoopbackURL(raw, protocol) {
  try {
    const target = new URL(raw);
    if (target.protocol !== protocol) {
      return null;
    }
    if (target.hostname !== "127.0.0.1") {
      return null;
    }
    const port = Number(target.port);
    if (!Number.isInteger(port) || port <= 0 || port > 65535) {
      return null;
    }
    if (target.username !== "" || target.password !== "") {
      return null;
    }
    return target;
  } catch {
    return null;
  }
}

function describe(error) {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function isSpotifyAuthorizeURL(raw) {
  try {
    const target = new URL(raw);
    return (
      target.protocol === "https:" &&
      target.hostname === "accounts.spotify.com" &&
      target.pathname === "/authorize" &&
      target.username === "" &&
      target.password === ""
    );
  } catch {
    return false;
  }
}

const controlArgument = readSwitch("cadence-control");
const action = readSwitch("cadence-action");
const rotate = readSwitch("cadence-rotate") === "true";
const oauthURL = readSwitch("cadence-oauth-url");
const controlURL = parseLoopbackURL(controlArgument, "ws:");

if (action === "oauth") {
  // OAuth mode only drives the browser: it never connects to the control
  // socket and never handles a Client ID, key, code or token.
  if (!isSpotifyAuthorizeURL(oauthURL)) {
    process.stderr.write("cadence developer: missing or invalid --cadence-oauth-url\n");
    process.exit(2);
  }
} else if (!controlURL || (action !== "web-app" && action !== "soloist-key")) {
  process.stderr.write("cadence developer: missing --cadence-control or --cadence-action\n");
  process.exit(2);
}

// These must be set before app.whenReady() to affect Chromium startup. The
// dashboard is remote and the window is hidden, so background throttling would
// stall the automation.
app.commandLine.appendSwitch("disable-background-timer-throttling");
app.commandLine.appendSwitch("disable-renderer-backgrounding");
app.commandLine.appendSwitch("disable-backgrounding-occluded-windows");

let socket = null;
let window = null;
let finished = false;
let capturedAuthorization = "";
let authCaptured = false;
let oauthCallbackReached = false;

// Only state names reach the log. Never log the Bearer token, the API key, the
// Client Secret or page contents.
function logState(state) {
  process.stderr.write("cadence developer: state=" + state + "\n");
}

function sendControl(message) {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    return false;
  }
  try {
    socket.send(JSON.stringify(message));
    return true;
  } catch {
    return false;
  }
}

async function finish(message) {
  if (finished) {
    return;
  }
  finished = true;
  logState("finish:" + message.type + (message.code ? ":" + message.code : ""));
  sendControl(message);
  // Give the WebSocket a moment to flush before the process exits.
  await new Promise((resolve) => setTimeout(resolve, 150));
  app.quit();
}

function fail(code, message) {
  return finish({ type: "error", code: code, message: message });
}

function loginRequired() {
  return fail("login_required", "Spotify login timed out. Sign in to Spotify and try again.");
}

function isLoginURL(url) {
  try {
    return new URL(url).hostname === "accounts.spotify.com";
  } catch {
    return false;
  }
}

// Consent pages live under /authorize or /oauth2 on the accounts host. Login
// pages (for example /en/login) must be shown to the user; only consent pages
// are clicked automatically.
function isConsentURL(url) {
  try {
    const target = new URL(url);
    return (
      target.hostname === "accounts.spotify.com" &&
      (target.pathname.endsWith("/authorize") || target.pathname.startsWith("/oauth2/"))
    );
  } catch {
    return false;
  }
}

function isCallbackURL(url) {
  try {
    const target = new URL(url);
    return (
      target.protocol === "http:" &&
      target.hostname === "127.0.0.1" &&
      target.port === "8888" &&
      target.pathname === "/callback"
    );
  } catch {
    return false;
  }
}

// A login page with a password field needs the user. A consent page has no
// password field and is accepted automatically while the window stays hidden.
const LOGIN_STATE = `(() => {
  if (document.querySelector('input[type="password"]')) {
    return { password: true, consent: false };
  }
  const candidates = [...document.querySelectorAll('button, [role="button"], a')]
    .filter((element) => {
      if (element.disabled) return false;
      const text = (element.innerText || "").trim();
      if (!/^(agree|accept|continue|allow|aceptar|continuar|permitir|autorizar|aceitar)$/i.test(text)) return false;
      return element.getClientRects().length > 0;
    });
  if (candidates.length === 0) {
    return { password: false, consent: false };
  }
  candidates[0].click();
  return { password: false, consent: true };
})()`;

function showLoginWindow() {
  if (!window || window.isDestroyed()) return;
  if (!window.isVisible()) {
    window.show();
  }
  window.focus();
}

function hideLoginWindow() {
  if (!window || window.isDestroyed()) return;
  if (!window.isVisible()) return;
  window.hide();
}

// syncLoginWindow shows the window only for a credential prompt and hides it
// again once the page leaves the login host. It returns true while the page is
// still on accounts.spotify.com.
async function syncLoginWindow() {
  if (!window || window.isDestroyed()) return false;
  const url = window.webContents.getURL();
  if (!isLoginURL(url)) {
    hideLoginWindow();
    return false;
  }
  if (isConsentURL(url)) {
    // The consent page is accepted automatically; the window stays hidden
    // unless the page unexpectedly asks for a credential.
    const state = await runScript(LOGIN_STATE);
    if (state && state.password) {
      showLoginWindow();
    } else {
      hideLoginWindow();
    }
  } else {
    // A real login page (password, email-first, 2FA) is the only case where
    // the user needs to see and use the browser. Never automate its fields.
    showLoginWindow();
  }
  return true;
}

// A navigation event and the login poll must not click the same consent
// button twice; every login check runs through this single chain.
let loginSync = Promise.resolve();
function syncLoginWindowQueued() {
  loginSync = loginSync.then(
    () => syncLoginWindow(),
    () => syncLoginWindow(),
  );
  return loginSync;
}

function sleep(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function runScript(script) {
  try {
    return await window.webContents.executeJavaScript(script, true);
  } catch (error) {
    // A navigation can destroy the execution context; the caller retries.
    return null;
  }
}

// ensureSignedIn waits until an interactive login completes. It never fails on
// the first login redirect: the user gets the full login window, and only the
// timeout (or an explicit error path) fails with login_required. It reports
// whether it had to wait, so callers can restart their own deadlines.
async function ensureSignedIn() {
  const deadline = Date.now() + LOGIN_TIMEOUT_MS;
  if (!(await syncLoginWindowQueued())) {
    return false;
  }
  while (!finished) {
    if (Date.now() >= deadline) {
      throw new LoginRequired();
    }
    await sleep(500);
    if (!(await syncLoginWindowQueued())) {
      return true;
    }
  }
  return true;
}

async function waitFor(check, timeout, interval) {
  let deadline = Date.now() + timeout;
  while (true) {
    if (await ensureSignedIn()) {
      deadline = Date.now() + timeout;
    }
    if (Date.now() >= deadline) {
      return null;
    }
    const value = await check();
    if (value) {
      return value;
    }
    await sleep(interval);
  }
}

function connectControl() {
  return new Promise((resolve, reject) => {
    let settled = false;
    const ws = new WebSocket(controlURL.href);
    const timer = setTimeout(() => {
      if (settled) {
        return;
      }
      settled = true;
      reject(new Error("control socket open timeout"));
      try {
        ws.close();
      } catch {
        // The socket may already be closed.
      }
    }, CONTROL_TIMEOUT_MS);

    ws.addEventListener("open", () => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      socket = ws;
      try {
        ws.send(JSON.stringify({ type: "hello", version: "1" }));
      } catch (error) {
        reject(error);
        return;
      }
      resolve(ws);
    });

    ws.addEventListener("message", (event) => {
      if (typeof event.data !== "string") {
        return;
      }
      let message = null;
      try {
        message = JSON.parse(event.data);
      } catch {
        return;
      }
      if (message && message.type === "shutdown") {
        app.quit();
      }
    });

    ws.addEventListener("error", () => {
      clearTimeout(timer);
      if (!settled) {
        settled = true;
        reject(new Error("control socket connection failed"));
        return;
      }
      if (!finished) {
        process.stderr.write("cadence developer: control socket error\n");
        app.quit();
      }
    });

    ws.addEventListener("close", () => {
      clearTimeout(timer);
      socket = null;
      if (settled && !finished) {
        app.quit();
      }
    });
  });
}

function installHeaderHook() {
  window.webContents.session.webRequest.onBeforeSendHeaders(
    { urls: ["https://developer.spotify.com/api/wristband/*"] },
    (details, callback) => {
      const headers = details.requestHeaders;
      const authorization = headers.Authorization || headers.authorization;
      if (authorization) {
        capturedAuthorization = authorization;
        if (!authCaptured) {
          authCaptured = true;
          logState("auth_captured");
        }
      }
      const tagged = headers["X-Cadence-Developer"] || headers["x-cadence-developer"];
      if (tagged === "1" && capturedAuthorization) {
        headers.Authorization = capturedAuthorization;
      }
      callback({ requestHeaders: headers });
    },
  );
}

async function loadPage(target) {
  try {
    await window.loadURL(target);
  } catch (error) {
    if (!window || !isLoginURL(window.webContents.getURL())) {
      throw error;
    }
  }
  await ensureSignedIn();
}

const FIND_CADENCE = `(() => {
  const links = [...document.querySelectorAll('a[href^="/dashboard/"]')]
    .map((a) => ({ href: a.getAttribute('href'), text: (a.innerText || '').trim() }))
    .filter((item) => /^\\/dashboard\\/[0-9a-f]{32}$/i.test(item.href));
  const match = links.find((item) => item.text.toLowerCase() === "cadence");
  return { clientId: match ? match.href.split("/").pop() : "" };
})()`;

const FORM_READY = `(() => {
  return {
    ok: Boolean(
      document.querySelector("#name") &&
      document.querySelector("#description") &&
      document.querySelector("#newRedirectUri")
    ),
  };
})()`;

const FILL_FORM = `(() => {
  const setValue = (element, value) => {
    const prototype = element instanceof HTMLTextAreaElement
      ? HTMLTextAreaElement.prototype
      : HTMLInputElement.prototype;
    const setter = Object.getOwnPropertyDescriptor(prototype, "value").set;
    setter.call(element, value);
    element.dispatchEvent(new Event("input", { bubbles: true }));
    element.dispatchEvent(new Event("change", { bubbles: true }));
  };
  const name = document.querySelector("#name");
  const description = document.querySelector("#description");
  const redirect = document.querySelector("#newRedirectUri");
  if (!name || !description || !redirect) {
    return { ok: false };
  }
  setValue(name, "Cadence");
  setValue(description, "Cadence is a desktop Spotify client with a no-app web playback runtime.");
  setValue(redirect, ${JSON.stringify(REDIRECT_URI)});
  return { ok: true };
})()`;

const CLICK_ADD = `(() => {
  const add = document.querySelector('button[aria-label="Add redirect URI"]');
  if (!add) return { ok: false };
  if (add.disabled) return { ok: false };
  add.click();
  return { ok: true };
})()`;

const SELECT_APIS = `(() => {
  const webAPI = document.querySelector("#apis-used-1");
  const playback = document.querySelector("#apis-used-3");
  if (!webAPI || !playback) return { ok: false };
  if (!webAPI.checked) webAPI.click();
  if (!playback.checked) playback.click();
  return { ok: true, webAPI: webAPI.checked, playback: playback.checked };
})()`;

const TERMS_AND_SAVE = `(() => {
  const terms = document.querySelector("#termsAccepted");
  const save = document.querySelector('button[type="submit"]');
  const body = (document.body.innerText || "").toLowerCase();
  const appLimit = /maximum number of apps|app limit|too many apps|limit of apps/.test(body);
  // The Cadence action is the user's consent: accept the Spotify Developer
  // Terms instead of sending them to the dashboard themselves.
  if (terms && !terms.checked) terms.click();
  const state = {
    terms: terms ? Boolean(terms.checked) : null,
    saveDisabled: save ? Boolean(save.disabled) : null,
    appLimit,
  };
  if (appLimit) return { ...state, submitted: false };
  const attempts = window.__cadenceSubmitAttempts || 0;
  if (attempts >= 2) return { ...state, submitted: false };
  if (terms && terms.checked && save && !save.disabled) {
    window.__cadenceSubmitAttempts = attempts + 1;
    save.click();
    return { ...state, submitted: true };
  }
  return { ...state, submitted: false };
})()`;

function soloistScript(rotateKey) {
  return `(async () => {
    let csrf = "";
    async function api(endpoint, method) {
      const request = (token) => fetch("/api/wristband" + endpoint, {
        method: method || "GET",
        credentials: "same-origin",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
          "X-CSRF-Token": token,
          "X-Cadence-Developer": "1",
        },
      });
      let response = await request(csrf);
      const next = response.headers.get("X-CSRF-Token");
      if (next) csrf = next;
      if ((method || "GET") !== "GET" && response.headers.get("X-CSRF-Valid") !== "true") {
        response = await request(csrf);
        const retryNext = response.headers.get("X-CSRF-Token");
        if (retryNext) csrf = retryNext;
      }
      const text = await response.text();
      let body = null;
      try { body = text ? JSON.parse(text) : null; } catch { body = null; }
      return { status: response.status, body };
    }

    let consent = await api("/v1/consent");
    if (consent.status === 401 || consent.status === 403) {
      return { state: "not_ready", status: consent.status };
    }
    if (consent.status !== 200) {
      return { state: "failed", status: consent.status };
    }
    if (!consent.body || consent.body.accepted !== true) {
      // The Cadence settings action is the user's consent: accept the Spotify
      // Developer Terms here instead of sending them to the dashboard.
      const accepted = await api("/v1/consent", "POST");
      if (accepted.status < 200 || accepted.status >= 300) {
        return { state: "consent_failed", status: accepted.status };
      }
      consent = await api("/v1/consent");
    }
    if (!consent.body || consent.body.accepted !== true) {
      return { state: "consent_failed", status: consent.status };
    }
    if (consent.body.isPremium !== true) {
      return { state: "premium" };
    }

    if (${rotateKey ? "true" : "false"}) {
      const rotated = await api("/v1/key", "POST");
      if (rotated.status === 200 && rotated.body && rotated.body.key && rotated.body.key.keyValue) {
        return { state: "key", source: "rotated", key: rotated.body.key.keyValue };
      }
      return { state: "key_failed", status: rotated.status };
    }

    const existing = await api("/v1/key");
    if (existing.status === 200 && existing.body && existing.body.key && existing.body.key.keyValue) {
      return { state: "key", source: "existing", key: existing.body.key.keyValue };
    }
    const created = await api("/v1/key", "POST");
    if (created.status === 200 && created.body && created.body.key && created.body.key.keyValue) {
      return { state: "key", source: "created", key: created.body.key.keyValue };
    }
    return { state: "key_failed", status: created.status };
  })()`;
}

function matchClientID(url) {
  const match = String(url).match(
    /^https:\/\/developer\.spotify\.com\/dashboard\/([0-9a-f]{32})(?:[/?#].*)?$/i,
  );
  return match ? match[1] : "";
}

async function runWebApp() {
  await loadPage(DASHBOARD_URL);
  logState("dashboard");

  const existing = await waitFor(async () => {
    const result = await runScript(FIND_CADENCE);
    return result && result.clientId ? result : null;
  }, SEARCH_TIMEOUT_MS, 1000);
  if (existing) {
    await finish({ type: "webApp", clientId: existing.clientId, state: "existing" });
    return;
  }

  await loadPage(CREATE_URL);
  logState("create");
  const ready = await waitFor(async () => {
    const result = await runScript(FORM_READY);
    return result && result.ok ? result : null;
  }, SEARCH_TIMEOUT_MS, 1000);
  if (!ready) {
    await fail("failed", "The Spotify app creation form did not load.");
    return;
  }

  const filled = await runScript(FILL_FORM);
  if (!filled || !filled.ok) {
    await fail("failed", "The Spotify app creation form could not be filled.");
    return;
  }
  await sleep(600);
  await runScript(CLICK_ADD);
  await sleep(600);
  const apis = await waitFor(async () => {
    const result = await runScript(SELECT_APIS);
    return result && result.ok ? result : null;
  }, 30000, 500);
  if (!apis) {
    await fail("failed", "The Spotify app API selection did not load.");
    return;
  }
  logState("form");
  await sleep(600);

  let deadline = Date.now() + SUBMIT_TIMEOUT_MS;
  let lastState = null;
  while (Date.now() < deadline) {
    if (await ensureSignedIn()) {
      deadline = Date.now() + SUBMIT_TIMEOUT_MS;
    }
    const clientID = matchClientID(window.webContents.getURL());
    if (clientID) {
      await finish({ type: "webApp", clientId: clientID, state: "created" });
      return;
    }
    const state = await runScript(TERMS_AND_SAVE);
    if (state) {
      lastState = state;
      if (state.appLimit) {
        await fail(
          "app_limit",
          "Spotify rejected the app because the account reached its app limit.",
        );
        return;
      }
      if (state.submitted) {
        logState("submitted");
      }
    }
    await sleep(1000);
  }

  if (lastState && lastState.saveDisabled) {
    await fail("failed", "Spotify did not enable the app save action.");
    return;
  }
  await fail("failed", "Spotify did not create the app in time.");
}

async function runSoloist() {
  await loadPage(SOLOIST_URL);
  logState("soloist");
  // The dashboard app fetches the wristband API with its own Bearer token.
  // Wait for that token before issuing tagged requests.
  const ready = await waitFor(async () => {
    return authCaptured ? true : null;
  }, SEARCH_TIMEOUT_MS, 500);
  if (!ready) {
    await fail("failed", "The Spotify Developer dashboard session could not be read.");
    return;
  }

  let deadline = Date.now() + SUBMIT_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (await ensureSignedIn()) {
      deadline = Date.now() + SUBMIT_TIMEOUT_MS;
    }
    const result = await runScript(soloistScript(rotate));
    if (!result || typeof result.state !== "string") {
      await sleep(1000);
      continue;
    }
    switch (result.state) {
      case "key":
        await finish({ type: "soloistKey", key: result.key, state: result.source });
        return;
      case "premium":
        await fail("premium_required", "Spotify Premium is required for Soloist.");
        return;
      case "not_ready":
        await sleep(1000);
        continue;
      case "consent_failed":
      case "key_failed":
        await fail("failed", "The Spotify Soloist key request was rejected.");
        return;
      default:
        await fail("failed", "The Spotify Soloist key request failed.");
        return;
    }
  }
  await fail("failed", "Spotify did not return a Soloist API key in time.");
}

// OAuth mode drives the persistent profile to the authorize URL and quits as
// soon as Spotify redirects to the loopback callback. It never sends a Client
// ID, key, code or token over the control socket, and it never logs the URL
// query.
async function runOAuth() {
  logState("oauth");
  try {
    await window.loadURL(oauthURL);
  } catch {
    // Spotify redirects to the login or consent page, which aborts the first
    // load. The polling loop follows the resulting navigation instead.
  }

  const deadline = Date.now() + ACTION_TIMEOUT_MS;
  while (!finished && !oauthCallbackReached && Date.now() < deadline) {
    await ensureSignedIn();
    await sleep(500);
  }
  if (oauthCallbackReached) {
    hideLoginWindow();
    await finish({ type: "oauth", state: "callback" });
    return;
  }
  if (!finished) {
    await loginRequired();
  }
}

async function main() {
  const timeout = setTimeout(() => {
    const onLogin = Boolean(window && isLoginURL(window.webContents.getURL()));
    if (onLogin) {
      void loginRequired();
      return;
    }
    void fail("failed", "The Spotify Developer dashboard automation timed out.");
  }, ACTION_TIMEOUT_MS);
  timeout.unref();

  await app.whenReady();
  logState("ready");
  if (action !== "oauth") {
    await connectControl();
  }

  window = new BrowserWindow({
    width: 1100,
    height: 820,
    show: false,
    title: LOGIN_WINDOW_TITLE,
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      backgroundThrottling: false,
    },
  });
  window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  window.webContents.on("did-navigate", (_event, url) => {
    if (isCallbackURL(url)) {
      oauthCallbackReached = true;
      hideLoginWindow();
    }
    if (!finished) {
      void syncLoginWindowQueued();
    }
  });
  window.on("page-title-updated", (event) => {
    event.preventDefault();
  });
  window.on("closed", () => {
    window = null;
  });

  if (action !== "oauth") {
    installHeaderHook();
  }

  try {
    if (action === "oauth") {
      await runOAuth();
    } else if (action === "web-app") {
      await runWebApp();
    } else {
      await runSoloist();
    }
  } catch (error) {
    if (error instanceof LoginRequired) {
      await loginRequired();
      return;
    }
    logState("error");
    const message = action === "oauth" ? "The Spotify login flow failed." : describe(error);
    await fail("failed", message);
  }
}

app.on("before-quit", () => {
  finished = true;
});

app.on("window-all-closed", () => {
  app.quit();
});

if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  main().catch((error) => {
    logState("fatal");
    const message = action === "oauth" ? "The Spotify login flow failed." : describe(error);
    void fail("failed", message);
  });
}
