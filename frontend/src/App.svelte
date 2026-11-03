<script lang="ts">
  import { onMount } from "svelte";
  import { Window } from "@wailsio/runtime";
  import { WindowService } from "../bindings/cadence/internal/platform";
  import DevicePopover from "./lib/components/DevicePopover.svelte";
  import Icon from "./lib/components/Icon.svelte";
  import Player from "./lib/components/Player.svelte";
  import Queue from "./lib/components/Queue.svelte";
  import SettingsPopover from "./lib/components/SettingsPopover.svelte";
  import TitleBar from "./lib/components/TitleBar.svelte";
  import { player } from "./lib/player.svelte";
  import { preferences, themes, visualizerModes, type ThemeId } from "./lib/preferences.svelte";
  import SkinHost from "./lib/skins/SkinHost.svelte";
  import SkinManager from "./lib/skins/SkinManager.svelte";
  import { skins } from "./lib/skins.svelte";
  import MusicBrowser from "./lib/MusicBrowser.svelte";
  import { music } from "./lib/music.svelte";

  let queueClosing = $state(false);
  let settingsOpen = $state(false);
  let devicesOpen = $state(false);
  let skinManagerOpen = $state(false);
  let musicOpen = $state(false);
  let resizeTimer: ReturnType<typeof setTimeout> | null = null;
  let lastVolume = 65;

  let queueOpen = $derived(skins.queueOpen);
  let declarative = $derived(skins.isDeclarative);
  let baseHeight = $derived(declarative ? (skins.runtime?.baseHeight ?? 220) : 220);
  let expandedHeight = $derived(declarative ? (skins.runtime?.queueHeight ?? 460) : 460);

  let activeTheme = $derived(themes.find((theme) => theme.id === preferences.theme) ?? themes[0]);
  let activeModeLabel = $derived(
    visualizerModes.find((mode) => mode.id === preferences.visualizer)?.label ?? "Spectrum",
  );
  let displayError = $derived(player.error || skins.error);

  function resizeWindow(height: number): void {
    void WindowService.SetSize(420, height).catch(() => {});
  }

  function openSettings(): void {
    if (declarative) {
      openSkinManager();
      return;
    }
    devicesOpen = false;
    skinManagerOpen = false;
    musicOpen = false;
    settingsOpen = true;
  }

  function openSkinManager(): void {
    devicesOpen = false;
    settingsOpen = false;
    musicOpen = false;
    if (queueOpen) setQueue(false);
    skinManagerOpen = true;
  }

  function closeSkinManager(): void {
    skinManagerOpen = false;
  }

  function openMusic(): void {
    devicesOpen = false;
    settingsOpen = false;
    skinManagerOpen = false;
    if (queueOpen) setQueue(false);
    musicOpen = true;
  }

  function closeMusic(): void {
    musicOpen = false;
    music.clearContext();
  }

  function openDevices(): void {
    settingsOpen = false;
    skinManagerOpen = false;
    musicOpen = false;
    devicesOpen = true;
  }

  function closePopovers(): void {
    settingsOpen = false;
    devicesOpen = false;
    skinManagerOpen = false;
    musicOpen = false;
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
      resizeWindow(expandedHeight);
      return;
    }

    if (!queueOpen || queueClosing) return;
    queueClosing = true;
    closePopovers();
    resizeTimer = setTimeout(() => {
      skins.queueOpen = false;
      queueClosing = false;
      resizeTimer = null;
      resizeWindow(baseHeight);
    }, 180);
  }

  function toggleQueue(): void {
    setQueue(!queueOpen);
  }

  function minimise(): void {
    void Window.Minimise().catch(() => {});
  }

  function closeWindow(): void {
    void Window.Close().catch(() => {});
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
    if (event.metaKey || event.ctrlKey || event.altKey) return;

    if (event.key === "Escape") {
      if (skinManagerOpen) {
        skinManagerOpen = false;
      } else if (musicOpen) {
        closeMusic();
      } else if (settingsOpen || devicesOpen) {
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
          void player.seek(player.state.progressMs - 5_000);
        }
        break;
      case "ArrowRight":
        event.preventDefault();
        if (event.shiftKey) {
          void player.next();
        } else if (player.state.canSeek) {
          void player.seek(player.state.progressMs + 5_000);
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
  }

  $effect(() => {
    if (skins.loading) return;
    resizeWindow(musicOpen ? 460 : queueOpen ? expandedHeight : baseHeight);
  });

  $effect(() => {
    if (declarative && !skins.hasQueue && skins.queueOpen) {
      skins.queueOpen = false;
    }
  });

  onMount(() => {
    skins.setHandlers({
      toggleQueue,
      openDevices,
      openSettings,
      openMusic,
    });
    void player.start();
    void skins.start();
    void music.start();
    const cleanupMotion = preferences.syncSystemMotion();
    return () => {
      cleanupMotion();
      player.stop();
      skins.stop();
      music.stop();
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
          provider={player.state.provider}
          deviceName={player.state.device.name}
          {queueOpen}
          onToggleQueue={toggleQueue}
          onOpenDevices={openDevices}
          onOpenSettings={openSettings}
          onOpenMusic={openMusic}
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
            <Queue items={player.state.queue ?? []} provider={player.state.provider} />
          </div>
        {/if}

        {#if settingsOpen || devicesOpen}
          <div class="popover-layer">
            <button
              class="popover-backdrop"
              type="button"
              aria-label="Close popover"
              tabindex="-1"
              onclick={closePopovers}
            ></button>
            <div class="popover-position">
              {#if settingsOpen}
                <SettingsPopover
                  theme={preferences.theme}
                  visualizer={preferences.visualizer}
                  motion={preferences.motion}
                  systemReducedMotion={preferences.systemReducedMotion}
                  onTheme={(theme) => preferences.setTheme(theme)}
                  onVisualizer={(mode) => preferences.setVisualizer(mode)}
                  onMotion={(motion) => preferences.setMotion(motion)}
                  onOpenSkins={openSkinManager}
                  onClose={closePopovers}
                />
              {:else}
                <DevicePopover
                  devices={player.state.devices ?? []}
                  onTransfer={(deviceId) => {
                    closePopovers();
                    void player.transfer(deviceId);
                  }}
                  onClose={closePopovers}
                />
              {/if}
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#if devicesOpen && declarative}
    <div class="clarity overlay-layer" data-theme={preferences.theme}>
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

  {#if skinManagerOpen}
    <div class="clarity overlay-layer" data-theme={preferences.theme}>
      <SkinManager onOpenMusic={openMusic} onClose={closeSkinManager} />
    </div>
  {/if}

  {#if musicOpen}
    <div class="clarity overlay-layer" data-theme={preferences.theme}>
      <MusicBrowser onClose={closeMusic} />
    </div>
  {/if}

  {#if displayError}
    <div class="clarity toast-layer" data-theme={preferences.theme}>
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
