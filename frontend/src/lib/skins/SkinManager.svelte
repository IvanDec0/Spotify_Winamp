<script lang="ts">
  import Icon from "../components/Icon.svelte";
  import {
    preferences,
    themes,
    visualizerModes,
    type MotionPreference,
    type ThemeId,
    type VisualizerMode,
  } from "../preferences.svelte";
  import { skins } from "../skins.svelte";

  interface Props {
    onOpenSpotify: () => void;
    onClose: () => void;
  }

  let { onOpenSpotify, onClose }: Props = $props();

  let installing = $state<"zip" | "directory" | null>(null);
  let pendingRemove = $state<string | null>(null);
  let resetArmed = $state(false);

  let catalog = $derived(skins.catalog);
  let declarative = $derived(skins.isDeclarative);
  let activeSkin = $derived(skins.activeSkin);
  let skinList = $derived(catalog?.skins ?? []);
  let themeOptions = $derived(
    declarative ? (skins.runtime?.themes ?? []) : themes.map((item) => item.id),
  );
  let activeTheme = $derived(declarative ? skins.theme : preferences.theme);
  let modeOptions = $derived(
    declarative ? (skins.runtime?.visualizerModes ?? []) : visualizerModes.map((item) => item.id),
  );
  let activeMode = $derived(declarative ? skins.visualizer : preferences.visualizer);

  function label(value: string): string {
    return value.replace(/[-_]/g, " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
  }

  function modeLabel(value: string): string {
    return visualizerModes.find((item) => item.id === value)?.label ?? label(value);
  }

  function sourceLabel(source: string): string {
    if (source === "builtin") return "Built-in";
    if (source === "development") return "Development";
    return "Installed";
  }

  async function install(kind: "zip" | "directory"): Promise<void> {
    installing = kind;
    try {
      if (kind === "zip") await skins.installZip();
      else await skins.installDirectory();
    } finally {
      installing = null;
    }
  }

  async function chooseTheme(theme: string): Promise<void> {
    if (declarative) await skins.setTheme(theme);
    else preferences.setTheme(theme as ThemeId);
  }

  function chooseMode(mode: string): void {
    if (declarative) skins.setVisualizer(mode);
    else preferences.setVisualizer(mode as VisualizerMode);
  }

  function chooseMotion(motion: MotionPreference): void {
    preferences.setMotion(motion);
  }

  async function remove(id: string): Promise<void> {
    if (pendingRemove !== id) {
      pendingRemove = id;
      return;
    }
    pendingRemove = null;
    await skins.uninstall(id);
  }

  async function reset(): Promise<void> {
    if (!resetArmed) {
      resetArmed = true;
      return;
    }
    resetArmed = false;
    await skins.reset();
  }
</script>

<div class="skin-manager" role="dialog" aria-modal="true" aria-label="Skin library">
  <header class="manager-head">
    <div class="manager-title">
      <span class="eyebrow">Cadence</span>
      <h2>Skins</h2>
    </div>
    <div class="head-actions">
      <button class="spotify-link" type="button" onclick={onOpenSpotify}>Spotify</button>
      <button class="close" type="button" aria-label="Close skin library" onclick={onClose}>
        <Icon name="close" size={12} />
      </button>
    </div>
  </header>

  <div class="manager-body">
    {#if catalog?.safeMode}
      <p class="banner warn">
        Safe mode is active. Third-party skins are disabled until Cadence restarts without
        <code>--safe-mode</code>.
      </p>
    {/if}
    {#if catalog?.lastError}
      <p class="banner error">{catalog.lastError}</p>
    {/if}
    {#if skins.error}
      <div class="banner error banner-row">
        <span>{skins.error}</span>
        <button type="button" onclick={() => skins.dismissError()}>Dismiss</button>
      </div>
    {/if}
    {#if skins.notice}
      <div class="banner info banner-row">
        <span>{skins.notice}</span>
        <button type="button" onclick={() => skins.dismissNotice()}>Dismiss</button>
      </div>
    {/if}

    <section class="active-card" aria-label="Active skin">
      <div class="active-head">
        <div>
          <span class="eyebrow">Active skin</span>
          <strong>{activeSkin?.name ?? "Clarity"}</strong>
          <span class="version">v{activeSkin?.version ?? "1.0.0"}</span>
        </div>
        <span class="source">{sourceLabel(activeSkin?.source ?? "builtin")}</span>
      </div>
      {#if activeSkin?.description}
        <p class="description">{activeSkin.description}</p>
      {/if}

      <div class="setting-row">
        <span class="setting-label">Theme</span>
        <div class="chips" role="group" aria-label="Skin theme">
          {#each themeOptions as theme (theme)}
            <button
              class="chip"
              class:is-active={activeTheme === theme}
              type="button"
              aria-pressed={activeTheme === theme}
              onclick={() => void chooseTheme(theme)}
            >
              {label(theme)}
            </button>
          {/each}
        </div>
      </div>

      <div class="setting-row">
        <span class="setting-label">Visualizer</span>
        <div class="chips" role="group" aria-label="Visualizer mode">
          {#each modeOptions as mode (mode)}
            <button
              class="chip"
              class:is-active={activeMode === mode}
              type="button"
              aria-pressed={activeMode === mode}
              onclick={() => chooseMode(mode)}
            >
              {modeLabel(mode)}
            </button>
          {/each}
        </div>
      </div>

      <div class="setting-row">
        <span class="setting-label">Motion</span>
        <div class="chips" role="group" aria-label="Motion preference">
          <button
            class="chip"
            class:is-active={preferences.motion === "system"}
            type="button"
            aria-pressed={preferences.motion === "system"}
            onclick={() => chooseMotion("system")}
          >
            Auto
          </button>
          <button
            class="chip"
            class:is-active={preferences.motion === "reduced"}
            type="button"
            aria-pressed={preferences.motion === "reduced"}
            onclick={() => chooseMotion("reduced")}
          >
            Reduced
          </button>
        </div>
      </div>
    </section>

    <div class="install-row">
      <button
        class="action"
        type="button"
        disabled={!catalog?.canInstall || installing !== null}
        onclick={() => void install("zip")}
      >
        {installing === "zip" ? "Installing…" : "Install .zip…"}
      </button>
      <button
        class="action"
        type="button"
        disabled={!catalog?.canInstall || installing !== null}
        onclick={() => void install("directory")}
      >
        {installing === "directory" ? "Installing…" : "Install folder…"}
      </button>
    </div>
    <p class="hint">
      Packages are validated before installation. Installed skins are not activated automatically.
    </p>

    <ul class="skin-list" aria-label="Installed skins">
      {#each skinList as skin (skin.id)}
        <li class="skin-row" class:is-active={skin.active}>
          <div class="skin-copy">
            <div class="skin-name">
              <strong>{skin.name}</strong>
              <span class="version">v{skin.version}</span>
              {#if skin.active}<span class="active-badge">Active</span>{/if}
            </div>
            <div class="skin-meta">
              <span>{sourceLabel(skin.source)}</span>
              <span>{skin.author}</span>
              <span>{skin.license}</span>
            </div>
            {#if skin.description}<p class="skin-description">{skin.description}</p>{/if}
          </div>
          <div class="skin-actions">
            {#if !skin.active}
              <button
                class="mini-action"
                type="button"
                disabled={!catalog?.canChangeActiveSkin && skin.source !== "builtin"}
                onclick={() => void skins.activate(skin.id)}
              >
                Use
              </button>
            {/if}
            {#if skin.removable}
              <button
                class="mini-action danger"
                class:is-armed={pendingRemove === skin.id}
                type="button"
                onclick={() => void remove(skin.id)}
              >
                {pendingRemove === skin.id ? "Confirm" : "Remove"}
              </button>
            {/if}
          </div>
        </li>
      {/each}
    </ul>

    <div class="security-row">
      <button
        class="toggle"
        class:is-on={!catalog?.thirdPartyDisabled}
        type="button"
        aria-pressed={!catalog?.thirdPartyDisabled}
        disabled={catalog?.safeMode || !catalog}
        onclick={() => void skins.setThirdPartyDisabled(!catalog?.thirdPartyDisabled)}
      >
        <span class="toggle-track" aria-hidden="true"></span>
        Allow third-party skins
      </button>
      <button
        class="mini-action"
        class:is-armed={resetArmed}
        type="button"
        onclick={() => void reset()}
      >
        {resetArmed ? "Confirm reset" : "Reset skin settings"}
      </button>
    </div>

    {#if catalog?.warnings?.length}
      <details class="warnings">
        <summary
          >{catalog.warnings.length} warning{catalog.warnings.length === 1 ? "" : "s"}</summary
        >
        <ul>
          {#each catalog.warnings as warning, index (index)}
            <li>{warning}</li>
          {/each}
        </ul>
      </details>
    {/if}
  </div>
</div>

<style>
  .skin-manager {
    display: flex;
    height: 100%;
    flex-direction: column;
    color: var(--text);
    background: var(--bg);
    font-family: "Inter", "Segoe UI", system-ui, sans-serif;
  }

  .manager-head {
    display: flex;
    height: 34px;
    flex: 0 0 34px;
    align-items: center;
    justify-content: space-between;
    padding: 0 10px 0 13px;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--bg) 80%, transparent);
  }

  .manager-title {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  h2 {
    margin: 0;
    font-size: 14px;
    font-weight: 620;
    letter-spacing: -0.02em;
  }

  .eyebrow {
    color: var(--accent);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    letter-spacing: 0.2em;
    text-transform: uppercase;
  }

  .head-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .spotify-link {
    height: 20px;
    padding: 0 5px;
    border: 1px solid var(--line);
    border-radius: 4px;
    color: var(--muted);
    font-size: 7.5px;
  }

  .spotify-link:hover {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    color: var(--accent);
  }

  .close {
    display: grid;
    width: 22px;
    height: 22px;
    place-items: center;
    border-radius: 4px;
    color: var(--faint);
  }

  .close:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .manager-body {
    overflow: auto;
    flex: 1 1 auto;
    padding: 8px 12px 12px;
    scrollbar-width: thin;
  }

  .banner {
    margin: 0 0 7px;
    padding: 5px 7px;
    border: 1px solid var(--line);
    border-radius: 4px;
    font-size: 8px;
    line-height: 1.35;
  }

  .banner.warn {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    color: color-mix(in srgb, var(--accent) 80%, var(--text));
    background: color-mix(in srgb, var(--accent) 7%, transparent);
  }

  .banner.error {
    border-color: color-mix(in srgb, var(--danger) 35%, transparent);
    color: color-mix(in srgb, var(--danger) 80%, var(--text));
    background: color-mix(in srgb, var(--danger) 7%, transparent);
  }

  .banner.info {
    border-color: color-mix(in srgb, var(--accent) 26%, transparent);
    color: var(--text);
    background: color-mix(in srgb, var(--accent) 6%, transparent);
  }

  .banner-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
  }

  .banner-row button {
    flex: 0 0 auto;
    color: var(--muted);
    font-size: 7.5px;
    text-decoration: underline;
  }

  .active-card {
    margin-bottom: 8px;
    padding: 7px 8px 6px;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background: color-mix(in srgb, var(--surface) 80%, transparent);
  }

  .active-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
  }

  .active-head strong {
    margin-left: 5px;
    font-size: 11px;
  }

  .version,
  .source {
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
  }

  .source {
    flex: 0 0 auto;
    padding: 1px 4px;
    border: 1px solid var(--line);
    border-radius: 3px;
    text-transform: uppercase;
  }

  .description,
  .skin-description {
    overflow: hidden;
    margin: 4px 0 0;
    color: var(--muted);
    font-size: 8px;
    line-height: 1.35;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .setting-row {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    margin-top: 5px;
  }

  .setting-label {
    width: 52px;
    flex: 0 0 52px;
    padding-top: 2px;
    color: var(--faint);
    font-size: 7px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
  }

  .chip {
    padding: 2px 5px;
    border: 1px solid var(--line);
    border-radius: 3px;
    color: var(--muted);
    font-size: 7.5px;
    line-height: 1.3;
  }

  .chip:hover {
    border-color: var(--line-strong);
    color: var(--text);
  }

  .chip.is-active {
    border-color: color-mix(in srgb, var(--accent) 40%, transparent);
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--accent);
  }

  .install-row {
    display: flex;
    gap: 5px;
  }

  .action,
  .mini-action {
    border: 1px solid var(--line-strong);
    border-radius: 4px;
    color: var(--text);
    background: var(--surface-2);
    transition:
      border-color 120ms ease,
      background-color 120ms ease;
  }

  .action {
    flex: 1 1 0;
    padding: 4px 6px;
    font-size: 8px;
  }

  .action:hover:not(:disabled),
  .mini-action:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--accent) 40%, transparent);
    background: color-mix(in srgb, var(--accent) 10%, var(--surface-2));
  }

  .hint {
    margin: 4px 0 8px;
    color: var(--faint);
    font-size: 7px;
    line-height: 1.35;
  }

  .skin-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .skin-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 5px 7px;
    border: 1px solid var(--line);
    border-radius: 4px;
  }

  .skin-row.is-active {
    border-color: color-mix(in srgb, var(--accent) 28%, transparent);
    background: color-mix(in srgb, var(--accent) 5%, transparent);
  }

  .skin-copy {
    min-width: 0;
    flex: 1 1 auto;
  }

  .skin-name {
    display: flex;
    align-items: baseline;
    gap: 5px;
  }

  .skin-name strong {
    overflow: hidden;
    font-size: 9px;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .active-badge {
    padding: 1px 3px;
    border-radius: 2px;
    background: color-mix(in srgb, var(--accent) 14%, transparent);
    color: var(--accent);
    font-family: "JetBrains Mono", monospace;
    font-size: 5.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .skin-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    color: var(--faint);
    font-size: 6.5px;
  }

  .skin-actions {
    display: flex;
    flex: 0 0 auto;
    gap: 3px;
  }

  .mini-action {
    padding: 3px 6px;
    font-size: 7px;
  }

  .mini-action.danger {
    color: color-mix(in srgb, var(--danger) 78%, var(--text));
  }

  .mini-action.is-armed {
    border-color: color-mix(in srgb, var(--danger) 50%, transparent);
    background: color-mix(in srgb, var(--danger) 13%, transparent);
    color: var(--danger);
  }

  .security-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-top: 8px;
    padding-top: 7px;
    border-top: 1px solid var(--line);
  }

  .toggle {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--muted);
    font-size: 7.5px;
  }

  .toggle-track {
    position: relative;
    width: 20px;
    height: 10px;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background: var(--surface-2);
  }

  .toggle-track::after {
    position: absolute;
    top: 1px;
    left: 1px;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--faint);
    content: "";
    transition: transform 120ms ease;
  }

  .toggle.is-on .toggle-track {
    border-color: color-mix(in srgb, var(--accent) 50%, transparent);
    background: color-mix(in srgb, var(--accent) 18%, transparent);
  }

  .toggle.is-on .toggle-track::after {
    background: var(--accent);
    transform: translateX(10px);
  }

  .warnings {
    margin-top: 8px;
    color: var(--faint);
    font-size: 7px;
  }

  .warnings summary {
    cursor: pointer;
  }

  .warnings ul {
    margin: 4px 0 0;
    padding-left: 12px;
    line-height: 1.4;
  }
</style>
