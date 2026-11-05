<script lang="ts">
  import { onMount } from "svelte";
  import {
    preferences,
    themes,
    visualizerModes,
    type MotionPreference,
    type ThemeId,
    type VisualizerMode,
  } from "../../lib/preferences.svelte";
  import { skins } from "../../lib/skins.svelte";
  import { i18n, t, type Locale } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Dialog from "../../shared/ui/Dialog.svelte";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import Icon from "./Icon.svelte";
  import { browseSettings } from "./settings.svelte";

  const BITRATES = [96, 160, 320] as const;
  const CACHE_LIMITS = ["512MB", "1GB", "2GB", "4GB", "8GB"] as const;
  const IMAGE_SIZES = ["default", "small", "large", "xlarge"] as const;
  const MOTION_OPTIONS = [
    { id: "system", label: "browse.appearance.motionSystem" },
    { id: "reduced", label: "browse.appearance.motionReduced" },
  ] as const;
  // Language names stay in their own language; they are proper names.
  const LANGUAGE_OPTIONS: Array<{ id: Locale; label: string }> = [
    { id: "en", label: "English" },
    { id: "es", label: "Español" },
  ];

  let confirmRestart = $state(false);
  let form = $derived(browseSettings.settings);
  let declarative = $derived(skins.isDeclarative);
  let themeOptions = $derived(
    declarative
      ? (skins.runtime?.themes ?? []).map((id) => ({ id, label: labelFromSlug(id) }))
      : themes.map((theme) => ({ id: theme.id, label: theme.label })),
  );
  let activeTheme = $derived(declarative ? skins.theme : preferences.theme);
  let skinList = $derived(skins.catalog?.skins ?? []);

  onMount(() => {
    void browseSettings.load();
  });

  function labelFromSlug(value: string): string {
    return value.replace(/[-_]/g, " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
  }

  function sourceLabel(source: string): Parameters<typeof t>[0] {
    if (source === "builtin") return "browse.skins.builtin";
    if (source === "development") return "browse.skins.development";
    return "browse.skins.installed";
  }

  function chooseTheme(theme: string): void {
    if (declarative) {
      void skins.setTheme(theme);
      return;
    }
    preferences.setTheme(theme as ThemeId);
  }

  function chooseVisualizer(mode: string): void {
    preferences.setVisualizer(mode as VisualizerMode);
  }

  function chooseMotion(motion: string): void {
    preferences.setMotion(motion as MotionPreference);
  }

  function visualizerLabel(mode: string): Parameters<typeof t>[0] {
    switch (mode) {
      case "radial":
        return "browse.appearance.visualizerRadial";
      case "oscilloscope":
        return "browse.appearance.visualizerOscilloscope";
      case "grid":
        return "browse.appearance.visualizerGrid";
      default:
        return "browse.appearance.visualizerSpectrum";
    }
  }

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

  <form
    class="settings-grid"
    aria-busy={browseSettings.saving || browseSettings.restarting}
    onsubmit={(event) => {
      event.preventDefault();
      void browseSettings.save();
    }}
  >
    {#if browseSettings.loading && !form}
      <div class="skeleton-wrap is-wide" aria-busy="true" aria-label={t("browse.settingsLoading")}>
        <Skeleton rows={9} height={16} />
      </div>
    {:else if !form}
      <div class="empty-panel is-wide" role="alert">
        <Icon name="settings" size={22} />
        <h2>{t("browse.settingsLoadError")}</h2>
        {#if browseSettings.error}<p>{browseSettings.error}</p>{/if}
        <Button variant="secondary" size="sm" onclick={() => void browseSettings.load(true)}>
          {t("ui.retry")}
        </Button>
      </div>
    {:else}
      <div class="settings-feedback is-wide" aria-live="polite" aria-atomic="true">
        {#if browseSettings.notice}
          <p class="settings-notice is-success">{t(browseSettings.notice)}</p>
        {/if}
      </div>

      {#if browseSettings.error}
        <div class="settings-notice is-error is-wide" role="alert" aria-live="assertive">
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
    {/if}

    <fieldset class="settings-section is-wide">
      <legend>{t("browse.appearance.title")}</legend>
      <p class="settings-help is-lead">{t("browse.appearance.immediate")}</p>

      <div class="settings-row">
        <label class="settings-label" for="settings-language">
          {t("browse.appearance.language")}
        </label>
        <div class="settings-control">
          <select
            id="settings-language"
            value={i18n.locale}
            aria-describedby="settings-language-help"
            onchange={(event) =>
              void i18n.setLocale((event.currentTarget as HTMLSelectElement).value as Locale)}
          >
            {#each LANGUAGE_OPTIONS as option (option.id)}
              <option value={option.id}>{option.label}</option>
            {/each}
          </select>
        </div>
        <p class="settings-help" id="settings-language-help">
          {t("browse.appearance.languageHelp")}
        </p>
      </div>

      <div class="settings-row">
        <label class="settings-label" for="settings-theme">
          {t("browse.appearance.theme")}
        </label>
        <div class="settings-control">
          <select
            id="settings-theme"
            value={activeTheme}
            onchange={(event) => chooseTheme((event.currentTarget as HTMLSelectElement).value)}
          >
            {#each themeOptions as option (option.id)}
              <option value={option.id}>{option.label}</option>
            {/each}
          </select>
        </div>
      </div>

      <div class="settings-row">
        <label class="settings-label" for="settings-visualizer">
          {t("browse.appearance.visualizer")}
        </label>
        <div class="settings-control">
          <select
            id="settings-visualizer"
            value={preferences.visualizer}
            onchange={(event) => chooseVisualizer((event.currentTarget as HTMLSelectElement).value)}
          >
            {#each visualizerModes as mode (mode.id)}
              <option value={mode.id}>{t(visualizerLabel(mode.id))}</option>
            {/each}
          </select>
        </div>
      </div>

      <div class="settings-row">
        <label class="settings-label" for="settings-motion">
          {t("browse.appearance.motion")}
        </label>
        <div class="settings-control">
          <select
            id="settings-motion"
            value={preferences.motion}
            aria-describedby="settings-motion-help"
            onchange={(event) => chooseMotion((event.currentTarget as HTMLSelectElement).value)}
          >
            {#each MOTION_OPTIONS as option (option.id)}
              <option value={option.id}>{t(option.label)}</option>
            {/each}
          </select>
        </div>
        <p class="settings-help" id="settings-motion-help">
          {t("browse.appearance.motionHelp")}
          {#if preferences.motion === "system" && preferences.systemReducedMotion}
            {t("browse.appearance.systemReduced")}
          {/if}
        </p>
      </div>
    </fieldset>

    <fieldset class="settings-section is-wide" aria-busy={skins.loading}>
      <legend>{t("browse.skins.title")}</legend>
      <p class="settings-help is-lead">{t("browse.skins.description")}</p>

      {#if skins.loading}
        <div class="skeleton-wrap" aria-busy="true" aria-label={t("browse.skins.loading")}>
          <Skeleton rows={3} height={14} />
        </div>
      {:else}
        {#if skins.catalog?.safeMode}
          <p class="settings-notice is-error" role="status">{t("browse.skins.safeMode")}</p>
        {/if}
        {#if skins.error}
          <div class="settings-notice is-error" role="alert" aria-live="assertive">
            <span>{skins.error}</span>
            <button
              class="settings-dismiss"
              type="button"
              title={t("ui.dismiss")}
              aria-label={t("ui.dismiss")}
              onclick={() => skins.dismissError()}
            >
              ×
            </button>
          </div>
        {/if}
        {#if skins.notice}
          <div class="settings-notice is-success" role="status" aria-live="polite">
            <span>{skins.notice}</span>
            <button
              class="settings-dismiss"
              type="button"
              title={t("ui.dismiss")}
              aria-label={t("ui.dismiss")}
              onclick={() => skins.dismissNotice()}
            >
              ×
            </button>
          </div>
        {/if}

        <div class="skin-actions">
          <Button
            variant="secondary"
            size="sm"
            disabled={skins.loading || skins.catalog?.safeMode}
            onclick={() => void skins.installZip()}
          >
            {t("browse.skins.installZip")}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={skins.loading || skins.catalog?.safeMode}
            onclick={() => void skins.installDirectory()}
          >
            {t("browse.skins.installFolder")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            disabled={skins.loading}
            onclick={() => void skins.reset()}
          >
            {t("browse.skins.reset")}
          </Button>
        </div>

        <div class="settings-row">
          <label class="settings-label" for="settings-third-party">
            {t("browse.skins.thirdPartyDisabled")}
          </label>
          <div class="settings-control">
            <input
              id="settings-third-party"
              type="checkbox"
              checked={skins.catalog?.thirdPartyDisabled ?? false}
              disabled={skins.catalog?.safeMode}
              onchange={(event) =>
                void skins.setThirdPartyDisabled((event.currentTarget as HTMLInputElement).checked)}
            />
          </div>
          <p class="settings-help">{t("browse.skins.thirdPartyHelp")}</p>
        </div>

        {#if skinList.length === 0}
          <p class="empty-panel">{t("browse.skins.empty")}</p>
        {:else}
          <ul class="skin-list">
            {#each skinList as skin (skin.id)}
              <li class="skin-row" class:is-active={skin.active}>
                <div class="skin-copy">
                  <span class="skin-name">
                    {skin.name}
                    <span class="skin-version">{skin.version}</span>
                  </span>
                  <span class="skin-meta">
                    {t(sourceLabel(skin.source))} · {skin.author} · {skin.description}
                  </span>
                </div>
                <div class="skin-row-actions">
                  {#if skin.active}
                    <span class="skin-active">{t("browse.skins.active")}</span>
                  {:else}
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={skins.loading || !skins.catalog?.canChangeActiveSkin}
                      ariaLabel={t("browse.skins.activateNamed", { name: skin.name })}
                      onclick={() => void skins.activate(skin.id)}
                    >
                      {t("browse.skins.activate")}
                    </Button>
                  {/if}
                  {#if skin.removable}
                    <Button
                      variant="ghost"
                      size="sm"
                      class="skin-remove"
                      disabled={skins.loading || skins.catalog?.safeMode}
                      ariaLabel={t("browse.skins.uninstallNamed", { name: skin.name })}
                      onclick={() => void skins.uninstall(skin.id)}
                    >
                      {t("browse.skins.uninstall")}
                    </Button>
                  {/if}
                </div>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    </fieldset>
  </form>

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
