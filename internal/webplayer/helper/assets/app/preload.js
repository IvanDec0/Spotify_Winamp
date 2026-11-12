"use strict";

// The only bridge the player page gets. It exposes readiness and error
// reporting to the Electron main process; tokens stay in the renderer's SDK
// callback and are never forwarded here.

const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("cadence", {
  ready(deviceID) {
    if (typeof deviceID !== "string" || deviceID.length === 0) {
      return;
    }
    ipcRenderer.send("cadence:ready", deviceID);
  },
  error(code, message) {
    ipcRenderer.send("cadence:error", String(code || "sdk_error"), String(message || ""));
  },
  state(payload) {
    if (payload && typeof payload === "object") {
      ipcRenderer.send("cadence:state", payload);
    }
  },
});
