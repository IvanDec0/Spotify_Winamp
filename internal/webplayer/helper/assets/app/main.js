"use strict";

// Cadence web player helper: a minimal Electron main process that renders the
// Spotify Web Playback SDK page served by the Cadence Go process and reports
// the connection state over the loopback control socket.
//
// It never receives OAuth tokens: the renderer fetches them from the local
// server, and this process only relays device/ready/error events.

const path = require("node:path");
const { app, BrowserWindow, components, ipcMain, session } = require("electron");

const HELLO_TIMEOUT_MS = 10000;

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

const controlArgument = readSwitch("cadence-control");
const playerArgument = readSwitch("cadence-url");
const controlURL = parseLoopbackURL(controlArgument, "ws:");
const playerURL = parseLoopbackURL(playerArgument, "http:");

if (!controlURL || !playerURL) {
  process.stderr.write("cadence webplayer: missing or invalid --cadence-control/--cadence-url\n");
  process.exit(2);
}

// These must be set before app.whenReady() to affect Chromium startup.
app.commandLine.appendSwitch("disable-background-timer-throttling");
app.commandLine.appendSwitch("disable-renderer-backgrounding");
app.commandLine.appendSwitch("disable-backgrounding-occluded-windows");

let socket = null;
let playerWindow = null;
let quitting = false;

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

function sendError(code, message) {
  sendControl({ type: "error", code: String(code), message: String(message) });
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
        // The socket may already be closed; nothing else to do.
      }
    }, HELLO_TIMEOUT_MS);

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
      if (!message || typeof message.type !== "string") {
        return;
      }
      switch (message.type) {
        case "show":
          if (playerWindow && !playerWindow.isDestroyed()) {
            playerWindow.showInactive();
          }
          break;
        case "hide":
          if (playerWindow && !playerWindow.isDestroyed()) {
            playerWindow.hide();
          }
          break;
        case "shutdown":
          app.quit();
          break;
        default:
          break;
      }
    });

    ws.addEventListener("error", () => {
      clearTimeout(timer);
      if (!settled) {
        settled = true;
        reject(new Error("control socket connection failed"));
        return;
      }
      if (!quitting) {
        process.stderr.write("cadence webplayer: control socket error\n");
        app.quit();
      }
    });

    ws.addEventListener("close", () => {
      clearTimeout(timer);
      socket = null;
      if (settled && !quitting) {
        app.quit();
      }
    });
  });
}

function installIPC() {
  ipcMain.on("cadence:ready", (_event, deviceID) => {
    if (typeof deviceID !== "string" || deviceID.length === 0) {
      return;
    }
    sendControl({ type: "ready", deviceId: deviceID });
  });
  ipcMain.on("cadence:error", (_event, code, message) => {
    sendError(code || "sdk_error", message || "");
  });
}

function createPlayerWindow() {
  const window = new BrowserWindow({
    width: 480,
    height: 320,
    show: false,
    frame: false,
    skipTaskbar: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      webviewTag: false,
      backgroundThrottling: false,
      spellcheck: false,
    },
  });

  window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));

  window.webContents.on("will-navigate", (event, target) => {
    let allowed = false;
    try {
      allowed = new URL(target).origin === playerURL.origin;
    } catch {
      allowed = false;
    }
    if (!allowed) {
      event.preventDefault();
    }
  });

  window.webContents.on("did-fail-load", (_event, code, description, failedURL, isMainFrame) => {
    if (!isMainFrame) {
      return;
    }
    sendError("load_failed", "the player page failed to load (" + code + ": " + description + ")");
  });

  window.on("closed", () => {
    playerWindow = null;
  });

  return window;
}

async function main() {
  await app.whenReady();

  installIPC();
  session.defaultSession.setPermissionRequestHandler((_webContents, _permission, callback) => {
    callback(false);
  });

  await connectControl();

  try {
    await components.whenReady();
  } catch (error) {
    sendError("widevine", "Widevine initialization failed: " + describe(error));
  }

  playerWindow = createPlayerWindow();
  await playerWindow.loadURL(playerURL.href);
}

app.on("before-quit", () => {
  quitting = true;
});

app.on("window-all-closed", () => {
  app.quit();
});

if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  main().catch((error) => {
    process.stderr.write("cadence webplayer: " + describe(error) + "\n");
    app.quit();
  });
}
