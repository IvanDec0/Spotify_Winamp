<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Dialog from "../../shared/ui/Dialog.svelte";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import Icon from "./Icon.svelte";
  import { browseSettings } from "./settings.svelte";

  const BITRATES = [96, 160, 320] as const;
  const CACHE_LIMITS = ["512MB", "1GB", "2GB", "4GB", "8GB"] as const;
  const IMAGE_SIZES = ["default", "small", "large", "xlarge"] as const;

  let confirmRestart = $state(false);
  let form = $derived(browseSettings.settings);

  onMount(() => {
    void browseSettings.load();
  });

  function imageLabel(size: (typeof IMAGE_SIZES)[number]): Parameters<typeof t>[0] {
    switch (size) {
      case "small":
        return "browse.settingsImageSmall";
      case "large":
        return "browse.settingsImageLarge";
      case "xlarge":
        return "browse.settingsImageXlarge";
      default:
        return "browse.settingsImageDefault";
    }
  }

  function requestRestart(): void {
    if (!browseSettings.restartRequired || browseSettings.restarting || browseSettings.saving) {
      return;
    }
    confirmRestart = true;
  }

  async function confirmRestartNow(): Promise<void> {
    confirmRestart = false;
    await browseSettings.restart();
  }
</script>

<section class="browse-view settings-view" aria-labelledby="settings-view-title">
  <header class="view-head">
    <div>
      <p class="eyebrow">{t("browse.settings")}</p>
      <h1 id="settings-view-title" class="view-title" tabindex="-1">
        {t("browse.settingsTitle")}
      </h1>
      <p class="view-subtitle">{t("browse.settingsRestartHelp")}</p>
    </div>
    {#if form}
      <div class="settings-actions">
        <Button
          variant="primary"
          size="sm"
          disabled={browseSettings.saving || browseSettings.restarting}
          onclick={() => void browseSettings.save()}
        >
          {browseSettings.saving ? t("browse.settingsSaving") : t("browse.settingsSave")}
        </Button>
        <Button
          variant="secondary"
          size="sm"
          disabled={!browseSettings.restartRequired ||
            browseSettings.restarting ||
            browseSettings.saving}
          title={browseSettings.restartRequired
            ? t("browse.settingsRestart")
            : t("browse.settingsRestartHint")}
          onclick={requestRestart}
        >
          {browseSettings.restarting ? t("browse.settingsRestarting") : t("browse.settingsRestart")}
        </Button>
      </div>
    {/if}
  </header>

  {#if browseSettings.loading && !form}
    <div class="skeleton-wrap" aria-busy="true" aria-label={t("browse.settingsLoading")}>
      <Skeleton rows={9} height={16} />
    </div>
  {:else if !form}
    <div class="empty-panel" role="alert">
      <Icon name="settings" size={22} />
      <h2>{t("browse.settingsLoadError")}</h2>
      {#if browseSettings.error}<p>{browseSettings.error}</p>{/if}
      <Button variant="secondary" size="sm" onclick={() => void browseSettings.load(true)}>
        {t("ui.retry")}
      </Button>
    </div>
  {:else}
    <div class="settings-feedback" aria-live="polite" aria-atomic="true">
      {#if browseSettings.notice}
        <p class="settings-notice is-success">{t(browseSettings.notice)}</p>
      {/if}
    </div>

    {#if browseSettings.error}
      <div class="settings-notice is-error" role="alert" aria-live="assertive">
        <span>{t("browse.settingsError")} {browseSettings.error}</span>
        <button
          class="settings-dismiss"
          type="button"
          title={t("ui.dismiss")}
          aria-label={t("ui.dismiss")}
          onclick={() => browseSettings.dismissError()}
        >
          ×
        </button>
      </div>
    {/if}

    <form
      class="settings-grid"
      aria-busy={browseSettings.saving || browseSettings.restarting}
      onsubmit={(event) => {
        event.preventDefault();
        void browseSettings.save();
      }}
    >
      <fieldset class="settings-section">
        <legend>{t("browse.settingsEngine")}</legend>

        <div class="settings-row">
          <label class="settings-label" for="settings-bitrate">
            {t("browse.settingsBitrate")}
          </label>
          <div class="settings-control">
            <select
              id="settings-bitrate"
              value={form.bitrate}
              aria-describedby="settings-bitrate-help"
              onchange={(event) =>
                browseSettings.update(
                  "bitrate",
                  Number((event.currentTarget as HTMLSelectElement).value),
                )}
            >
              {#each BITRATES as bitrate (bitrate)}
                <option value={bitrate}>
                  {t("browse.settingsBitrateOption", { value: bitrate })}
                </option>
              {/each}
            </select>
          </div>
          <p class="settings-help" id="settings-bitrate-help">
            {t("browse.settingsBitrateHelp")}
          </p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-crossfade">
            {t("browse.settingsCrossfade")}
          </label>
          <div class="settings-control is-range">
            <input
              id="settings-crossfade"
              type="range"
              min="0"
              max="12000"
              step="250"
              value={form.crossfadeDurationMs}
              aria-describedby="settings-crossfade-help"
              oninput={(event) =>
                browseSettings.update(
                  "crossfadeDurationMs",
                  Number((event.currentTarget as HTMLInputElement).value),
                )}
            />
            <output class="settings-value" for="settings-crossfade">
              {t("browse.settingsMilliseconds", { value: form.crossfadeDurationMs })}
            </output>
          </div>
          <p class="settings-help" id="settings-crossfade-help">
            {t("browse.settingsCrossfadeHelp")}
          </p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-normalisation-disabled">
            {t("browse.settingsNormalisationDisabled")}
          </label>
          <div class="settings-control">
            <input
              id="settings-normalisation-disabled"
              type="checkbox"
              checked={form.normalisationDisabled}
              aria-describedby="settings-normalisation-help"
              onchange={(event) =>
                browseSettings.update(
                  "normalisationDisabled",
                  (event.currentTarget as HTMLInputElement).checked,
                )}
            />
          </div>
          <p class="settings-help" id="settings-normalisation-help">
            {t("browse.settingsNormalisationHelp")}
          </p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-album-gain">
            {t("browse.settingsAlbumGain")}
          </label>
          <div class="settings-control">
            <input
              id="settings-album-gain"
              type="checkbox"
              checked={form.normalisationUseAlbumGain}
              disabled={form.normalisationDisabled}
              aria-describedby="settings-album-gain-help"
              onchange={(event) =>
                browseSettings.update(
                  "normalisationUseAlbumGain",
                  (event.currentTarget as HTMLInputElement).checked,
                )}
            />
          </div>
          <p class="settings-help" id="settings-album-gain-help">
            {t("browse.settingsAlbumGainHelp")}
          </p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-pregain">
            {t("browse.settingsPregain")}
          </label>
          <div class="settings-control is-range">
            <input
              id="settings-pregain"
              type="range"
              min="-20"
              max="20"
              step="0.5"
              value={form.normalisationPregain}
              disabled={form.normalisationDisabled}
              aria-describedby="settings-pregain-help"
              oninput={(event) =>
                browseSettings.update(
                  "normalisationPregain",
                  Number((event.currentTarget as HTMLInputElement).value),
                )}
            />
            <output class="settings-value" for="settings-pregain">
              {t("browse.settingsDecibels", { value: form.normalisationPregain })}
            </output>
          </div>
          <p class="settings-help" id="settings-pregain-help">
            {t("browse.settingsPregainHelp")}
          </p>
        </div>
      </fieldset>

      <fieldset class="settings-section">
        <legend>{t("browse.settingsCache")}</legend>

        <div class="settings-row">
          <label class="settings-label" for="settings-cache-enabled">
            {t("browse.settingsCacheEnabled")}
          </label>
          <div class="settings-control">
            <input
              id="settings-cache-enabled"
              type="checkbox"
              checked={form.cacheEnabled}
              aria-describedby="settings-cache-enabled-help"
              onchange={(event) =>
                browseSettings.update(
                  "cacheEnabled",
                  (event.currentTarget as HTMLInputElement).checked,
                )}
            />
          </div>
          <p class="settings-help" id="settings-cache-enabled-help">
            {t("browse.settingsCacheEnabledHelp")}
          </p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-cache-size">
            {t("browse.settingsCacheSize")}
          </label>
          <div class="settings-control">
            <select
              id="settings-cache-size"
              value={form.cacheSizeLimit}
              aria-describedby="settings-cache-size-help"
              onchange={(event) =>
                browseSettings.update(
                  "cacheSizeLimit",
                  (event.currentTarget as HTMLSelectElement).value,
                )}
            >
              {#each CACHE_LIMITS as limit (limit)}
                <option value={limit}>{limit}</option>
              {/each}
            </select>
          </div>
          <p class="settings-help" id="settings-cache-size-help">
            {t("browse.settingsCacheSizeHelp")}
          </p>
        </div>

        <div class="settings-row">
          <span class="settings-label">{t("browse.settingsCacheDir")}</span>
          <div class="settings-control is-wide">
            <output
              class="settings-readonly"
              aria-label={t("browse.settingsCacheDir")}
              aria-describedby="settings-cache-dir-help"
            >
              {form.cacheDir || t("browse.settingsCacheDirDefault")}
            </output>
          </div>
          <p class="settings-help" id="settings-cache-dir-help">
            {t("browse.settingsCacheDirHelp")}
          </p>
        </div>
      </fieldset>

      <fieldset class="settings-section is-wide">
        <legend>{t("browse.settingsFeatures")}</legend>

        <div class="settings-row">
          <label class="settings-label" for="settings-flac">{t("browse.settingsFlac")}</label>
          <div class="settings-control">
            <input
              id="settings-flac"
              type="checkbox"
              checked={form.flacEnabled}
              aria-describedby="settings-flac-help"
              onchange={(event) =>
                browseSettings.update(
                  "flacEnabled",
                  (event.currentTarget as HTMLInputElement).checked,
                )}
            />
          </div>
          <p class="settings-help" id="settings-flac-help">{t("browse.settingsFlacHelp")}</p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-mpris">{t("browse.settingsMpris")}</label>
          <div class="settings-control">
            <input
              id="settings-mpris"
              type="checkbox"
              checked={form.mprisEnabled}
              aria-describedby="settings-mpris-help"
              onchange={(event) =>
                browseSettings.update(
                  "mprisEnabled",
                  (event.currentTarget as HTMLInputElement).checked,
                )}
            />
          </div>
          <p class="settings-help" id="settings-mpris-help">{t("browse.settingsMprisHelp")}</p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-device-group">
            {t("browse.settingsDeviceGroup")}
          </label>
          <div class="settings-control">
            <input
              id="settings-device-group"
              type="checkbox"
              checked={form.deviceIsGroup}
              aria-describedby="settings-device-group-help"
              onchange={(event) =>
                browseSettings.update(
                  "deviceIsGroup",
                  (event.currentTarget as HTMLInputElement).checked,
                )}
            />
          </div>
          <p class="settings-help" id="settings-device-group-help">
            {t("browse.settingsDeviceGroupHelp")}
          </p>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-image-size">
            {t("browse.settingsImageSize")}
          </label>
          <div class="settings-control">
            <select
              id="settings-image-size"
              value={form.imageSize}
              aria-describedby="settings-image-size-help"
              onchange={(event) =>
                browseSettings.update(
                  "imageSize",
                  (event.currentTarget as HTMLSelectElement).value,
                )}
            >
              {#each IMAGE_SIZES as size (size)}
                <option value={size}>{t(imageLabel(size))}</option>
              {/each}
            </select>
          </div>
          <p class="settings-help" id="settings-image-size-help">
            {t("browse.settingsImageSizeHelp")}
          </p>
        </div>
      </fieldset>
    </form>
  {/if}

  <Dialog
    open={confirmRestart}
    title={t("browse.settingsRestartTitle")}
    onclose={() => (confirmRestart = false)}
  >
    <p class="settings-dialog-copy">{t("browse.settingsRestartBody")}</p>
    <div class="settings-dialog-actions">
      <Button variant="ghost" size="sm" onclick={() => (confirmRestart = false)}>
        {t("ui.cancel")}
      </Button>
      <Button variant="primary" size="sm" onclick={() => void confirmRestartNow()}>
        {t("browse.settingsConfirmRestart")}
      </Button>
    </div>
  </Dialog>
</section>
