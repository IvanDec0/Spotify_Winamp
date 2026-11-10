<script lang="ts">
  import { tick, type Snippet } from "svelte";
  import { t } from "../i18n";

  interface Props {
    open: boolean;
    title: string;
    onclose: () => void;
    children?: Snippet;
  }

  let { open, title, onclose, children }: Props = $props();

  const titleId = $props.id();
  const FOCUSABLE =
    "button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]";

  let dialog: HTMLDivElement | undefined = $state();
  let restoreFocus: HTMLElement | null = null;

  function isVisible(node: HTMLElement): boolean {
    const checkVisibility = (node as HTMLElement & { checkVisibility?: () => boolean })
      .checkVisibility;
    return typeof checkVisibility !== "function" || checkVisibility.call(node);
  }

  function focusable(): HTMLElement[] {
    if (!dialog) return [];
    return Array.from(dialog.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((node) => {
      if (node instanceof HTMLInputElement && node.type === "hidden") return false;
      if (node.closest("[hidden], [aria-hidden='true'], [inert]")) return false;
      const tabindex = node.getAttribute("tabindex");
      if (tabindex !== null && Number(tabindex) < 0) return false;
      return isVisible(node);
    });
  }

  $effect(() => {
    if (!open) return;
    restoreFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    void tick().then(() => {
      const target =
        dialog?.querySelector<HTMLElement>("[data-autofocus]") ?? focusable()[0] ?? dialog;
      target?.focus();
    });
    return () => {
      restoreFocus?.focus();
      restoreFocus = null;
    };
  });

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      onclose();
      return;
    }
    if (event.key !== "Tab") return;

    const items = focusable();
    if (items.length === 0) {
      event.preventDefault();
      dialog?.focus();
      return;
    }

    const first = items[0];
    const last = items[items.length - 1];
    const active = document.activeElement;

    if (event.shiftKey && (active === first || active === dialog)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }
</script>

{#if open}
  <div class="ui-dialog-overlay">
    <button
      class="ui-dialog-backdrop"
      type="button"
      tabindex="-1"
      aria-label={t("ui.dismiss")}
      onclick={onclose}
    ></button>
    <div
      class="ui-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      tabindex="-1"
      bind:this={dialog}
      onkeydown={handleKeydown}
    >
      <header class="ui-dialog-head">
        <h2 id={titleId}>{title}</h2>
        <button class="ui-dialog-close" type="button" aria-label={t("ui.close")} onclick={onclose}>
          ×
        </button>
      </header>
      <div class="ui-dialog-body">
        {@render children?.()}
      </div>
    </div>
  </div>
{/if}

<style>
  .ui-dialog-overlay {
    position: fixed;
    z-index: 100;
    inset: 0;
    display: grid;
    place-items: center;
    animation: dialog-fade 220ms ease-out both;
  }

  .ui-dialog-backdrop {
    position: absolute;
    inset: 0;
    border: 0;
    background: rgb(0 0 0 / 52%);
    cursor: default;
    backdrop-filter: blur(4px);
  }

  .ui-dialog {
    position: relative;
    width: min(440px, calc(100vw - 24px));
    max-height: calc(100vh - 24px);
    overflow: auto;
    border: 1px solid var(--line, rgb(255 255 255 / 12%));
    border-radius: var(--radius-md, 11px);
    background:
      linear-gradient(180deg, rgb(255 255 255 / 5%), transparent 30%),
      color-mix(in srgb, var(--surface, #10141b) 94%, transparent);
    color: var(--text, #e8edf4);
    box-shadow:
      0 24px 64px rgb(0 0 0 / 58%),
      0 0 0 1px rgb(0 0 0 / 26%),
      0 0 42px color-mix(in srgb, var(--accent, #5ef0ff) 7%, transparent);
    backdrop-filter: blur(22px) saturate(125%);
    animation: dialog-in 260ms var(--ease-out, cubic-bezier(0.16, 1, 0.3, 1)) both;
  }

  @keyframes dialog-fade {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes dialog-in {
    from {
      opacity: 0;
      transform: translateY(10px) scale(0.97);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  .ui-dialog-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--line, rgb(255 255 255 / 10%));
  }

  .ui-dialog-head h2 {
    overflow: hidden;
    margin: 0;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ui-dialog-close {
    display: grid;
    width: 20px;
    height: 20px;
    flex: 0 0 20px;
    place-items: center;
    border-radius: 4px;
    color: var(--muted, #8a94a6);
    font-size: 14px;
    line-height: 1;
  }

  .ui-dialog-close:hover {
    background: var(--surface-2, #161b24);
    color: var(--text, #e8edf4);
  }

  .ui-dialog-body {
    padding: 12px;
  }
</style>
