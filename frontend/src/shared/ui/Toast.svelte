<script lang="ts">
  import { t } from "../i18n";
  import type { ToastMessage } from "./toasts.svelte";

  interface Props {
    toast: ToastMessage;
    ondismiss: (id: number) => void;
  }

  let { toast, ondismiss }: Props = $props();
</script>

<div class={`ui-toast ui-toast--${toast.kind}`} role={toast.kind === "error" ? "alert" : "status"}>
  <span class="ui-toast-message">{toast.message}</span>
  <button
    class="ui-toast-close"
    type="button"
    aria-label={t("ui.dismiss")}
    onclick={() => ondismiss(toast.id)}
  >
    ×
  </button>
</div>

<style>
  .ui-toast {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 180px;
    max-width: min(360px, calc(100vw - 24px));
    padding: 9px 9px 9px 12px;
    border: 1px solid var(--line, rgb(255 255 255 / 12%));
    border-radius: var(--radius-sm, 7px);
    background:
      linear-gradient(180deg, rgb(255 255 255 / 5%), transparent 46%), var(--surface-2, #161b24);
    box-shadow:
      0 14px 34px rgb(0 0 0 / 52%),
      0 0 24px color-mix(in srgb, var(--accent, #5ef0ff) 5%, transparent);
    color: var(--text, #e8edf4);
    backdrop-filter: blur(14px);
    animation: toast-in 260ms var(--ease-out, cubic-bezier(0.16, 1, 0.3, 1)) both;
  }

  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateY(8px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  .ui-toast--error {
    border-color: color-mix(in srgb, var(--danger, #ff6b8a) 36%, transparent);
  }

  .ui-toast--success {
    border-color: color-mix(in srgb, var(--accent, #5ef0ff) 36%, transparent);
  }

  .ui-toast-message {
    flex: 1 1 auto;
    font-size: 10px;
    line-height: 1.35;
  }

  .ui-toast-close {
    display: grid;
    width: 18px;
    height: 18px;
    flex: 0 0 18px;
    place-items: center;
    border-radius: 3px;
    color: var(--muted, #8a94a6);
    font-size: 13px;
    line-height: 1;
  }

  .ui-toast-close:hover {
    background: var(--surface-3, #1d232e);
    color: var(--text, #e8edf4);
  }
</style>
