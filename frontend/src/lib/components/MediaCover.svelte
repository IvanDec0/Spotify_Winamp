<script lang="ts">
  interface Props {
    src?: string;
    label: string;
    size?: number;
    round?: boolean;
  }

  let { src = "", label, size = 32, round = false }: Props = $props();
  let failed = $state(false);

  $effect(() => {
    void src;
    failed = false;
  });

  let initial = $derived(label.trim().charAt(0).toUpperCase() || "?");
</script>

<div
  class="media-cover"
  class:is-round={round}
  style={`--cover-size: ${size}px`}
  aria-hidden="true"
>
  {#if src && !failed}
    <img
      {src}
      alt=""
      draggable="false"
      onerror={() => {
        failed = true;
      }}
    />
  {:else}
    <span>{initial}</span>
  {/if}
</div>

<style>
  .media-cover {
    display: grid;
    width: var(--cover-size);
    height: var(--cover-size);
    flex: 0 0 var(--cover-size);
    place-items: center;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 5px;
    background:
      radial-gradient(
        80% 70% at 70% 20%,
        color-mix(in srgb, var(--accent) 28%, transparent),
        transparent 60%
      ),
      var(--surface-2);
    color: var(--muted);
    font-family: "JetBrains Mono", monospace;
    font-size: calc(var(--cover-size) * 0.32);
  }

  .media-cover.is-round {
    border-radius: 50%;
  }

  .media-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
</style>
