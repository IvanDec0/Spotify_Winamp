<script lang="ts">
  interface Props {
    size?: number;
    class?: string;
  }

  let { size = 14, class: className = "" }: Props = $props();
</script>

<svg
  class={`brand-mark ${className}`.trim()}
  width={size}
  height={size}
  viewBox="0 0 24 24"
  aria-hidden="true"
  focusable="false"
>
  <path
    d="M17.2 7.1A7 7 0 1 0 17.2 16.9"
    fill="none"
    stroke="currentColor"
    stroke-width="2.2"
    stroke-linecap="round"
  />
  <circle cx="17.2" cy="7.1" r="1.4" fill="currentColor" />
</svg>

<style>
  .brand-mark {
    display: block;
    flex: 0 0 auto;
    color: inherit;
    filter: drop-shadow(0 0 6px color-mix(in srgb, currentColor 45%, transparent));
  }
</style>
