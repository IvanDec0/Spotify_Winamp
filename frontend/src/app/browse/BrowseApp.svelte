<script lang="ts">
  import { onMount, tick } from "svelte";
  import { music } from "../../lib/music.svelte";
  import { player } from "../../lib/player.svelte";
  import { preferences } from "../../lib/preferences.svelte";
  import { skins } from "../../lib/skins.svelte";
  import { appWindow, onBrowseNavigate } from "../../shared/api";
  import { t } from "../../shared/i18n";
  import EngineGate from "./EngineGate.svelte";
  import EntityView from "./EntityView.svelte";
  import Icon from "./Icon.svelte";
  import type { BrowseIcon } from "./icons";
  import LibraryView from "./LibraryView.svelte";
  import NowPlayingBar from "./NowPlayingBar.svelte";
  import PodcastsView from "./PodcastsView.svelte";
  import QueueView from "./QueueView.svelte";
  import SearchView from "./SearchView.svelte";
  import SettingsView from "./SettingsView.svelte";
  import { browse, normalizeSection, type Section } from "./browse.svelte";
  import { applyTokens, readSkinTokens, syncBrowseStylesheet } from "./theme.svelte";

  const SECTIONS: ReadonlyArray<{
    id: Section;
    label: Parameters<typeof t>[0];
    glyph: BrowseIcon;
  }> = [
    { id: "search", label: "browse.search", glyph: "search" },
    { id: "library", label: "browse.library", glyph: "library" },
    { id: "podcasts", label: "browse.podcasts", glyph: "podcasts" },
    { id: "queue", label: "browse.queue", glyph: "queue" },
    { id: "settings", label: "browse.settings", glyph: "settings" },
  ];

  let root: HTMLDivElement;
  let searchInputEl: HTMLInputElement;
  let themeRequest = 0;
  let searchInput = $derived(browse.query);
  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  let previousSection: Section = browse.section;

  let navItems = $derived(SECTIONS.map((item) => ({ ...item, label: t(item.label) })));
  let skinLabel = $derived.by(() => {
    const runtime = skins.runtime;
    if (!runtime || runtime.builtin) {
      return t("browse.skinTheme", { skin: "Clarity", theme: preferences.theme });
    }
    return t("browse.skinTheme", { skin: runtime.name, theme: skins.theme });
  });

  $effect(() => {
    const runtime = skins.runtime;
    const theme = runtime && !runtime.builtin ? skins.theme : preferences.theme;
    const target = root;
    if (!target) return;

    const request = ++themeRequest;
    let active = true;
    void readSkinTokens(runtime, theme).then((tokens) => {
      if (!active || request !== themeRequest) return;
      applyTokens(target, tokens);
    });
    return () => {
      active = false;
    };
  });

  $effect(() => {
    const runtime = skins.runtime;
    const version = runtime ? `${runtime.id}@${runtime.version}` : "";
    syncBrowseStylesheet(
      runtime && !runtime.builtin ? (runtime.browseStylesheetUrl ?? "") : "",
      version,
    );
  });

  $effect(() => {
    void browse.syncTrackLike(player.state.track.uri);
  });

  $effect(() => {
    const section = browse.section;
    if (section === previousSection) return;
    previousSection = section;
    if (section === "search") {
      void tick().then(() => searchInputEl?.focus());
    }
  });

  onMount(() => {
    const unsubscribeNavigate = onBrowseNavigate((section) => {
      const next = normalizeSection(section, "library");
      if (music.authenticated) browse.setSection(next);
      else browse.setInitialSection(next);
    });
    return () => {
      unsubscribeNavigate();
      if (searchTimer) clearTimeout(searchTimer);
    };
  });

  function scheduleSearch(value: string): void {
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      searchTimer = null;
      void submitSearch(value, false);
    }, 300);
  }

  async function submitSearch(value = searchInput, explicit = false): Promise<void> {
    if (searchTimer) {
      clearTimeout(searchTimer);
      searchTimer = null;
    }
    if (!music.authenticated) return;
    if (!value.trim()) {
      browse.clearSearch();
      return;
    }
    if (browse.section !== "search") browse.setSection("search");
    if (explicit) browse.recordSearch(value);
    await browse.search(value);
  }

  function selectSection(section: Section): void {
    if (searchTimer) {
      clearTimeout(searchTimer);
      searchTimer = null;
    }
    if (music.authenticated) browse.setSection(section);
    else browse.setInitialSection(section);
    if (section === "search") {
      void tick().then(() => searchInputEl?.focus());
    }
  }

  function clearSearch(): void {
    if (searchTimer) {
      clearTimeout(searchTimer);
      searchTimer = null;
    }
    searchInput = "";
    browse.clearSearch();
  }

  function closeToPlayer(): void {
    void appWindow.closeBrowse().catch((error: unknown) => {
      browse.error = error instanceof Error ? error.message : String(error);
    });
  }
</script>

<div class="cadence-browse" bind:this={root}>
  <aside class="sidebar">
    <div class="sidebar-head">
      <span class="eyebrow">{t("browse.open")}</span>
      <span class="sidebar-caption">{t("browse.navLabel")}</span>
    </div>
    <nav class="nav" aria-label={t("browse.navLabel")}>
      {#each navItems as item (item.id)}
        <button
          class="nav-item"
          class:is-active={browse.section === item.id}
          type="button"
          aria-current={browse.section === item.id ? "page" : undefined}
          onclick={() => selectSection(item.id)}
        >
          <Icon name={item.glyph} size={16} />
          <span>{item.label}</span>
        </button>
      {/each}
    </nav>
    <div class="sidebar-foot">
      <div class="sidebar-status">
        <span class="status-dot" aria-hidden="true"></span>
        <span>{skins.loading ? t("ui.loading") : t("browse.catalogStatus")}</span>
      </div>
      <button
        class="back-to-player"
        type="button"
        title={t("browse.backToPlayer")}
        aria-label={t("browse.backToPlayer")}
        onclick={closeToPlayer}
      >
        <Icon name="back" size={13} />
        <span>{t("browse.backToPlayer")}</span>
      </button>
    </div>
  </aside>

  <header class="topbar">
    <div class="wordmark">
      <strong>Cadence</strong>
      <span>{t("browse.open")}</span>
    </div>

    <form
      class="search-field"
      role="search"
      onsubmit={(event) => {
        event.preventDefault();
        void submitSearch(undefined, true);
      }}
    >
      <Icon name="search" size={14} />
      <input
        bind:this={searchInputEl}
        id="browse-search-input"
        type="search"
        value={searchInput}
        placeholder={t("browse.searchPlaceholder")}
        aria-label={t("browse.searchPlaceholder")}
        disabled={!music.authenticated}
        autocomplete="off"
        spellcheck="false"
        oninput={(event) => {
          const value = (event.currentTarget as HTMLInputElement).value;
          searchInput = value;
          scheduleSearch(value);
        }}
      />
      {#if searchInput}
        <button
          class="search-clear"
          type="button"
          title={t("browse.clearSearch")}
          aria-label={t("browse.clearSearch")}
          onclick={clearSearch}
        >
          ×
        </button>
      {/if}
    </form>

    <span class="theme-chip" title={skinLabel}>{skinLabel}</span>
  </header>

  <main class="content">
    {#if browse.error || browse.notice}
      <div class="browse-banners">
        {#if browse.error}
          <div class="browse-banner is-error" role="alert" aria-live="assertive" aria-atomic="true">
            <span>{browse.error}</span>
            <button type="button" onclick={() => browse.dismissError()}>{t("ui.dismiss")}</button>
          </div>
        {/if}
        {#if browse.notice}
          <div class="browse-banner is-info" role="status" aria-live="polite" aria-atomic="true">
            <span>{t(browse.notice.key, browse.notice.params)}</span>
            <button type="button" onclick={() => browse.dismissNotice()}>{t("ui.dismiss")}</button>
          </div>
        {/if}
      </div>
    {/if}

    {#if browse.section !== "settings" && !music.authenticated}
      <EngineGate />
    {:else if browse.route.name === "entity"}
      <EntityView />
    {:else if browse.section === "search"}
      <SearchView />
    {:else if browse.section === "library"}
      <LibraryView />
    {:else if browse.section === "podcasts"}
      <PodcastsView />
    {:else if browse.section === "queue"}
      <QueueView />
    {:else}
      <SettingsView />
    {/if}
  </main>

  <NowPlayingBar />
</div>

<style>
  .cadence-browse {
    display: grid;
    height: 100%;
    min-height: 0;
    grid-template-columns: 200px minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr) auto;
    overflow: hidden;
    background:
      radial-gradient(
        80% 60% at 0% 0%,
        color-mix(in srgb, var(--accent) 7%, transparent),
        transparent 55%
      ),
      var(--bg);
    color: var(--text);
    font-family: var(--font-ui);
  }

  .sidebar {
    display: flex;
    min-height: 0;
    flex-direction: column;
    grid-row: 1 / 4;
    gap: 18px;
    padding: 20px 14px 16px;
    border-right: 1px solid var(--line);
    background: color-mix(in srgb, var(--surface) 70%, transparent);
  }

  .sidebar-head {
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 0 8px;
  }

  .sidebar-caption {
    color: var(--faint);
    font-size: 10px;
  }

  .nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .nav-item {
    display: flex;
    width: 100%;
    align-items: center;
    gap: 10px;
    padding: 9px 10px;
    border: 1px solid transparent;
    border-radius: 8px;
    color: var(--muted);
    font-size: 12px;
    text-align: left;
    transition:
      background-color var(--motion-fast, 120ms) ease,
      border-color var(--motion-fast, 120ms) ease,
      color var(--motion-fast, 120ms) ease;
  }

  .nav-item:hover {
    background: color-mix(in srgb, var(--text) 5%, transparent);
    color: var(--text);
  }

  .nav-item.is-active {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    background: color-mix(in srgb, var(--accent) 12%, var(--surface-2));
    color: var(--text);
  }

  .sidebar-foot {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
    margin-top: auto;
    padding: 8px;
    color: var(--faint);
    font-size: 10px;
  }

  .sidebar-status {
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .back-to-player {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 7px 8px;
    border: 1px solid var(--line);
    border-radius: 7px;
    color: var(--muted);
    font-size: 10px;
    text-align: left;
    transition:
      border-color var(--motion-fast, 120ms) ease,
      background-color var(--motion-fast, 120ms) ease,
      color var(--motion-fast, 120ms) ease;
  }

  .back-to-player:hover {
    border-color: color-mix(in srgb, var(--accent) 32%, var(--line));
    background: color-mix(in srgb, var(--accent) 7%, transparent);
    color: var(--text);
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 6px color-mix(in srgb, var(--accent) 45%, transparent);
  }

  .topbar {
    display: flex;
    min-width: 0;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 12px 24px;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--panel) 70%, transparent);
  }

  .wordmark {
    display: flex;
    min-width: 0;
    align-items: baseline;
    gap: 8px;
    font-size: 18px;
    letter-spacing: -0.03em;
  }

  .wordmark strong {
    font-weight: 650;
  }

  .wordmark span {
    color: var(--muted);
    font-weight: 450;
  }

  .search-field {
    display: flex;
    min-width: 0;
    max-width: 420px;
    flex: 1 1 auto;
    align-items: center;
    gap: 8px;
    padding: 0 8px 0 11px;
    border: 1px solid var(--line-strong);
    border-radius: 999px;
    background: color-mix(in srgb, var(--surface-2) 60%, transparent);
    color: var(--faint);
    transition:
      border-color var(--motion-fast, 120ms) ease,
      box-shadow var(--motion-fast, 120ms) ease;
  }

  .search-field:focus-within {
    border-color: color-mix(in srgb, var(--accent) 45%, transparent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 8%, transparent);
  }

  .search-field input {
    min-width: 0;
    flex: 1 1 auto;
    padding: 8px 0;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--text);
    font: inherit;
    font-size: 12px;
  }

  .search-field input::placeholder {
    color: var(--faint);
  }

  .search-field input:disabled {
    cursor: default;
    opacity: 0.55;
  }

  .search-field input::-webkit-search-cancel-button {
    display: none;
  }

  .search-clear {
    display: grid;
    width: 20px;
    height: 20px;
    flex: 0 0 20px;
    place-items: center;
    border-radius: 50%;
    color: var(--muted);
    font-size: 15px;
    line-height: 1;
  }

  .search-clear:hover {
    background: var(--surface-3);
    color: var(--text);
  }

  .theme-chip {
    overflow: hidden;
    flex: 0 0 auto;
    max-width: 30%;
    padding: 4px 9px;
    border: 1px solid var(--line);
    border-radius: 999px;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.03em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .content {
    display: flex;
    min-height: 0;
    flex-direction: column;
    overflow: hidden;
  }

  @media (max-width: 820px) {
    .cadence-browse {
      grid-template-columns: 164px minmax(0, 1fr);
    }

    .sidebar {
      padding-inline: 11px;
    }

    .topbar {
      gap: 10px;
      padding-inline: 16px;
    }

    .theme-chip {
      display: none;
    }
  }

  @media (max-width: 760px) {
    .cadence-browse {
      grid-template-columns: 148px minmax(0, 1fr);
    }

    .sidebar {
      padding: 14px 8px 12px;
    }

    .nav-item {
      padding: 8px;
    }

    .wordmark span {
      display: none;
    }
  }
</style>
