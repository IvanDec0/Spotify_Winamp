<script lang="ts">
  import { tick } from "svelte";
  import { music } from "../../lib/music.svelte";
  import { playerErrorCopy } from "../../lib/player-error";
  import { audio, toApiError } from "../../shared/api";
  import { t, type MessageKey } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Dialog from "../../shared/ui/Dialog.svelte";

  interface Props {
    open: boolean;
    /** X, Escape and the backdrop behave like "Later". */
    onclose: () => void;
    ondismiss: () => void;
    onstarted: () => void;
  }

  type Step = "ask" | "dashboard" | "pairing";
  type SetupError = { title: MessageKey; detail: string };

  let { open, onclose, ondismiss, onstarted }: Props = $props();

  const keyId = $props.id();

  let step = $state<Step>("ask");
  let apiKey = $state("");
  let busy = $state(false);
  let error = $state<SetupError | null>(null);
  let keyInput: HTMLInputElement | undefined = $state();

  let soloist = $derived(music.soloist);

  $effect(() => {
    if (!open) return;
    step = "ask";
    apiKey = "";
    error = null;
    busy = false;
  });

  let title = $derived(
    step === "dashboard"
      ? t("browse.soloistSetupDashboardTitle")
      : step === "pairing"
        ? t("browse.soloistSetupPairingTitle")
        : t("browse.soloistSetupTitle"),
  );

  let installFailure = $derived.by(() => {
    if (!soloist || soloist.installed || !soloist.error) return null;
    const copy = playerErrorCopy({
      code: soloist.errorCode,
      message: soloist.error,
      retryable: false,
    });
    return { title: copy.titleKey ?? "browse.soloistError", detail: copy.detail };
  });

  function installStageLabel(stage: string): string {
    switch (stage) {
      case "downloading":
        return t("browse.soloistStageDownloading");
      case "extracting":
        return t("browse.soloistStageExtracting");
      case "verifying":
        return t("browse.soloistStageVerifying");
      case "installing":
        return t("browse.soloistStageInstalling");
      default:
        return "";
    }
  }

  function downloadPercent(downloaded: number, total: number): number {
    if (total <= 0) return 0;
    return Math.min(100, Math.round((downloaded / total) * 100));
  }

  async function openDashboard(advance: boolean): Promise<void> {
    if (busy) return;
    busy = true;
    error = null;
    try {
      await audio.openSoloistDashboard();
      if (advance) {
        step = "dashboard";
        await tick();
        keyInput?.focus();
      }
    } catch (cause) {
      error = { title: "browse.soloistError", detail: toApiError(cause).message };
    } finally {
      busy = false;
    }
  }

  async function pasteKey(): Promise<void> {
    try {
      const text = await navigator.clipboard.readText();
      if (text.trim()) apiKey = text.trim();
    } catch {
      // Clipboard access is optional; the user can always paste by hand.
    }
  }

  async function retryInstall(): Promise<void> {
    if (busy) return;
    busy = true;
    error = null;
    try {
      await audio.installSoloist();
    } catch (cause) {
      error = await describeInstallFailure(cause);
    } finally {
      busy = false;
    }
  }

  async function describeInstallFailure(cause: unknown): Promise<SetupError> {
    const fallback = toApiError(cause).message;
    try {
      const state = await audio.getState();
      const code = state.soloist?.errorCode;
      if (code) {
        const copy = playerErrorCopy({
          code,
          message: state.soloist?.error || fallback,
          retryable: false,
        });
        return {
          title: copy.titleKey ?? "browse.soloistError",
          detail: copy.detail || fallback,
        };
      }
    } catch {
      // Fall through to the raw message.
    }
    return { title: "browse.soloistError", detail: fallback };
  }

  async function saveAndStart(): Promise<void> {
    if (busy) return;
    const key = apiKey.trim();
    if (!key) {
      error = { title: "browse.soloistKeyRequired", detail: "" };
      return;
    }
    busy = true;
    error = null;
    try {
      await audio.saveSoloistAPIKey(key);
      const state = await audio.startSoloist();
      if (state.soloist?.needsPairing) {
        step = "pairing";
        return;
      }
      onstarted();
      onclose();
    } catch (cause) {
      error = await describeKeyFailure(cause);
    } finally {
      busy = false;
    }
  }

  async function describeKeyFailure(cause: unknown): Promise<SetupError> {
    const fallback = toApiError(cause).message;
    try {
      const state = await audio.getState();
      const code = state.soloist?.errorCode;
      if (code) {
        const copy = playerErrorCopy({
          code,
          message: state.soloist?.error || fallback,
          retryable: false,
        });
        return {
          title: copy.titleKey ?? "browse.soloistError",
          detail: copy.detail || fallback,
        };
      }
    } catch {
      // Fall through to the raw message.
    }
    return { title: "browse.soloistError", detail: fallback };
  }

  function finishPairing(): void {
    onstarted();
    onclose();
  }
</script>

{#snippet errorNotice()}
  {#if error}
    <div class="soloist-setup-error" role="alert" aria-live="assertive">
      <strong>{t(error.title)}</strong>
      {#if error.detail}<p>{error.detail}</p>{/if}
    </div>
  {/if}
{/snippet}

<Dialog {open} {title} {onclose}>
  {#if soloist?.installing}
    <div class="soloist-setup" aria-busy="true">
      <h3 class="soloist-setup-heading">{t("browse.soloistSetupPreparing")}</h3>
      {#if soloist.total > 0}
        <progress
          class="soloist-setup-progress"
          value={soloist.downloaded}
          max={soloist.total}
          aria-label={t("browse.soloistInstalling")}
        ></progress>
      {/if}
      <p class="soloist-setup-copy" aria-live="polite">
        {installStageLabel(soloist.installStage)}
        {#if soloist.total > 0}
          {t("browse.soloistDownloading", {
            percent: downloadPercent(soloist.downloaded, soloist.total),
          })}
        {:else}
          {t("browse.soloistDownloadingUnknown")}
        {/if}
      </p>
    </div>
  {:else if installFailure}
    <div class="soloist-setup">
      <div class="soloist-setup-error" role="alert" aria-live="assertive">
        <strong>{t(error?.title ?? installFailure.title)}</strong>
        <p>{error?.detail || installFailure.detail}</p>
      </div>
      <div class="soloist-setup-actions">
        <Button variant="ghost" size="sm" onclick={ondismiss}>
          {t("browse.soloistSetupLater")}
        </Button>
        <Button
          variant="primary"
          size="sm"
          disabled={busy}
          data-autofocus
          onclick={() => void retryInstall()}
        >
          {t("browse.soloistSetupRetry")}
        </Button>
      </div>
    </div>
  {:else if step === "ask"}
    <div class="soloist-setup">
      <p class="soloist-setup-copy">{t("browse.soloistSetupBody")}</p>
      {@render errorNotice()}
      <div class="soloist-setup-actions">
        <Button variant="ghost" size="sm" onclick={ondismiss}>
          {t("browse.soloistSetupLater")}
        </Button>
        <Button
          variant="primary"
          size="sm"
          disabled={busy}
          data-autofocus
          onclick={() => void openDashboard(true)}
        >
          {t("browse.soloistSetupGenerate")}
        </Button>
      </div>
    </div>
  {:else if step === "dashboard"}
    <div class="soloist-setup">
      <p class="soloist-setup-copy">{t("browse.soloistSetupDashboardBody")}</p>
      <div class="soloist-setup-row">
        <Button
          variant="secondary"
          size="sm"
          disabled={busy}
          onclick={() => void openDashboard(false)}
        >
          {t("browse.soloistSetupOpenDashboard")}
        </Button>
      </div>
      <div class="soloist-setup-field">
        <label class="soloist-setup-label" for={keyId}>{t("browse.soloistSetupKeyLabel")}</label>
        <div class="soloist-setup-input">
          <input
            bind:this={keyInput}
            id={keyId}
            type="password"
            autocomplete="off"
            spellcheck="false"
            placeholder={t("browse.soloistSetupKeyPlaceholder")}
            bind:value={apiKey}
          />
          <Button variant="ghost" size="sm" disabled={busy} onclick={() => void pasteKey()}>
            {t("browse.soloistSetupPaste")}
          </Button>
        </div>
      </div>
      {@render errorNotice()}
      <div class="soloist-setup-actions">
        <Button variant="ghost" size="sm" onclick={ondismiss}>
          {t("browse.soloistSetupLater")}
        </Button>
        <Button variant="primary" size="sm" disabled={busy} onclick={() => void saveAndStart()}>
          {t("browse.soloistSetupSave")}
        </Button>
      </div>
    </div>
  {:else}
    <div class="soloist-setup">
      <p class="soloist-setup-copy">{t("browse.soloistSetupPairingBody")}</p>
      {#if soloist?.loggedIn}
        <p class="soloist-setup-status" role="status">{t("browse.soloistLoggedIn")}</p>
      {/if}
      {@render errorNotice()}
      <div class="soloist-setup-actions">
        <Button variant="ghost" size="sm" onclick={ondismiss}>
          {t("browse.soloistSetupLater")}
        </Button>
        <Button variant="primary" size="sm" data-autofocus onclick={finishPairing}>
          {t("browse.soloistSetupDone")}
        </Button>
      </div>
    </div>
  {/if}
</Dialog>

<style>
  .soloist-setup {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .soloist-setup-heading {
    margin: 0;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.04em;
  }

  .soloist-setup-copy {
    margin: 0;
    color: var(--muted, #8a94a6);
    font-size: 10px;
    line-height: 1.55;
  }

  .soloist-setup-progress {
    width: 100%;
    height: 8px;
    accent-color: var(--accent, #5ef0ff);
  }

  .soloist-setup-row {
    display: flex;
    justify-content: flex-start;
  }

  .soloist-setup-field {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .soloist-setup-label {
    color: var(--muted, #8a94a6);
    font-size: 10px;
  }

  .soloist-setup-input {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .soloist-setup-input input {
    min-width: 0;
    flex: 1 1 auto;
    padding: 6px 8px;
    border: 1px solid var(--line-strong, rgb(255 255 255 / 14%));
    border-radius: 6px;
    background: var(--surface-2, #161b24);
    color: var(--text, #e8edf4);
    font: inherit;
    font-size: 11px;
  }

  .soloist-setup-input input:focus-visible {
    border-color: color-mix(in srgb, var(--accent, #5ef0ff) 45%, transparent);
    outline: 0;
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent, #5ef0ff) 10%, transparent);
  }

  .soloist-setup-status {
    margin: 0;
    padding: 4px 8px;
    border: 1px solid color-mix(in srgb, var(--accent, #5ef0ff) 32%, transparent);
    border-radius: 999px;
    background: color-mix(in srgb, var(--accent, #5ef0ff) 9%, transparent);
    align-self: flex-start;
    font-size: 10px;
  }

  .soloist-setup-error {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px 10px;
    border: 1px solid color-mix(in srgb, var(--danger, #ff6b6b) 40%, transparent);
    border-radius: 6px;
    background: color-mix(in srgb, var(--danger, #ff6b6b) 9%, transparent);
    font-size: 10px;
  }

  .soloist-setup-error strong {
    font-weight: 600;
  }

  .soloist-setup-error p {
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--muted, #8a94a6);
  }

  .soloist-setup-actions {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }
</style>
