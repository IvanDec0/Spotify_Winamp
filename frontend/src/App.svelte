<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { appWindow, screens } from "./shared/api";
  import DevicePopover from "./lib/components/DevicePopover.svelte";
  import Icon from "./lib/components/Icon.svelte";
  import Player from "./lib/components/Player.svelte";
  import Queue from "./lib/components/Queue.svelte";
  import TitleBar from "./lib/components/TitleBar.svelte";
  import { player } from "./lib/player.svelte";
  import { preferences, themes, visualizerModes, type ThemeId } from "./lib/preferences.svelte";
  import SkinHost from "./lib/skins/SkinHost.svelte";
  import { skins } from "./lib/skins.svelte";
  import {
    DEFAULT_UI_SCALE,
    EXPANDED_HEIGHT,
    MIN_UI_SCALE,
    WINDOW_HEIGHT,
    WINDOW_WIDTH,
    fitUiScale,
    roundedUiScale,
  } from "./lib/layout";

  let queueClosing = $state(false);
  let devicesOpen = $state(false);
  let browseOpenError = $state<string | null>(null);
  let uiScale = $state(DEFAULT_UI_SCALE);
  let baseNativeWidth = Math.round(WINDOW_WIDTH * DEFAULT_UI_SCALE);
  let baseNativeHeight = Math.round(WINDOW_HEIGHT * DEFAULT_UI_SCALE);
  let resizeTimer: ReturnType<typeof setTimeout> | null = null;
  let scaleSyncTimer: ReturnType<typeof setTimeout> | null = null;
  let syncingScale = false;
  let lastVolume = 65;
  let lastSkinWindow = "";

  let queueOpen = $derived(skins.queueOpen);
  let queueTrackUri = $derived(player.state.track.uri);
  let declarative = $derived(skins.isDeclarative);
  let baseHeight = $derived(
    declarative ? (skins.runtime?.baseHeight ?? WINDOW_HEIGHT) : WINDOW_HEIGHT,
  );
  let expandedHeight = $derived(
    declarative ? (skins.runtime?.queueHeight ?? EXPANDED_HEIGHT) : EXPANDED_HEIGHT,
  );

  let activeTheme = $derived(themes.find((theme) => theme.id === preferences.theme) ?? themes[0]);
  let activeModeLabel = $derived(
    visualizerModes.find((mode) => mode.id === preferences.visualizer)?.label ?? "Spectrum",
  );
  let displayError = $derived(player.error || skins.error || browseOpenError);
  let overlayThemeStyle = $derived(
    skins.isDeclarative
      ? Object.entries(skins.palette)
          .map(([name, value]) => `${name}:${value}`)
          .join(";")
      : "",
  );

  function currentLogicalHeight(): number {
    return queueOpen ? expandedHeight : baseHeight;
  }

  async function syncUiScale(): Promise<void> {
    if (syncingScale) return;
    syncingScale = true;
    const logicalHeight = currentLogicalHeight();
    try {
      const size = await appWindow.size();
      // The window mode may have changed while Size() was resolving.
      if (currentLogicalHeight() !== logicalHeight) return;
      const next = roundedUiScale(fitUiScale(size.width, size.height, logicalHeight));
      if (!queueOpen) {
        baseNativeWidth = size.width;
        baseNativeHeight = size.height;
      }
      uiScale = next;
      const current = await appWindow.getZoom();
      if (Math.abs(current - next) > 0.01) {
        await appWindow.setZoom(next);
      }
    } catch {
      // Keep the last known scale if the native window is not ready.
    } finally {
      syncingScale = false;
    }
  }

  function scheduleUiScaleSync(): void {
    if (scaleSyncTimer) clearTimeout(scaleSyncTimer);
    scaleSyncTimer = setTimeout(() => {
      scaleSyncTimer = null;
      void syncUiScale();
    }, 120);
  }

  async function resizeWindow(height: number, scale: number): Promise<void> {
    let nextScale = roundedUiScale(scale);
    let width = Math.round(WINDOW_WIDTH * nextScale);
    let nativeHeight = Math.round(height * nextScale);

    try {
      const screen = await screens.getCurrent();
      const workArea =
        screen?.PhysicalWorkArea?.Height > 0 ? screen.PhysicalWorkArea : screen?.WorkArea;
      if (workArea && workArea.Height > 0) {
        const maxHeight = Math.max(Math.round(WINDOW_HEIGHT * MIN_UI_SCALE), workArea.Height - 48);
        if (nativeHeight > maxHeight) {
          nextScale = roundedUiScale(maxHeight / height);
          width = Math.round(WINDOW_WIDTH * nextScale);
          nativeHeight = Math.round(height * nextScale);
        }
      }

      await appWindow.setMinSize(
        Math.round(WINDOW_WIDTH * MIN_UI_SCALE),
        Math.round(height * MIN_UI_SCALE),
      );
      await appWindow.setSize(width, nativeHeight);
    } catch {
      // Ignore transient resize failures while starting or closing.
    }
  }

  async function restoreBaseWindow(): Promise<void> {
    try {
      await appWindow.setMinSize(
        Math.round(WINDOW_WIDTH * MIN_UI_SCALE),
        Math.round(baseHeight * MIN_UI_SCALE),
      );
      await appWindow.setSize(baseNativeWidth, baseNativeHeight);
    } catch {
      // Ignore transient resize failures while starting or closing.
    }
  }

  function openBrowseSection(section: "library" | "settings"): void {
    closePopovers();
    browseOpenError = null;
    void appWindow.openBrowse(section).catch((error: unknown) => {
      browseOpenError = error instanceof Error ? error.message : String(error);
    });
  }

  function openBrowse(): void {
    openBrowseSection("library");
  }

  function openSettings(): void {
    openBrowseSection("settings");
  }

  function openDevices(): void {
    devicesOpen = true;
  }

  function closePopovers(): void {
    devicesOpen = false;
  }

  function setQueue(open: boolean): void {
    if (declarative && !skins.hasQueue) return;

    if (open) {
      if (resizeTimer) {
        clearTimeout(resizeTimer);
        resizeTimer = null;
      }
      queueClosing = false;
      if (!queueOpen) skins.queueOpen = true;
      closePopovers();
      return;
    }

    if (!queueOpen || queueClosing) return;
    queueClosing = true;
    closePopovers();
    resizeTimer = setTimeout(() => {
      skins.queueOpen = false;
      queueClosing = false;
      resizeTimer = null;
    }, 180);
  }

  function toggleQueue(): void {
    setQueue(!queueOpen);
  }

  function minimise(): void {
    void appWindow.minimise().catch(() => {});
  }

  function closeWindow(): void {
    void appWindow.close().catch(() => {});
  }

  function toggleMute(): void {
    if (player.state.volume > 0) {
      lastVolume = player.state.volume;
      void player.setVolume(0);
    } else {
      void player.setVolume(lastVolume || 65);
    }
  }

  function cycleTheme(): void {
    const index = themes.findIndex((theme) => theme.id === preferences.theme);
    const next = themes[(index + 1) % themes.length];
    preferences.setTheme(next.id as ThemeId);
  }

  $effect(() => {
    if (player.state.volume > 0) lastVolume = player.state.volume;
  });

  function isInteractiveTarget(event: KeyboardEvent): boolean {
    for (const node of event.composedPath()) {
      if (!(node instanceof Element)) continue;
      if (
        node.matches(
          "input, textarea, select, button, a, [contenteditable='true'], [role='slider']",
        )
      ) {
        return true;
      }
    }
    return false;
  }

  function handleKeydown(event: KeyboardEvent): void {
    if (event.defaultPrevented) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;

    if (event.key === "Escape") {
      if (devicesOpen) {
        closePopovers();
      } else if (queueOpen) {
        setQueue(false);
      }
      return;
    }

    if (isInteractiveTarget(event)) return;

    switch (event.key) {
      case " ":
      case "k":
        event.preventDefault();
        if (player.state.canControl) void player.toggle();
        break;
      case "ArrowLeft":
        event.preventDefault();
        if (event.shiftKey) {
          void player.previous();
        } else if (player.state.canSeek) {
          void player.seek(player.positionMs - 5_000);
        }
        break;
      case "ArrowRight":
        event.preventDefault();
        if (event.shiftKey) {
          void player.next();
        } else if (player.state.canSeek) {
          void player.seek(player.positionMs + 5_000);
        }
        break;
      case "ArrowUp":
        event.preventDefault();
        if (player.state.canControl) void player.setVolume(Math.min(100, player.state.volume + 5));
        break;
      case "ArrowDown":
        event.preventDefault();
        if (player.state.canControl) void player.setVolume(Math.max(0, player.state.volume - 5));
        break;
      case "m":
        if (player.state.canControl) toggleMute();
        break;
      case "s":
        if (player.state.canControl) void player.toggleShuffle();
        break;
      case "r":
        if (player.state.canControl) void player.cycleRepeat();
        break;
      case "q":
        event.preventDefault();
        setQueue(!queueOpen);
        break;
      case "v":
        if (declarative) skins.cycleVisualizer();
        else preferences.cycleVisualizer();
        break;
      case "t":
        if (declarative) void skins.cycleTheme();
        else cycleTheme();
        break;
      default:
        break;
    }
  }

  function dismissError(): void {
    if (player.error) player.dismissError();
    if (skins.error) skins.dismissError();
    browseOpenError = null;
  }

  $effect(() => {
    if (skins.loading) return;
    const id = skins.runtime?.id ?? "clarity";
    const height = baseHeight;
    const windowKey = `${id}:${height}`;
    if (windowKey === lastSkinWindow) return;
    lastSkinWindow = windowKey;
    const scale = Math.max(
      DEFAULT_UI_SCALE,
      untrack(() => uiScale),
    );
    baseNativeWidth = Math.round(WINDOW_WIDTH * scale);
    baseNativeHeight = Math.round(height * scale);
    if (queueOpen) return;
    void restoreBaseWindow();
  });

  $effect(() => {
    if (skins.loading) return;
    if (!queueOpen) {
      void restoreBaseWindow();
      return;
    }
    const scale = untrack(() => uiScale);
    void resizeWindow(expandedHeight, scale);
  });

  $effect(() => {
    if (declarative && !skins.hasQueue && skins.queueOpen) {
      skins.queueOpen = false;
    }
  });

  $effect(() => {
    if (!queueOpen) return;
    if (!queueTrackUri) return;
    void player.loadQueue();
  });

  onMount(() => {
    skins.setHandlers({
      toggleQueue,
      openDevices,
      openSettings,
      openMusic: openBrowse,
    });
    void player.start();
    void skins.start();
    const cleanupMotion = preferences.syncSystemMotion();
    const cleanupPreferenceStorage = preferences.startStorageSync();
    globalThis.addEventListener("resize", scheduleUiScaleSync);
    void syncUiScale();
    return () => {
      cleanupMotion();
      cleanupPreferenceStorage();
      globalThis.removeEventListener("resize", scheduleUiScaleSync);
      if (scaleSyncTimer) clearTimeout(scaleSyncTimer);
      player.stop();
      skins.stop();
      skins.setHandlers(null);
      if (resizeTimer) clearTimeout(resizeTimer);
    };
  });
</script>

<svelte:head>
  <title>Cadence</title>
</svelte:head>

<svelte:window onkeydown={handleKeydown} />

<div class="app-frame">
  {#if declarative && skins.runtime}
    <SkinHost runtime={skins.runtime} theme={skins.theme} {queueOpen} />
  {:else}
    <div
      class="clarity"
      class:reduce-motion={preferences.reducedMotion}
      data-theme={preferences.theme}
    >
      <div class="shell">
        <TitleBar
          deviceName={player.state.device.name}
          {queueOpen}
          onToggleQueue={toggleQueue}
          onOpenDevices={openDevices}
          onOpenSettings={openSettings}
          onOpenMusic={openBrowse}
          onMinimise={minimise}
          onClose={closeWindow}
        />

        <main class="player-region" aria-label="Cadence player">
          <Player
            state={player.state}
            visualizerMode={preferences.visualizer}
            visualizerLabel={activeModeLabel}
            accent={activeTheme.accent}
            secondary={activeTheme.secondary}
            reducedMotion={preferences.reducedMotion}
            onToggle={() => void player.toggle()}
            onPrevious={() => void player.previous()}
            onNext={() => void player.next()}
            onSeek={(positionMs) => void player.seek(positionMs)}
            onVolume={(volume) => void player.setVolume(volume)}
            onShuffle={() => void player.toggleShuffle()}
            onRepeat={() => void player.cycleRepeat()}
            onCycleVisualizer={() => preferences.cycleVisualizer()}
          />
        </main>

        {#if queueOpen}
          <div class="queue-slot" class:is-closing={queueClosing}>
            <Queue
              items={player.queue}
              provider={player.state.provider}
              loading={player.queueLoading}
            />
          </div>
        {/if}

        {#if devicesOpen}
          <div class="popover-layer">
            <button
              class="popover-backdrop"
              type="button"
              aria-label="Close popover"
              tabindex="-1"
              onclick={closePopovers}
            ></button>
            <div class="popover-position">
              <DevicePopover
                devices={player.state.devices ?? []}
                onTransfer={(deviceId) => {
                  closePopovers();
                  void player.transfer(deviceId);
                }}
                onClose={closePopovers}
              />
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#if devicesOpen && declarative}
    <div
      class="clarity overlay-layer"
      class:reduce-motion={preferences.reducedMotion}
      data-theme={preferences.theme}
      style={overlayThemeStyle}
    >
      <div class="popover-layer">
        <button
          class="popover-backdrop"
          type="button"
          aria-label="Close popover"
          tabindex="-1"
          onclick={closePopovers}
        ></button>
        <div class="popover-position">
          <DevicePopover
            devices={player.state.devices ?? []}
            onTransfer={(deviceId) => {
              closePopovers();
              void player.transfer(deviceId);
            }}
            onClose={closePopovers}
          />
        </div>
      </div>
    </div>
  {/if}

  {#if displayError}
    <div
      class="clarity toast-layer"
      class:reduce-motion={preferences.reducedMotion}
      data-theme={preferences.theme}
      style={overlayThemeStyle}
    >
      <div class="toast" role="alert">
        <span class="toast-icon" aria-hidden="true">
          <Icon name="info" size={13} />
        </span>
        <span class="toast-message">{displayError}</span>
        <button class="toast-close" type="button" aria-label="Dismiss error" onclick={dismissError}>
          <Icon name="close" size={11} />
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .shell {
    position: relative;
    isolation: isolate;
  }

  .player-region {
    display: flex;
    flex: 1 1 auto;
    min-height: 0;
  }

  .queue-slot {
    flex: 0 0 240px;
    min-height: 0;
    overflow: hidden;
    animation: queue-in 220ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .queue-slot.is-closing {
    animation: queue-out 180ms ease-in forwards;
  }

  .popover-layer {
    position: absolute;
    z-index: 50;
    inset: 0;
    pointer-events: none;
  }

  .popover-backdrop {
    position: absolute;
    inset: 0;
    border: 0;
    background: rgb(0 0 0 / 16%);
    cursor: default;
    pointer-events: auto;
    backdrop-filter: blur(0.8px);
  }

  .popover-backdrop:focus-visible {
    outline: none;
  }

  .popover-position {
    position: absolute;
    top: 35px;
    right: 10px;
    z-index: 1;
    animation: pop-in 160ms cubic-bezier(0.22, 1, 0.36, 1);
    pointer-events: auto;
  }

  .toast {
    position: absolute;
    z-index: 60;
    right: 12px;
    bottom: 10px;
    left: 12px;
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 7px 8px 7px 10px;
    border: 1px solid color-mix(in srgb, var(--danger) 34%, transparent);
    border-radius: 6px;
    background: color-mix(in srgb, var(--surface) 92%, var(--danger));
    box-shadow: 0 10px 28px rgb(0 0 0 / 48%);
    animation: toast-in 180ms ease-out;
  }

  .toast-icon {
    display: grid;
    place-items: center;
    color: var(--danger);
  }

  .toast-message {
    overflow: hidden;
    flex: 1 1 auto;
    color: color-mix(in srgb, var(--text) 88%, var(--danger));
    font-size: 9.5px;
    line-height: 1.3;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .toast-close {
    display: grid;
    width: 18px;
    height: 18px;
    flex: 0 0 18px;
    place-items: center;
    border-radius: 3px;
    color: var(--faint);
  }

  .toast-close:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  @keyframes queue-in {
    from {
      opacity: 0;
      transform: translateY(-10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @keyframes queue-out {
    from {
      opacity: 1;
      transform: translateY(0);
    }
    to {
      opacity: 0;
      transform: translateY(-8px);
    }
  }

  @keyframes pop-in {
    from {
      opacity: 0;
      transform: translateY(-4px) scale(0.985);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateY(5px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
