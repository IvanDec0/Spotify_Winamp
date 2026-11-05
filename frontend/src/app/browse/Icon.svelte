<script lang="ts">
  import { BROWSE_ICONS, type BrowseIcon } from "./icons";

  interface Props {
    name: BrowseIcon;
    size?: number;
    filled?: boolean;
  }

  let { name, size = 14, filled = false }: Props = $props();
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill={filled ? "currentColor" : "none"}
  stroke="currentColor"
  stroke-width="1.7"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  <path d={BROWSE_ICONS[name]} />
</svg>
