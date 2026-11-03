<script lang="ts">
  import Icon from "./Icon.svelte";

  interface Props {
    provider: string;
    deviceName: string;
    queueOpen: boolean;
    onToggleQueue: () => void;
    onOpenDevices: () => void;
    onOpenSettings: () => void;
    onOpenSpotify: () => void;
    onMinimise: () => void;
    onClose: () => void;
  }

  let {
    provider,
    deviceName,
    queueOpen,
    onToggleQueue,
    onOpenDevices,
    onOpenSettings,
    onOpenSpotify,
    onMinimise,
    onClose,
  }: Props = $props();

  let providerLabel = $derived(provider === "demo" ? "DEMO" : "CONNECT");
  let deviceLabel = $derived(deviceName || "No device");
</script>

<header class="titlebar" aria-label="Window title bar">
  <div class="brand">
    <svg class="brand-mark" viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M17.2 7.1A7 7 0 1 0 17.2 16.9"
        fill="none"
        stroke="currentColor"
        stroke-width="2.2"
        stroke-linecap="round"
      />
      <circle cx="17.2" cy="7.1" r="1.4" fill="currentColor" />
    </svg>
    <span class="brand-name">CADENCE</span>
    <button
      class="provider-badge"
      type="button"
      title="Spotify account"
      aria-label="Open Spotify account"
      onclick={onOpenSpotify}
    >
      {providerLabel}
    </button>
  </div>

  <div class="titlebar-grow"></div>

  <button
    class="device-chip"
    type="button"
    title="Choose a playback device"
    aria-label={`Playback device: ${deviceLabel}. Choose a device`}
    onclick={onOpenDevices}
  >
    <Icon name="device" size={11} />
    <span class="device-name">{deviceLabel}</span>
    <Icon name="chevronDown" size={9} />
  </button>

  <button
    class="title-button"
    type="button"
    title="Appearance and motion"
    aria-label="Open appearance settings"
    onclick={onOpenSettings}
  >
    <Icon name="settings" size={14} />
  </button>

  <button
    class="title-button"
    class:is-active={queueOpen}
    type="button"
    title={queueOpen ? "Hide queue" : "Show queue"}
    aria-label={queueOpen ? "Hide queue" : "Show queue"}
    aria-pressed={queueOpen}
    onclick={onToggleQueue}
  >
    <Icon name="queue" size={14} />
  </button>

  <span class="title-divider" aria-hidden="true"></span>

  <button
    class="title-button window-button"
    type="button"
    title="Minimise"
    aria-label="Minimise window"
    onclick={onMinimise}
  >
    <Icon name="minimise" size={12} />
  </button>

  <button
    class="title-button window-button close-button"
    type="button"
    title="Close"
    aria-label="Close window"
    onclick={onClose}
  >
    <Icon name="close" size={12} />
  </button>
</header>

<style>
  .titlebar {
    --wails-draggable: drag;
    position: relative;
    z-index: 20;
    display: flex;
    height: 30px;
    flex: 0 0 30px;
    align-items: center;
    gap: 4px;
    padding: 0 7px 0 12px;
    border-bottom: 1px solid var(--line);
    background:
      linear-gradient(180deg, rgb(255 255 255 / 3%), transparent),
      color-mix(in srgb, var(--bg) 76%, transparent);
    backdrop-filter: blur(12px);
  }

  .brand {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 6px;
  }

  .brand-mark {
    width: 14px;
    height: 14px;
    color: var(--accent);
    filter: drop-shadow(0 0 5px var(--accent-glow));
  }

  .brand-name {
    color: color-mix(in srgb, var(--text) 88%, transparent);
    font-family: "Inter", sans-serif;
    font-size: 8.5px;
    font-weight: 700;
    letter-spacing: 0.22em;
    white-space: nowrap;
  }

  .provider-badge {
    --wails-draggable: no-drag;
    flex: 0 0 auto;
    padding: 2px 4px 1px;
    border: 1px solid color-mix(in srgb, var(--accent) 28%, transparent);
    border-radius: 3px;
    background: transparent;
    color: color-mix(in srgb, var(--accent) 74%, var(--muted));
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    font-weight: 600;
    letter-spacing: 0.1em;
    line-height: 1;
    cursor: pointer;
  }

  .provider-badge:hover {
    border-color: color-mix(in srgb, var(--accent) 55%, transparent);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .titlebar-grow {
    flex: 1 1 auto;
  }

  .device-chip {
    --wails-draggable: no-drag;
    display: flex;
    min-width: 0;
    max-width: 118px;
    height: 21px;
    align-items: center;
    gap: 5px;
    padding: 0 5px 0 6px;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--muted);
    transition:
      color 140ms ease,
      border-color 140ms ease,
      background-color 140ms ease;
  }

  .device-chip:hover {
    border-color: var(--line);
    background: var(--surface-2);
    color: var(--text);
  }

  .device-name {
    overflow: hidden;
    font-size: 9px;
    font-weight: 500;
    letter-spacing: 0.01em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .title-button {
    --wails-draggable: no-drag;
    display: grid;
    width: 22px;
    height: 22px;
    flex: 0 0 22px;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--faint);
    transition:
      color 140ms ease,
      border-color 140ms ease,
      background-color 140ms ease;
  }

  .title-button:hover {
    border-color: var(--line);
    background: var(--surface-2);
    color: var(--text);
  }

  .title-button.is-active {
    border-color: color-mix(in srgb, var(--accent) 24%, transparent);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .title-divider {
    width: 1px;
    height: 12px;
    margin: 0 2px;
    background: var(--line);
  }

  .window-button:hover {
    color: var(--text);
  }

  .close-button:hover {
    border-color: color-mix(in srgb, var(--danger) 35%, transparent);
    background: color-mix(in srgb, var(--danger) 14%, transparent);
    color: var(--danger);
  }
</style>
