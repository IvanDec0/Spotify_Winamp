<script lang="ts">
  import type { Snippet } from "svelte";

  type Variant = "primary" | "secondary" | "ghost";
  type Size = "sm" | "md" | "lg";

  interface Props {
    variant?: Variant;
    size?: Size;
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    onclick?: (event: MouseEvent) => void;
    class?: string;
    ariaLabel?: string;
    title?: string;
    children?: Snippet;
  }

  let {
    variant = "secondary",
    size = "md",
    type = "button",
    disabled = false,
    onclick,
    class: className = "",
    ariaLabel,
    title,
    children,
  }: Props = $props();

  function handleClick(event: MouseEvent): void {
    if (disabled) {
      event.preventDefault();
      return;
    }
    onclick?.(event);
  }
</script>

<button
  class={`ui-button ui-button--${variant} ui-button--${size} ${className}`.trim()}
  {type}
  {disabled}
  aria-label={ariaLabel}
  {title}
  onclick={handleClick}
>
  {@render children?.()}
</button>

<style>
  .ui-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    border: 1px solid var(--line, rgb(255 255 255 / 10%));
    border-radius: 6px;
    font: inherit;
    font-size: 10px;
    line-height: 1;
    cursor: pointer;
    transition:
      background var(--motion-fast, 120ms) ease,
      border-color var(--motion-fast, 120ms) ease,
      color var(--motion-fast, 120ms) ease;
  }

  .ui-button:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }

  .ui-button--sm {
    min-height: 22px;
    padding: 4px 8px;
  }

  .ui-button--md {
    min-height: 28px;
    padding: 6px 12px;
  }

  .ui-button--lg {
    min-height: 34px;
    padding: 8px 16px;
    font-size: 11px;
  }

  .ui-button--primary {
    border-color: color-mix(in srgb, var(--accent, #5ef0ff) 44%, transparent);
    background: color-mix(in srgb, var(--accent, #5ef0ff) 16%, var(--surface, #10141b));
    color: var(--text, #e8edf4);
  }

  .ui-button--primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent, #5ef0ff) 26%, var(--surface, #10141b));
  }

  .ui-button--secondary {
    background: var(--surface-2, #161b24);
    color: var(--text, #e8edf4);
  }

  .ui-button--secondary:hover:not(:disabled) {
    background: var(--surface-3, #1d232e);
  }

  .ui-button--ghost {
    border-color: transparent;
    background: transparent;
    color: var(--muted, #8a94a6);
  }

  .ui-button--ghost:hover:not(:disabled) {
    background: var(--surface-2, #161b24);
    color: var(--text, #e8edf4);
  }
</style>
