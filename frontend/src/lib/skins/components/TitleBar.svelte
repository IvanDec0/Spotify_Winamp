<script lang="ts">
  import { Window } from "@wailsio/runtime";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";
  import { skins } from "../../skins.svelte";

  interface Props {
    showBrand?: boolean;
    showProvider?: boolean;
    showDevice?: boolean;
    showWindowControls?: boolean;
    compact?: boolean;
  }

  let {
    showBrand = true,
    showProvider = true,
    showDevice = true,
    showWindowControls = true,
    compact = false,
  }: Props = $props();

  let providerLabel = $derived(player.state.provider === "demo" ? "DEMO" : "LOCAL");
  let deviceLabel = $derived(player.state.device.name || "No device");

  function minimise(): void {
    void Window.Minimise().catch(() => {});
  }

  function closeWindow(): void {
    void Window.Close().catch(() => {});
  }
</script>

<header class="cad-titlebar" class:is-compact={compact} data-component="titlebar">
  <div class="cad-brand">
    <svg class="cad-brand-mark" viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M17.2 7.1A7 7 0 1 0 17.2 16.9"
        fill="none"
        stroke="currentColor"
        stroke-width="2.2"
        stroke-linecap="round"
      />
      <circle cx="17.2" cy="7.1" r="1.4" fill="currentColor" />
    </svg>
    {#if showBrand}<span class="cad-brand-name">CADENCE</span>{/if}
    {#if showProvider}
      <button
        class="cad-provider"
        type="button"
        title="Music library"
        aria-label="Open music library"
        onclick={() => skins.openMusic()}
      >
        {providerLabel}
      </button>
    {/if}
  </div>

  <div class="cad-titlebar-grow"></div>

  {#if showDevice}
    <button
      class="cad-device"
      type="button"
      title="Choose a playback device"
      aria-label={`Playback device: ${deviceLabel}. Choose a device`}
      onclick={() => skins.openDevices()}
    >
      <Icon name="device" size={11} />
      <span class="cad-device-name">{deviceLabel}</span>
      <Icon name="chevronDown" size={9} />
    </button>
  {/if}

  <button
    class="cad-title-button"
    type="button"
    title="Skins and appearance"
    aria-label="Open skin settings"
    onclick={() => skins.openSettings()}
  >
    <Icon name="settings" size={14} />
  </button>

  {#if skins.hasQueue}
    <button
      class="cad-title-button"
      class:is-active={skins.queueOpen}
      type="button"
      title={skins.queueOpen ? "Hide queue" : "Show queue"}
      aria-label={skins.queueOpen ? "Hide queue" : "Show queue"}
      aria-pressed={skins.queueOpen}
      onclick={() => skins.toggleQueue()}
    >
      <Icon name="queue" size={14} />
    </button>
  {/if}

  {#if showWindowControls}
    <span class="cad-divider" aria-hidden="true"></span>
    <button
      class="cad-title-button cad-window-button"
      type="button"
      title="Minimise"
      aria-label="Minimise window"
      onclick={minimise}
    >
      <Icon name="minimise" size={12} />
    </button>
    <button
      class="cad-title-button cad-window-button close"
      type="button"
      title="Close"
      aria-label="Close window"
      onclick={closeWindow}
    >
      <Icon name="close" size={12} />
    </button>
  {/if}
</header>
