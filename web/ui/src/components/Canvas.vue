<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { VueFlow, useVueFlow, Panel, type Node, type Edge } from "@vue-flow/core";
import { Background } from "@vue-flow/background";
import { Controls } from "@vue-flow/controls";
import { MiniMap } from "@vue-flow/minimap";
import EntityCard from "./EntityCard.vue";
import { useMap } from "../composables/useMap";
import { projectRegions } from "../lib/regions";
import "@vue-flow/core/dist/style.css";
import "@vue-flow/core/dist/theme-default.css";
import "@vue-flow/controls/dist/style.css";
import "@vue-flow/minimap/dist/style.css";

const map = useMap();
const { onNodeDragStop, onMove, onNodeDoubleClick, setCenter, viewport, fitView } = useVueFlow();

const resetArmed = ref(false);
let armTimer: number | null = null;
function onReset(): void {
  if (!resetArmed.value) {
    resetArmed.value = true;
    if (armTimer !== null) window.clearTimeout(armTimer);
    armTimer = window.setTimeout(() => (resetArmed.value = false), 3000);
    return;
  }
  resetArmed.value = false;
  if (armTimer !== null) window.clearTimeout(armTimer);
  void map.resetLayout().then(async () => {
    // let the new masonry positions propagate into VueFlow's node store and
    // the pane re-measure before fitting, or fitView sees stale bounds.
    await nextTick();
    requestAnimationFrame(() => fitView({ padding: 0.1 }));
  });
}

// zoom -> level of detail
const lod = ref<"full" | "headers" | "chip">("chip");
function lodForZoom(z: number): "full" | "headers" | "chip" {
  if (z >= 0.8) return "full";
  if (z >= 0.45) return "headers";
  return "chip";
}
const chipScale = computed(() => Math.min(1 / viewport.value.zoom, 5));
onMove(() => {
  lod.value = lodForZoom(viewport.value.zoom);
});
onNodeDoubleClick(({ node }) => {
  if (node.type !== "entity") return;
  const w = node.dimensions?.width ?? 340;
  const h = node.dimensions?.height ?? 100;
  void setCenter(node.position.x + w / 2, node.position.y + h / 2, { zoom: 1 });
});

const entityNodes = computed<Node[]>(() =>
  map.entities.value.map((e) => {
    const id = `${e.project}/${e.name}`;
    return {
      id,
      type: "entity",
      position: map.positions.value[id] ?? { x: 0, y: 0 },
      data: { entity: e, lod: lod.value, kindFilter: map.kindFilter.value },
    };
  }),
);

const regionNodes = computed<Node[]>(() =>
  projectRegions(map.entities.value, map.positions.value, map.projects.value).map((r) => ({
    id: `region:${r.slug}`,
    type: "region",
    zIndex: -1,
    draggable: false,
    selectable: false,
    position: { x: r.x, y: r.y },
    data: { label: r.label, w: r.w, h: r.h },
  })),
);

const nodes = computed<Node[]>(() => [...regionNodes.value, ...entityNodes.value]);
const edges = computed<Edge[]>(() =>
  map.edges.value.map((e) => ({ ...e, type: "default", animated: false })),
);

let saveTimer: number | null = null;
onNodeDragStop(({ node }) => {
  if (node.type !== "entity") return;
  // grid-quantize (multiples of 8) and debounce the persist
  const gx = Math.round(node.position.x / 8) * 8;
  const gy = Math.round(node.position.y / 8) * 8;
  if (saveTimer !== null) window.clearTimeout(saveTimer);
  saveTimer = window.setTimeout(() => {
    void map.savePosition(node.id, { x: gx, y: gy });
  }, 300);
});

onMounted(async () => {
  await map.load();
  map.startPolling();
  lod.value = lodForZoom(viewport.value.zoom); // after fit-view-on-init
});
onUnmounted(() => {
  map.stopPolling();
  if (armTimer !== null) window.clearTimeout(armTimer);
});
</script>

<template>
  <VueFlow
    :nodes="nodes"
    :edges="edges"
    :min-zoom="0.05"
    :max-zoom="2"
    :zoom-on-double-click="false"
    fit-view-on-init
    :style="{ '--chip-scale': String(chipScale) }"
  >
    <template #node-entity="{ data }">
      <EntityCard :data="data" />
    </template>
    <template #node-region="{ data }">
      <div class="region" :style="{ width: data.w + 'px', height: data.h + 'px' }">
        <span class="region-label">{{ data.label }}</span>
      </div>
    </template>
    <Background :gap="16" :size="1" pattern-color="var(--hairline)" />
    <Controls />
    <Panel position="top-right" class="canvas-actions">
      <button @click="fitView({ padding: 0.1 })">fit</button>
      <button :class="{ armed: resetArmed }" @click="onReset">
        {{ resetArmed ? "confirm reset" : "reset layout" }}
      </button>
    </Panel>
    <MiniMap v-if="map.entities.value.length > 10" />
  </VueFlow>
</template>

<style>
.canvas-actions {
  display: flex;
  border: 1px solid var(--hairline);
  border-radius: 3px;
  overflow: hidden;
}
.canvas-actions button {
  border: none;
  border-right: 1px solid var(--hairline);
  background: var(--body);
  font-family: var(--font-body);
  font-size: 12px;
  color: var(--steel);
  padding: calc(var(--unit) * 1) calc(var(--unit) * 2);
  cursor: pointer;
}
.canvas-actions button:last-child {
  border-right: none;
}
.canvas-actions button.armed {
  background: var(--ink);
  color: var(--body);
}
.chip {
  transform: scale(var(--chip-scale, 1));
  transform-origin: top left;
}
.vue-flow__node-entity.selected .card {
  outline: 2px solid var(--revision);
  outline-offset: 1px;
}
.vue-flow__node-region {
  pointer-events: none;
}
.region {
  position: relative;
  width: 100%;
  height: 100%;
  background: transparent;
  border: 1px solid var(--hairline);
  border-radius: 3px;
  pointer-events: none;
}
.region-label {
  position: absolute;
  top: -22px;
  left: 0;
  font-family: var(--font-display);
  font-size: 13px;
  font-weight: 600;
  color: var(--steel);
}
.vue-flow__edge-path {
  stroke: var(--hairline);
  stroke-width: 1.5;
}
.vue-flow__edge.selected .vue-flow__edge-path,
.vue-flow__edge:hover .vue-flow__edge-path {
  stroke: var(--steel);
  stroke-width: 2;
}
</style>
