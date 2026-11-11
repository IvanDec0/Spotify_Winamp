<script lang="ts">
  import { appWindow } from "../../../shared/api";
  import { t } from "../../../shared/i18n";
  import BrandMark from "../../components/BrandMark.svelte";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";
  import { skins } from "../../skins.svelte";

  interface Props {
    brand?: string;
    showBrand?: boolean;
    showMark?: boolean;
    showProvider?: boolean;
    showDevice?: boolean;
    showWindowControls?: boolean;
    compact?: boolean;
  }

  let {
    brand = "CADENCE",
    showBrand = true,
    showMark = true,
    showProvider = true,
    showDevice = true,
    showWindowControls = true,
    compact = false,
  }: Props = $props();

  let deviceLabel = $derived(player.state.device.name || t("device.noDevice"));

  function minimise(): void {
    void appWindow.minimise().catch(() => {});
  }

  function closeWindow(): void {
    void appWindow.close().catch(() => {});
  }
</script>

<header class="cad-titlebar" class:is-compact={compact} data-component="titlebar">
  <div class="cad-brand">
    {#if showMark}
      <span class="cad-brand-mark"><BrandMark size={14} /></span>
    {/if}
    {#if showBrand}<span class="cad-brand-name">{brand}</span>{/if}
    {#if showProvider}
      <button
        class="cad-provider"
        type="button"
        title={t("browse.openWindow")}
        aria-label={t("browse.openWindow")}
        onclick={() => skins.openMusic()}
      >
        {t("browse.open")}
      </button>
    {/if}
  </div>

  <div class="cad-titlebar-grow"></div>

  {#if showDevice}
    <button
      class="cad-device"
      type="button"
      title={t("device.choose")}
      aria-label={t("device.chooseLabel", { name: deviceLabel })}
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
    title={t("browse.settings")}
    aria-label={t("browse.settings")}
    onclick={() => skins.openSettings()}
  >
    <Icon name="settings" size={14} />
  </button>

  {#if skins.hasQueue}
    <button
      class="cad-title-button"
      class:is-active={skins.queueOpen}
      type="button"
      title={skins.queueOpen ? t("titlebar.hideQueue") : t("titlebar.showQueue")}
      aria-label={skins.queueOpen ? t("titlebar.hideQueue") : t("titlebar.showQueue")}
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
      title={t("titlebar.minimise")}
      aria-label={t("titlebar.minimiseWindow")}
      onclick={minimise}
    >
      <Icon name="minimise" size={12} />
    </button>
    <button
      class="cad-title-button cad-window-button close"
      type="button"
      title={t("ui.close")}
      aria-label={t("titlebar.closeWindow")}
      onclick={closeWindow}
    >
      <Icon name="close" size={12} />
    </button>
  {/if}
</header>
