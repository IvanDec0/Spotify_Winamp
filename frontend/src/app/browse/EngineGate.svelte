<script lang="ts">
  import { onDestroy } from "svelte";
  import { engineErrorHelpKey, engineErrorKey } from "../../lib/engine-error";
  import { music } from "../../lib/music.svelte";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Icon from "./Icon.svelte";

  let copied = $state(false);
  let resetArmed = $state(false);
  let copyTimer: ReturnType<typeof setTimeout> | null = null;
  let engineError = $derived(music.audio.error ? engineErrorKey(music.audio.errorCode) : null);
  let engineHelp = $derived(engineErrorHelpKey(music.audio.errorCode));

  onDestroy(() => {
    if (copyTimer) clearTimeout(copyTimer);
  });

  function fallbackCopy(text: string): boolean {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.left = "-10000px";
    document.body.appendChild(area);
    area.select();
    try {
      return document.execCommand("copy");
    } catch {
      return false;
    } finally {
      area.remove();
    }
  }

  async function copyCode(): Promise<void> {
    const code = music.pairing?.code;
    if (!code) return;
    let copiedNow: boolean;
    try {
      await navigator.clipboard.writeText(code);
      copiedNow = true;
    } catch {
      copiedNow = fallbackCopy(code);
    }
    if (!copiedNow) return;
    copied = true;
    if (copyTimer) clearTimeout(copyTimer);
    copyTimer = setTimeout(() => {
      copyTimer = null;
      copied = false;
    }, 1600);
  }

  async function resetPairing(): Promise<void> {
    if (!resetArmed) {
      resetArmed = true;
      return;
    }
    resetArmed = false;
    await music.resetSession();
  }
</script>

{#snippet diagnosticDetail()}
  {#if music.audio.error}
    <details class="engine-gate-detail">
      <summary>{t("errors.detail")}</summary>
      <p>{music.audio.error}</p>
    </details>
  {/if}
{/snippet}

{#if !music.authenticated}
  <section class="browse-view engine-gate" aria-labelledby="engine-gate-title" aria-live="polite">
    <div class="engine-gate-card">
      {#if music.loading && !music.audio.available}
        <p class="eyebrow">{t("browse.pairing.localDevice")}</p>
        <h1 id="engine-gate-title" class="view-title">{t("browse.pairing.startingTitle")}</h1>
        <p class="engine-gate-copy">{t("browse.pairing.startingBody")}</p>
      {:else if !music.audio.available}
        <span class="engine-gate-icon" aria-hidden="true">
          <Icon name="settings" size={22} />
        </span>
        <p class="eyebrow">{t("browse.pairing.localDevice")}</p>
        <h1 id="engine-gate-title" class="view-title">
          {engineError ? t(engineError) : t("browse.pairing.unavailableTitle")}
        </h1>
        {#if music.audio.errorCode === "engine_disabled"}
          <p class="engine-gate-copy">{t("errors.engineDisabledHelp")}</p>
        {:else if music.audio.errorCode && music.audio.errorCode !== "engine_not_installed"}
          <p class="engine-gate-copy">{t(engineHelp)}</p>
        {:else}
          <p class="engine-gate-copy">{t("browse.pairing.unavailableBody")}</p>
        {/if}
        {#if music.audio.binaryPath}
          <p class="engine-gate-path">{music.audio.binaryPath}</p>
        {/if}
        {@render diagnosticDetail()}
        {#if music.audio.errorCode !== "engine_disabled"}
          <div class="engine-gate-actions">
            <Button
              variant="primary"
              size="sm"
              disabled={music.busy}
              onclick={() => void music.restart()}
            >
              {music.busy ? t("ui.loading") : t("browse.pairing.retry")}
            </Button>
          </div>
        {/if}
      {:else if engineError}
        <span class="engine-gate-icon" aria-hidden="true">
          <Icon name="settings" size={22} />
        </span>
        <p class="eyebrow">{t("browse.pairing.localDevice")}</p>
        <h1 id="engine-gate-title" class="view-title">{t(engineError)}</h1>
        <p class="engine-gate-copy">{t(engineHelp)}</p>
        {@render diagnosticDetail()}
        <div class="engine-gate-actions">
          <Button
            variant="primary"
            size="sm"
            disabled={music.busy}
            onclick={() => void music.restart()}
          >
            {music.busy ? t("ui.loading") : t("browse.pairing.retry")}
          </Button>
        </div>
      {:else if music.pairing}
        <p class="eyebrow">{t("browse.pairing.localDevice")}</p>
        <h1 id="engine-gate-title" class="view-title">{t("browse.pairing.title")}</h1>
        <p class="engine-gate-copy">{t("browse.pairing.hint")}</p>
        <code class="pairing-code">{music.pairing.code}</code>
        <div class="engine-gate-actions">
          <Button
            variant="primary"
            size="sm"
            disabled={music.busy}
            onclick={() => void music.openPairing()}
          >
            {t("browse.pairing.open")}
          </Button>
          <Button variant="secondary" size="sm" onclick={() => void copyCode()}>
            {copied ? t("browse.pairing.copied") : t("browse.pairing.copy")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="engine-gate-danger"
            disabled={music.busy}
            onclick={() => void resetPairing()}
          >
            {resetArmed ? t("browse.pairing.confirmReset") : t("browse.pairing.reset")}
          </Button>
        </div>
        {#if music.pairing.url}
          <p class="engine-gate-path">{music.pairing.url}</p>
        {/if}
      {:else}
        <p class="eyebrow">{t("browse.pairing.localDevice")}</p>
        <h1 id="engine-gate-title" class="view-title">{t("browse.pairing.startingTitle")}</h1>
        <p class="engine-gate-copy">{t("browse.pairing.waitingBody")}</p>
        <div class="engine-gate-actions">
          <Button variant="secondary" size="sm" onclick={() => void music.ensureEngine()}>
            {t("browse.pairing.checkAgain")}
          </Button>
        </div>
      {/if}

      {#if music.busy}
        <p class="engine-gate-status">{t("ui.loading")}</p>
      {/if}
      {#if music.error}
        <p class="browse-banner is-error" role="alert" aria-live="assertive">
          {t("browse.pairing.errorLabel")}
          {music.error}
        </p>
      {/if}
      {#if music.notice}
        <p class="engine-gate-status" role="status">
          {t(music.notice.key, music.notice.params)}
        </p>
      {/if}
    </div>
  </section>
{/if}
