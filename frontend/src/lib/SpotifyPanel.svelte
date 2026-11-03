<script lang="ts">
  import { AuthStatus } from "../../bindings/cadence/internal/spotify";
  import Icon from "./components/Icon.svelte";
  import { spotify } from "./spotify.svelte";

  interface Props {
    onClose: () => void;
  }

  let { onClose }: Props = $props();

  let clientID = $state(spotify.auth.clientId);
  let dirty = $state(false);
  let saving = $state(false);
  let copied = $state(false);

  let auth = $derived(spotify.auth);
  let statusLabel = $derived.by(() => {
    switch (auth.status) {
      case AuthStatus.AuthChecking:
        return "Checking Spotify session…";
      case AuthStatus.AuthAuthenticating:
        return "Waiting for authorization in your browser…";
      case AuthStatus.AuthAuthenticated:
        return auth.playerMode === "spotify"
          ? "Connected to Spotify"
          : "Connected, using demo player";
      case AuthStatus.AuthError:
        return "Spotify needs attention";
      default:
        return "Spotify is not connected";
    }
  });

  $effect(() => {
    const value = spotify.auth.clientId;
    if (!dirty) clientID = value;
  });

  function markDirty(): void {
    dirty = true;
  }

  async function saveClientID(): Promise<void> {
    saving = true;
    await spotify.setClientID(clientID.trim());
    saving = false;
    if (!spotify.error) dirty = false;
  }

  async function connect(): Promise<void> {
    const trimmed = clientID.trim();
    if (!trimmed) return;
    if (trimmed !== spotify.auth.clientId) {
      saving = true;
      await spotify.setClientID(trimmed);
      saving = false;
      dirty = false;
      if (spotify.error) return;
    }
    await spotify.login();
  }

  async function copyRedirect(): Promise<void> {
    try {
      await navigator.clipboard.writeText(auth.redirectUri);
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 1600);
    } catch {
      copied = false;
    }
  }
</script>

<div class="spotify-panel" role="dialog" aria-modal="true" aria-label="Spotify account">
  <header class="panel-head">
    <div class="panel-title">
      <span class="eyebrow">Cadence</span>
      <h2>Spotify</h2>
      <span
        class="status-chip"
        class:is-connected={spotify.isAuthenticated}
        class:is-error={auth.status === AuthStatus.AuthError}
      >
        <span class="status-dot"></span>
        {statusLabel}
      </span>
    </div>
    <button class="close" type="button" aria-label="Close Spotify account" onclick={onClose}>
      <Icon name="close" size={12} />
    </button>
  </header>

  <div class="panel-body">
    {#if auth.status === AuthStatus.AuthChecking}
      <p class="banner info">Checking the OS keyring and the saved Spotify session.</p>
    {/if}
    {#if auth.status === AuthStatus.AuthAuthenticating}
      <p class="banner info">
        Complete the authorization in your browser. This panel updates automatically.
      </p>
    {/if}
    {#if !auth.keyringAvailable}
      <p class="banner error">
        The OS keyring is unavailable. Start a Secret Service provider such as GNOME Keyring so
        Cadence can store the refresh token securely.
      </p>
    {/if}
    {#if auth.error}
      <p class="banner error">{auth.error}</p>
    {/if}
    {#if spotify.error}
      <div class="banner error banner-row">
        <span>{spotify.error}</span>
        <button type="button" onclick={() => spotify.dismissError()}>Dismiss</button>
      </div>
    {/if}

    <section class="account-card" aria-label="Spotify connection">
      <div class="account-row">
        <span class="account-label">Account</span>
        <strong>{spotify.isAuthenticated ? "Spotify authorized" : "Not authorized"}</strong>
      </div>
      <div class="account-row">
        <span class="account-label">Playback source</span>
        <strong>{auth.playerMode === "spotify" ? "Spotify Connect" : "Demo player"}</strong>
      </div>
      <div class="account-row redirect-row">
        <span class="account-label">Redirect URI</span>
        <code>{auth.redirectUri}</code>
        <button class="mini-action" type="button" onclick={() => void copyRedirect()}>
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
    </section>

    <label class="field">
      <span class="field-label">Spotify Client ID</span>
      <input
        type="text"
        spellcheck="false"
        autocomplete="off"
        placeholder="Paste the Client ID from the Spotify Developer Dashboard"
        value={clientID}
        disabled={auth.status === AuthStatus.AuthAuthenticating}
        oninput={(event) => {
          clientID = (event.currentTarget as HTMLInputElement).value;
          markDirty();
        }}
      />
    </label>

    <div class="primary-row">
      {#if spotify.isAuthenticated}
        {#if auth.playerMode === "spotify"}
          <button class="primary" type="button" onclick={() => void spotify.useDemo()}>
            Use demo player
          </button>
        {:else}
          <button class="primary" type="button" onclick={() => void spotify.useSpotify()}>
            Use Spotify Connect
          </button>
        {/if}
        <button class="secondary" type="button" onclick={() => void spotify.logout()}>
          Log out
        </button>
        {#if dirty}
          <button
            class="secondary"
            type="button"
            disabled={saving || !clientID.trim()}
            onclick={() => void saveClientID()}
          >
            Save Client ID
          </button>
        {/if}
      {:else if auth.status === AuthStatus.AuthAuthenticating}
        <button class="secondary" type="button" onclick={() => void spotify.logout()}>
          Cancel authorization
        </button>
      {:else}
        <button
          class="primary"
          type="button"
          disabled={!clientID.trim() || saving || !auth.keyringAvailable}
          onclick={() => void connect()}
        >
          {saving
            ? "Saving…"
            : auth.status === AuthStatus.AuthError
              ? "Reconnect Spotify"
              : "Connect Spotify"}
        </button>
        <button
          class="secondary"
          type="button"
          disabled={!dirty || !clientID.trim() || saving}
          onclick={() => void saveClientID()}
        >
          Save Client ID
        </button>
      {/if}
    </div>

    <p class="hint">
      Register the exact redirect URI above in your Spotify Developer Dashboard before connecting.
      Cadence uses Authorization Code + PKCE, never a client secret, and never sends tokens to the
      interface.
    </p>
    <p class="hint privacy">
      The refresh token is stored in the OS keyring. Access tokens stay in Go memory. Cadence never
      receives audio or your Spotify password.
    </p>
  </div>
</div>

<style>
  .spotify-panel {
    display: flex;
    height: 100%;
    flex-direction: column;
    color: var(--text);
    background: var(--bg);
    font-family: "Inter", "Segoe UI", system-ui, sans-serif;
  }

  .panel-head {
    display: flex;
    min-height: 34px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px 6px 13px;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--bg) 80%, transparent);
  }

  .panel-title {
    display: flex;
    min-width: 0;
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

  .status-chip {
    display: inline-flex;
    min-width: 0;
    align-items: center;
    gap: 4px;
    overflow: hidden;
    color: var(--faint);
    font-size: 7.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status-dot {
    width: 5px;
    height: 5px;
    flex: 0 0 5px;
    border-radius: 50%;
    background: var(--faint);
  }

  .status-chip.is-connected {
    color: var(--accent);
  }

  .status-chip.is-connected .status-dot {
    background: var(--accent);
    box-shadow: 0 0 6px var(--accent);
  }

  .status-chip.is-error {
    color: var(--danger);
  }

  .status-chip.is-error .status-dot {
    background: var(--danger);
  }

  .close {
    display: grid;
    width: 22px;
    height: 22px;
    flex: 0 0 22px;
    place-items: center;
    border-radius: 4px;
    color: var(--faint);
  }

  .close:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .panel-body {
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

  .banner.info {
    border-color: color-mix(in srgb, var(--accent) 26%, transparent);
    background: color-mix(in srgb, var(--accent) 6%, transparent);
    color: var(--text);
  }

  .banner.error {
    border-color: color-mix(in srgb, var(--danger) 35%, transparent);
    background: color-mix(in srgb, var(--danger) 7%, transparent);
    color: color-mix(in srgb, var(--danger) 80%, var(--text));
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

  .account-card {
    margin-bottom: 8px;
    padding: 7px 8px 6px;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background: color-mix(in srgb, var(--surface) 80%, transparent);
  }

  .account-row {
    display: flex;
    min-height: 18px;
    align-items: center;
    gap: 6px;
    font-size: 8px;
  }

  .account-row strong {
    overflow: hidden;
    color: var(--text);
    font-weight: 550;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .account-label {
    width: 74px;
    flex: 0 0 74px;
    color: var(--faint);
    font-size: 7px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .redirect-row code {
    overflow: hidden;
    flex: 1 1 auto;
    color: var(--muted);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
    -webkit-user-select: text;
    user-select: text;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin-bottom: 8px;
  }

  .field-label {
    color: var(--faint);
    font-size: 7px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .field input {
    width: 100%;
    height: 24px;
    padding: 0 7px;
    border: 1px solid var(--line-strong);
    border-radius: 4px;
    outline: none;
    background: var(--surface);
    color: var(--text);
    font-family: "JetBrains Mono", monospace;
    font-size: 8px;
    -webkit-user-select: text;
    user-select: text;
  }

  .field input:focus {
    border-color: color-mix(in srgb, var(--accent) 45%, transparent);
  }

  .field input::placeholder {
    color: var(--faint);
  }

  .primary-row {
    display: flex;
    gap: 5px;
  }

  .primary,
  .secondary,
  .mini-action {
    border: 1px solid var(--line-strong);
    border-radius: 4px;
    color: var(--text);
    transition:
      border-color 120ms ease,
      background-color 120ms ease;
  }

  .primary {
    flex: 1 1 auto;
    padding: 5px 7px;
    border-color: color-mix(in srgb, var(--accent) 35%, transparent);
    background: color-mix(in srgb, var(--accent) 13%, var(--surface-2));
    font-size: 8px;
    font-weight: 550;
  }

  .secondary {
    flex: 0 0 auto;
    padding: 5px 7px;
    background: var(--surface-2);
    color: var(--muted);
    font-size: 8px;
  }

  .mini-action {
    flex: 0 0 auto;
    padding: 2px 5px;
    background: var(--surface-2);
    color: var(--muted);
    font-size: 7px;
  }

  .primary:hover:not(:disabled),
  .secondary:hover:not(:disabled),
  .mini-action:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--accent) 45%, transparent);
    color: var(--text);
  }

  .hint {
    margin: 7px 0 0;
    color: var(--faint);
    font-size: 7px;
    line-height: 1.4;
  }

  .hint.privacy {
    margin-top: 4px;
  }
</style>
