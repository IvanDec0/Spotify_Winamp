<script lang="ts">
  import { appWindow } from "../../shared/api";
  import type { LayoutNode, SkinRuntime } from "../../shared/api/types";
  import { t } from "../../shared/i18n";
  import { player } from "../player.svelte";
  import { skins } from "../skins.svelte";
  import { componentRegistry } from "./registry";

  interface Props {
    runtime: SkinRuntime;
  }

  let { runtime }: Props = $props();

  const assetBase = $derived(runtime.stylesheetUrl.replace(/[^/]*$/, ""));
  const playerControlActions = new Set(["play-pause", "next", "previous", "shuffle", "repeat"]);

  function assetURL(src: unknown): string {
    if (typeof src !== "string" || src === "") return "";
    return `${assetBase}${src.replace(/^\.?\//, "")}`;
  }

  function buttonDisabled(action: string): boolean {
    if (playerControlActions.has(action) && !player.state.canControl) return true;
    if (action === "toggle-queue" && !skins.hasQueue) return true;
    if (action === "cycle-visualizer" && !skins.hasVisualizer) return true;
    return false;
  }

  function performAction(action: string): void {
    switch (action) {
      case "open-library":
        skins.openMusic();
        break;
      case "open-settings":
        skins.openSettings();
        break;
      case "open-devices":
        skins.openDevices();
        break;
      case "toggle-queue":
        skins.toggleQueue();
        break;
      case "play-pause":
        void player.toggle();
        break;
      case "next":
        void player.next();
        break;
      case "previous":
        void player.previous();
        break;
      case "shuffle":
        void player.toggleShuffle();
        break;
      case "repeat":
        void player.cycleRepeat();
        break;
      case "cycle-visualizer":
        skins.cycleVisualizer();
        break;
      case "cycle-theme":
        skins.cycleTheme();
        break;
      case "minimise":
        void appWindow.minimise().catch(() => {});
        break;
      case "close":
        void appWindow.close().catch(() => {});
        break;
    }
  }

  const alignValues: Record<string, string> = {
    start: "flex-start",
    center: "center",
    end: "flex-end",
    stretch: "stretch",
  };
  const justifyValues: Record<string, string> = {
    start: "flex-start",
    center: "center",
    end: "flex-end",
    "space-between": "space-between",
    "space-around": "space-around",
  };

  function numeric(value: unknown): number | null {
    return typeof value === "number" && Number.isFinite(value) ? value : null;
  }

  function containerStyle(node: LayoutNode): string {
    const props = node.props ?? {};
    const style: string[] = [];

    const gap = numeric(props.gap);
    if (gap !== null) style.push(`gap:${gap}px`);
    const padding = numeric(props.padding);
    if (padding !== null) style.push(`padding:${padding}px`);

    const align = typeof props.align === "string" ? alignValues[props.align] : undefined;
    if (align) style.push(`align-items:${align}`);
    const justify = typeof props.justify === "string" ? justifyValues[props.justify] : undefined;
    if (justify) style.push(`justify-content:${justify}`);

    if (node.type === "grid") {
      const columns = numeric(props.columns);
      if (columns !== null) {
        style.push(`grid-template-columns:repeat(${Math.round(columns)}, minmax(0, 1fr))`);
      }
      const rows = numeric(props.rows);
      if (rows !== null) {
        style.push(`grid-template-rows:repeat(${Math.round(rows)}, minmax(0, 1fr))`);
      }
      if (props.flow === "column") style.push("grid-auto-flow:column");
    }
    if (node.type === "row" && props.wrap === true) {
      style.push("flex-wrap:wrap");
    }
    return style.join(";");
  }
</script>

{#snippet renderNode(node: LayoutNode, root: boolean)}
  {#if node.type === "component"}
    {@const Component = node.component ? componentRegistry[node.component] : undefined}
    <div
      class="cad-component"
      class:cad-root={root}
      data-component={node.component}
      data-name={node.name || undefined}
    >
      {#if Component}
        <Component {...node.props ?? {}} />
      {:else}
        <div class="cad-missing-component">{t("skin.unsupportedComponent")}</div>
      {/if}
    </div>
  {:else if node.type === "box"}
    <div
      class="cad-primitive cad-box"
      class:cad-root={root}
      data-primitive="box"
      data-name={node.name || undefined}
    ></div>
  {:else if node.type === "text"}
    <div
      class="cad-primitive cad-text"
      class:cad-root={root}
      data-primitive="text"
      data-name={node.name || undefined}
    >
      {typeof node.props?.text === "string" ? node.props.text : ""}
    </div>
  {:else if node.type === "image"}
    <img
      class="cad-primitive cad-image"
      class:cad-root={root}
      data-primitive="image"
      data-name={node.name || undefined}
      src={assetURL(node.props?.src)}
      alt={typeof node.props?.alt === "string" ? node.props.alt : ""}
      draggable="false"
      style={`object-fit:${node.props?.fit === "cover" ? "cover" : "contain"};`}
    />
  {:else if node.type === "button"}
    {@const action = typeof node.props?.action === "string" ? node.props.action : ""}
    <button
      class="cad-primitive cad-custom-button"
      class:cad-root={root}
      data-primitive="button"
      data-action={action}
      data-name={node.name || undefined}
      type="button"
      aria-label={typeof node.props?.label === "string" ? node.props.label : undefined}
      disabled={buttonDisabled(action)}
      onclick={() => performAction(action)}
    >
      {typeof node.props?.text === "string" ? node.props.text : ""}
    </button>
  {:else}
    <div
      class={`cad-container cad-${node.type}`}
      class:cad-root={root}
      data-name={node.name || undefined}
      style={containerStyle(node)}
    >
      {#each node.children ?? [] as child, index (index)}
        {@render renderNode(child, false)}
      {/each}
    </div>
  {/if}
{/snippet}

<div class="cad-runtime">
  {@render renderNode(runtime.layout.root, true)}
</div>
