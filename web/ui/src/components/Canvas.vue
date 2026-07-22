<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { VueFlow, useVueFlow, type Node, type Edge } from "@vue-flow/core";
import { Background } from "@vue-flow/background";
import { Controls } from "@vue-flow/controls";
import { MiniMap } from "@vue-flow/minimap";
import EntityCard from "./EntityCard.vue";
import { useMap } from "../composables/useMap";
import "@vue-flow/core/dist/style.css";
import "@vue-flow/core/dist/theme-default.css";
import "@vue-flow/controls/dist/style.css";
import "@vue-flow/minimap/dist/style.css";

const map = useMap();
const { onNodeDragStop, onMove, viewport } = useVueFlow();

// zoom -> level of detail
const lod = ref<"full" | "headers" | "badges">("full");
function lodForZoom(z: number): "full" | "headers" | "badges" {
  if (z >= 0.8) return "full";
  if (z >= 0.4) return "headers";
  return "badges";
}
onMove(() => {
  lod.value = lodForZoom(viewport.value.zoom);
});

const nodes = computed<Node[]>(() =>
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
const edges = computed<Edge[]>(() =>
  map.edges.value.map((e) => ({ ...e, type: "default", animated: false })),
);

let saveTimer: number | null = null;
onNodeDragStop(({ node }) => {
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
});
onUnmounted(() => map.stopPolling());
</script>

<template>
  <VueFlow :nodes="nodes" :edges="edges" :min-zoom="0.15" :max-zoom="2" fit-view-on-init>
    <template #node-entity="{ data }">
      <EntityCard :data="data" />
    </template>
    <Background :gap="16" :size="1" pattern-color="var(--hairline)" />
    <Controls />
    <MiniMap />
  </VueFlow>
</template>

<style>
.vue-flow__node-entity.selected .card {
  outline: 2px solid var(--revision);
  outline-offset: 1px;
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
