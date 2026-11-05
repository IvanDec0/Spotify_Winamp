<script lang="ts">
  import type { Device } from "../../shared/api/types";
  import { t } from "../../shared/i18n";
  import Icon from "./Icon.svelte";

  interface Props {
    devices: Device[];
    onTransfer: (deviceId: string) => void;
    onClose: () => void;
  }

  let { devices, onTransfer, onClose }: Props = $props();
</script>

<div class="devices" role="dialog" aria-modal="true" aria-label={t("device.dialogLabel")}>
  <header class="devices-head">
    <div class="devices-title">
      <span class="eyebrow">{t("device.connect")}</span>
      <h2>{t("device.title")}</h2>
    </div>
    <button class="close" type="button" aria-label={t("device.closePicker")} onclick={onClose}>
      <Icon name="close" size={12} />
    </button>
  </header>

  {#if devices.length === 0}
    <p class="empty">{t("device.empty")}</p>
  {:else}
    <div class="device-list">
      {#each devices as device (device.id)}
        <button
          class="device"
          class:is-active={device.active}
          type="button"
          disabled={device.active || device.restricted}
          aria-label={device.active ? t("device.nameActive", { name: device.name }) : device.name}
          aria-pressed={device.active}
          onclick={() => onTransfer(device.id)}
        >
          <span class="device-icon" aria-hidden="true">
            <Icon name="device" size={13} />
          </span>
          <span class="device-copy">
            <span class="device-name">{device.name}</span>
            <span class="device-meta mono">
              {device.type || t("device.genericType")} · {device.volumePercent}%
              {#if device.restricted}
                · {t("device.restricted")}{/if}
            </span>
          </span>
          {#if device.active}
            <span class="active-mark">
              <Icon name="check" size={11} strokeWidth={2.1} />
            </span>
          {:else}
            <span class="transfer mono">{t("device.use")}</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}

  <p class="hint mono">{t("device.localPlayback")}</p>
</div>

<style>
  .devices {
    width: 228px;
    padding: 11px 12px 10px;
    border: 1px solid var(--line-strong);
    border-radius: 8px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 3%), transparent 30%),
      color-mix(in srgb, var(--surface) 94%, transparent);
    box-shadow:
      0 18px 44px rgb(0 0 0 / 55%),
      0 0 0 1px rgb(0 0 0 / 22%),
      0 1px 0 rgb(255 255 255 / 7%) inset;
    backdrop-filter: blur(16px);
  }

  .devices-head {
    --wails-draggable: drag;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding-bottom: 9px;
    border-bottom: 1px solid var(--line);
    cursor: grab;
  }

  .devices-title {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .eyebrow {
    color: var(--accent);
    font-family: "JetBrains Mono", monospace;
    font-size: 6px;
    letter-spacing: 0.22em;
    line-height: 1;
    text-transform: uppercase;
  }

  h2 {
    margin: 0;
    color: var(--text);
    font-size: 12.5px;
    font-weight: 580;
    letter-spacing: -0.01em;
    line-height: 1.2;
  }

  .close {
    --wails-draggable: no-drag;
    display: grid;
    width: 19px;
    height: 19px;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--faint);
    cursor: pointer;
    transition:
      color 120ms ease,
      background-color 120ms ease;
  }

  .close:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .device-list {
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 6px 0;
  }

  .device {
    display: grid;
    grid-template-columns: 24px minmax(0, 1fr) auto;
    gap: 7px;
    align-items: center;
    width: 100%;
    padding: 5px 5px;
    border: 1px solid transparent;
    border-radius: 5px;
    text-align: left;
    transition:
      border-color 120ms ease,
      background-color 120ms ease;
  }

  .device:hover:not(:disabled) {
    border-color: var(--line);
    background: var(--surface-2);
  }

  .device:disabled {
    cursor: default;
  }

  .device.is-active {
    border-color: color-mix(in srgb, var(--accent) 18%, transparent);
    background: color-mix(in srgb, var(--accent) 6%, transparent);
  }

  .device-icon {
    display: grid;
    width: 24px;
    height: 24px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 50%;
    color: var(--muted);
  }

  .device.is-active .device-icon {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    color: var(--accent);
  }

  .device-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 2px;
  }

  .device-name {
    overflow: hidden;
    color: var(--text);
    font-size: 10px;
    font-weight: 530;
    line-height: 1.2;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .device-meta {
    overflow: hidden;
    color: var(--faint);
    font-size: 6.5px;
    letter-spacing: 0.04em;
    text-overflow: ellipsis;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .active-mark {
    display: grid;
    width: 18px;
    height: 18px;
    place-items: center;
    color: var(--accent);
  }

  .transfer {
    color: var(--faint);
    font-size: 6px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }

  .hint {
    margin: 2px 0 0;
    padding-top: 7px;
    border-top: 1px solid var(--line);
    color: var(--faint);
    font-size: 6.5px;
    letter-spacing: 0.04em;
    line-height: 1.3;
    text-transform: uppercase;
  }

  .empty {
    margin: 12px 0;
    color: var(--faint);
    font-size: 9.5px;
    line-height: 1.4;
  }
</style>
