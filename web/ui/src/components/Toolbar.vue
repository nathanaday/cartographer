<script setup lang="ts">
import { computed } from "vue";
import { useMap } from "../composables/useMap";

const map = useMap();
const allKinds = computed(() => {
  const set = new Set<string>();
  for (const e of map.entities.value) for (const a of e.aspects) set.add(a.kind);
  return [...set].sort();
});
function select(kind: string | null) {
  map.kindFilter.value = kind;
}
</script>

<template>
  <div class="toolbar">
    <span class="label">kind</span>
    <div class="segmented">
      <button :class="{ active: map.kindFilter.value === null }" @click="select(null)">all</button>
      <button
        v-for="k in allKinds"
        :key="k"
        :class="{ active: map.kindFilter.value === k }"
        @click="select(k)"
      >
        {{ k }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex; align-items: center; gap: calc(var(--unit) * 2);
  padding: var(--unit) calc(var(--unit) * 3);
  border-bottom: 1px solid var(--hairline); background: var(--body);
}
.label { font-size: 11px; color: var(--steel); font-family: var(--font-mono); }
.segmented { display: flex; border: 1px solid var(--hairline); border-radius: 3px; overflow: hidden; }
.segmented button {
  border: none; border-right: 1px solid var(--hairline); background: var(--body);
  font-family: var(--font-body); font-size: 12px; color: var(--steel);
  padding: calc(var(--unit) * 1) calc(var(--unit) * 2); cursor: pointer;
}
.segmented button:last-child { border-right: none; }
.segmented button.active { background: var(--ink); color: var(--body); }
</style>
