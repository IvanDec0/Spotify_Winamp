<script lang="ts">
  import { Window } from "@wailsio/runtime";

  const appWindow = Window;
  const appVersion = "0.1.0";

  function minimise(): void {
    void appWindow.Minimise();
  }

  function close(): void {
    void appWindow.Close();
  }
</script>

<svelte:head>
  <title>Cadence</title>
</svelte:head>

<div class="shell">
  <header class="titlebar">
    <div class="identity">
      <svg class="brand-mark" viewBox="0 0 24 24" aria-hidden="true">
        <path
          d="M16.4 7.7A6 6 0 1 0 16.4 16.3"
          fill="none"
          stroke="currentColor"
          stroke-width="2.1"
          stroke-linecap="round"
        />
        <circle cx="16.4" cy="7.7" r="1.3" fill="currentColor" />
      </svg>
      <span class="brand-name">CADENCE</span>
      <span class="badge">DEMO</span>
    </div>

    <div class="window-actions">
      <button class="window-button" type="button" aria-label="Minimise window" onclick={minimise}>
        <svg viewBox="0 0 12 12" aria-hidden="true">
          <path d="M2 6h8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
        </svg>
      </button>
      <button class="window-button close" type="button" aria-label="Close window" onclick={close}>
        <svg viewBox="0 0 12 12" aria-hidden="true">
          <path
            d="m2.5 2.5 7 7m0-7-7 7"
            stroke="currentColor"
            stroke-width="1.2"
            stroke-linecap="round"
          />
        </svg>
      </button>
    </div>
  </header>

  <main class="stage">
    <div class="intro">
      <div class="signal" aria-hidden="true">
        <span style="--i: 0; --level: 0.35"></span>
        <span style="--i: 1; --level: 0.75"></span>
        <span style="--i: 2; --level: 1"></span>
        <span style="--i: 3; --level: 0.55"></span>
        <span style="--i: 4; --level: 0.2"></span>
      </div>

      <div class="copy">
        <p class="eyebrow">Spotify Connect controller</p>
        <h1>A quieter way to control playback.</h1>
        <p class="muted">The demo player and Clarity skin arrive in the next milestone.</p>
      </div>
    </div>

    <footer class="status">
      <span class="dot" aria-hidden="true"></span>
      <span>Scaffold ready</span>
      <span class="version">v{appVersion}</span>
    </footer>
  </main>
</div>
