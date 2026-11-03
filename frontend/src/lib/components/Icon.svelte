<script lang="ts">
  import { icons, type IconName } from "../icons";

  interface Props {
    name: IconName;
    size?: number;
    strokeWidth?: number;
    label?: string;
    class?: string;
  }

  let { name, size = 16, strokeWidth = 1.6, label, class: className = "" }: Props = $props();
</script>

<svg
  class={className}
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width={strokeWidth}
  stroke-linecap="round"
  stroke-linejoin="round"
  role={label ? "img" : undefined}
  aria-hidden={label ? undefined : "true"}
  focusable="false"
>
  {#if label}<title>{label}</title>{/if}
  {#each icons[name] as shape, index (index)}
    {#if shape.kind === "path"}
      <path
        d={shape.d}
        fill={shape.fill ? "currentColor" : undefined}
        stroke={shape.fill ? "none" : undefined}
      />
    {:else if shape.kind === "rect"}
      <rect
        x={shape.x}
        y={shape.y}
        width={shape.width}
        height={shape.height}
        rx={shape.rx}
        fill={shape.fill ? "currentColor" : undefined}
        stroke={shape.fill ? "none" : undefined}
      />
    {:else}
      <circle
        cx={shape.cx}
        cy={shape.cy}
        r={shape.r}
        fill={shape.fill ? "currentColor" : undefined}
        stroke={shape.fill ? "none" : undefined}
      />
    {/if}
  {/each}
</svg>
