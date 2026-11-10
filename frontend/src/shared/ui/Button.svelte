<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLButtonAttributes } from "svelte/elements";

  type Variant = "primary" | "secondary" | "ghost";
  type Size = "sm" | "md" | "lg";

  type NativeButtonProps = Omit<
    HTMLButtonAttributes,
    "class" | "type" | "disabled" | "onclick" | "children"
  >;

  type Props = NativeButtonProps & {
    variant?: Variant;
    size?: Size;
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    onclick?: (event: MouseEvent) => void;
    class?: string;
    ariaLabel?: string;
    title?: string;
    children?: Snippet;
  };

  let {
    variant = "secondary",
    size = "md",
    type = "button",
    disabled = false,
    onclick,
    class: className = "",
    ariaLabel,
    "aria-label": ariaLabelNative,
    title,
    children,
    ...rest
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
  {...rest}
  class={`ui-button ui-button--${variant} ui-button--${size} ${className}`.trim()}
  {type}
  {disabled}
  aria-label={ariaLabel ?? ariaLabelNative}
  {title}
  onclick={handleClick}
>
  {@render children?.()}
</button>

<style>
  .ui-button {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    overflow: hidden;
    border: 1px solid var(--line, rgb(255 255 255 / 10%));
    border-radius: var(--radius-sm, 7px);
    font: inherit;
    font-size: 10px;
    line-height: 1;
    cursor: pointer;
    transition:
      background-color var(--motion-fast, 140ms) ease,
      border-color var(--motion-fast, 140ms) ease,
      color var(--motion-fast, 140ms) ease,
      box-shadow var(--motion-base, 240ms) ease,
      transform var(--motion-fast, 140ms) var(--ease-spring, ease);
  }

  .ui-button::after {
    content: "";
    position: absolute;
    inset: 0;
    background: linear-gradient(180deg, rgb(255 255 255 / 7%), transparent 52%);
    pointer-events: none;
  }

  .ui-button:hover:not(:disabled) {
    transform: translateY(-1px);
  }

  .ui-button:active:not(:disabled) {
    transform: translateY(0) scale(0.97);
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
    border-color: color-mix(in srgb, var(--accent, #5ef0ff) 48%, transparent);
    background:
      linear-gradient(180deg, color-mix(in srgb, white 9%, transparent), transparent 58%),
      color-mix(in srgb, var(--accent, #5ef0ff) 20%, var(--surface, #10141b));
    color: var(--text, #e8edf4);
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--accent, #5ef0ff) 5%, transparent),
      0 6px 18px color-mix(in srgb, var(--accent, #5ef0ff) 10%, transparent);
  }

  .ui-button--primary:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--accent, #5ef0ff) 64%, transparent);
    background:
      linear-gradient(180deg, color-mix(in srgb, white 12%, transparent), transparent 58%),
      color-mix(in srgb, var(--accent, #5ef0ff) 30%, var(--surface, #10141b));
    box-shadow:
      0 0 0 4px color-mix(in srgb, var(--accent, #5ef0ff) 7%, transparent),
      0 10px 26px color-mix(in srgb, var(--accent, #5ef0ff) 18%, transparent);
  }

  .ui-button--secondary {
    background:
      linear-gradient(180deg, rgb(255 255 255 / 5%), transparent 56%), var(--surface-2, #161b24);
    color: var(--text, #e8edf4);
  }

  .ui-button--secondary:hover:not(:disabled) {
    border-color: var(--line-strong, rgb(255 255 255 / 14%));
    background:
      linear-gradient(180deg, rgb(255 255 255 / 7%), transparent 56%), var(--surface-3, #1d232e);
  }

  .ui-button--ghost {
    border-color: transparent;
    background: transparent;
    color: var(--muted, #8a94a6);
  }

  .ui-button--ghost::after {
    display: none;
  }

  .ui-button--ghost:hover:not(:disabled) {
    background: var(--surface-2, #161b24);
    color: var(--text, #e8edf4);
  }
</style>
