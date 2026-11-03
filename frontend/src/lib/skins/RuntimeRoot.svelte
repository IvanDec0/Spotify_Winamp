<script lang="ts">
  import type { LayoutNode, SkinRuntime } from "../../../bindings/cadence/internal/skins";
  import { componentRegistry } from "./registry";

  interface Props {
    runtime: SkinRuntime;
  }

  let { runtime }: Props = $props();

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
        <div class="cad-missing-component">Unsupported component</div>
      {/if}
    </div>
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
