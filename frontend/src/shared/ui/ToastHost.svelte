<script lang="ts">
  import Toast from "./Toast.svelte";
  import { toasts } from "./toasts.svelte";
</script>

<div class="ui-toast-host">
  {#each toasts.toasts as toast (toast.id)}
    <Toast {toast} ondismiss={(id) => toasts.dismiss(id)} />
  {/each}
</div>

<style>
  .ui-toast-host {
    position: fixed;
    z-index: 90;
    right: 12px;
    bottom: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    align-items: flex-end;
    pointer-events: none;
  }

  .ui-toast-host :global(.ui-toast) {
    pointer-events: auto;
  }
</style>
