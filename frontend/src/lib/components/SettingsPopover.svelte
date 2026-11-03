<script lang="ts">
  import {
    themes,
    visualizerModes,
    type MotionPreference,
    type ThemeId,
    type VisualizerMode,
  } from "../preferences.svelte";
  import Icon from "./Icon.svelte";

  interface Props {
    theme: ThemeId;
    visualizer: VisualizerMode;
    motion: MotionPreference;
    systemReducedMotion: boolean;
    onTheme: (theme: ThemeId) => void;
    onVisualizer: (mode: VisualizerMode) => void;
    onMotion: (motion: MotionPreference) => void;
    onClose: () => void;
  }

  let {
    theme,
    visualizer,
    motion,
    systemReducedMotion,
    onTheme,
    onVisualizer,
    onMotion,
    onClose,
  }: Props = $props();

  let activeTheme = $derived(themes.find((option) => option.id === theme) ?? themes[0]);
  let activeMode = $derived(
    visualizerModes.find((option) => option.id === visualizer) ?? visualizerModes[0],
  );
  let motionHint = $derived(
    motion === "system" && systemReducedMotion
      ? "System requests reduced motion."
      : "Simulated visualizer · audio stays in Spotify.",
  );
</script>

<div class="settings" role="dialog" aria-modal="true" aria-label="Appearance settings">
  <header class="settings-head">
    <div class="settings-title">
      <span class="eyebrow">Clarity</span>
      <h2>Appearance</h2>
    </div>
    <button class="close" type="button" aria-label="Close appearance settings" onclick={onClose}>
      <Icon name="close" size={12} />
    </button>
  </header>

  <div class="section-line">
    <span class="section-label">
      <Icon name="palette" size={10} />
      Theme
    </span>
    <span class="section-value mono">{activeTheme.label}</span>
  </div>

  <div class="swatches" role="group" aria-label="Color theme">
    {#each themes as option (option.id)}
      <button
        class="swatch"
        class:is-active={theme === option.id}
        type="button"
        style={`--swatch-a: ${option.accent}; --swatch-b: ${option.secondary}`}
        aria-label={`${option.label} theme`}
        aria-pressed={theme === option.id}
        title={option.label}
        onclick={() => onTheme(option.id)}
      >
        {#if theme === option.id}
          <Icon name="check" size={9} strokeWidth={2.2} />
        {/if}
      </button>
    {/each}
  </div>

  <div class="section-line">
    <span class="section-label">
      <Icon name="visualizer" size={10} />
      Visualizer
    </span>
    <span class="section-value mono">{activeMode.label}</span>
  </div>

  <div class="modes" role="group" aria-label="Visualizer mode">
    {#each visualizerModes as option (option.id)}
      <button
        class="mode"
        class:is-active={visualizer === option.id}
        type="button"
        aria-label={`${option.label} visualizer`}
        aria-pressed={visualizer === option.id}
        title={option.label}
        onclick={() => onVisualizer(option.id)}
      >
        <span class="mode-glyph" aria-hidden="true">{option.glyph}</span>
        <span>{option.label}</span>
      </button>
    {/each}
  </div>

  <div class="motion-row">
    <span class="section-label">
      <Icon name="motion" size={10} />
      Motion
    </span>
    <div class="motion-options" role="group" aria-label="Motion preference">
      <button
        class="motion-option"
        class:is-active={motion === "system"}
        type="button"
        aria-pressed={motion === "system"}
        title="Follow the system preference"
        onclick={() => onMotion("system")}
      >
        Auto
      </button>
      <button
        class="motion-option"
        class:is-active={motion === "reduced"}
        type="button"
        aria-pressed={motion === "reduced"}
        title="Reduce pulses, sweeps and particles"
        onclick={() => onMotion("reduced")}
      >
        Reduced
      </button>
    </div>
  </div>

  <p class="hint">{motionHint}</p>
</div>

<style>
  .settings {
    width: 252px;
    padding: 9px 11px 8px;
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

  .settings-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding-bottom: 6px;
  }

  .settings-title {
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
    display: grid;
    width: 19px;
    height: 19px;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--faint);
    transition:
      color 120ms ease,
      background-color 120ms ease;
  }

  .close:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .section-line {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 5px;
    margin-bottom: 4px;
  }

  .section-label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--muted);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    letter-spacing: 0.13em;
    text-transform: uppercase;
  }

  .section-value {
    color: var(--faint);
    font-size: 6.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .swatches {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .swatch {
    position: relative;
    display: grid;
    width: 20px;
    height: 20px;
    place-items: center;
    border: 1px solid rgb(255 255 255 / 13%);
    border-radius: 50%;
    background:
      radial-gradient(circle at 34% 28%, rgb(255 255 255 / 42%), transparent 42%),
      linear-gradient(140deg, var(--swatch-a), var(--swatch-b));
    color: rgb(8 10 14 / 78%);
    box-shadow: 0 2px 6px rgb(0 0 0 / 32%);
    transition:
      transform 120ms ease,
      box-shadow 120ms ease;
  }

  .swatch:hover {
    transform: translateY(-1px) scale(1.05);
  }

  .swatch.is-active {
    box-shadow:
      0 0 0 1.5px var(--bg),
      0 0 0 2.5px var(--swatch-a),
      0 0 12px color-mix(in srgb, var(--swatch-a) 38%, transparent);
  }

  .modes {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 4px;
  }

  .mode {
    display: flex;
    height: 22px;
    min-width: 0;
    align-items: center;
    justify-content: center;
    gap: 3px;
    padding: 0 3px;
    border: 1px solid var(--line);
    border-radius: 4px;
    color: var(--faint);
    font-size: 7.5px;
    font-weight: 500;
    transition:
      color 120ms ease,
      border-color 120ms ease,
      background-color 120ms ease;
  }

  .mode span:last-child {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mode-glyph {
    color: color-mix(in srgb, var(--accent) 65%, var(--muted));
    font-family: "JetBrains Mono", monospace;
    font-size: 8px;
    line-height: 1;
  }

  .mode:hover {
    border-color: var(--line-strong);
    background: var(--surface-2);
    color: var(--text);
  }

  .mode.is-active {
    border-color: color-mix(in srgb, var(--accent) 34%, transparent);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .motion-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 7px;
  }

  .motion-options {
    display: flex;
    padding: 1px;
    border: 1px solid var(--line);
    border-radius: 4px;
    background: color-mix(in srgb, var(--bg) 48%, transparent);
  }

  .motion-option {
    height: 17px;
    padding: 0 6px;
    border-radius: 3px;
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 6px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    transition:
      color 120ms ease,
      background-color 120ms ease;
  }

  .motion-option:hover {
    color: var(--text);
  }

  .motion-option.is-active {
    background: var(--surface-3);
    color: var(--text);
  }

  .hint {
    margin: 6px 0 0;
    padding-top: 5px;
    border-top: 1px solid var(--line);
    color: var(--faint);
    font-size: 7.5px;
    line-height: 1.3;
  }
</style>
