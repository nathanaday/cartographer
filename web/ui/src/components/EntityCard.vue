<script setup lang="ts">
import { computed, ref } from "vue";
import { sigilFor } from "../lib/sigil";
import type { Entity } from "../types/api";

const props = defineProps<{
  data: { entity: Entity; lod: "full" | "headers" | "badges"; kindFilter: string | null };
}>();

const entity = computed(() => props.data.entity);
const lod = computed(() => props.data.lod);
const kinds = computed(() => entity.value.aspects.map((a) => a.kind));

const collapsed = ref<Record<string, boolean>>({});
function toggle(kind: string) {
  collapsed.value = { ...collapsed.value, [kind]: !collapsed.value[kind] };
}

function tintVar(kind: string): string {
  return `var(--k-${kind}, var(--k-unknown))`;
}
function dimmed(kind: string): boolean {
  return props.data.kindFilter !== null && props.data.kindFilter !== kind;
}
</script>

<template>
  <div class="card">
    <div class="titleblock">
      <div class="row1">
        <span class="name">{{ entity.name }}</span>
        <span class="project">{{ entity.project }}</span>
      </div>
      <div class="badges">
        <span v-for="k in kinds" :key="k" class="badge" :style="{ background: tintVar(k) }">{{ k }}</span>
      </div>
      <div v-if="entity.description && lod !== 'badges'" class="desc">{{ entity.description }}</div>
    </div>

    <template v-if="lod !== 'badges'">
      <div v-for="aspect in entity.aspects" :key="aspect.kind" class="strip" :class="{ dim: dimmed(aspect.kind) }">
        <button class="strip-head" :style="{ background: tintVar(aspect.kind) }" @click="toggle(aspect.kind)">
          <span class="kind">{{ aspect.kind }}</span>
          <span class="count">· {{ aspect.fields.length }}</span>
          <span class="chevron">{{ collapsed[aspect.kind] ? "▸" : "▾" }}</span>
        </button>
        <div v-if="lod === 'full' && !collapsed[aspect.kind]" class="fields">
          <div v-for="f in aspect.fields" :key="f.name" class="field">
            <span class="sigil">{{ sigilFor(f.type) }}</span>
            <span class="fname">{{ f.name }}</span>
            <span class="ftype">{{ f.type }}</span>
            <span v-if="f.references" class="port" title="relationship">→</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.card {
  width: 340px;
  background: var(--body);
  border: 1px solid var(--hairline);
  border-radius: 3px;
  font-family: var(--font-body);
  overflow: hidden;
}
.titleblock { padding: calc(var(--unit) * 2); border-bottom: 1px solid var(--hairline); }
.row1 { display: flex; justify-content: space-between; align-items: baseline; }
.name { font-family: var(--font-display); font-weight: 600; font-size: 15px; color: var(--ink); }
.project { font-size: 11px; color: var(--steel); }
.badges { display: flex; gap: var(--unit); margin-top: var(--unit); flex-wrap: wrap; }
.badge { font-size: 10px; padding: 1px 6px; border-radius: 2px; color: var(--ink); }
.desc { margin-top: var(--unit); font-size: 11px; color: var(--steel); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.strip.dim { opacity: 0.35; }
.strip-head {
  width: 100%; display: flex; align-items: center; gap: var(--unit);
  border: none; border-bottom: 1px solid var(--hairline); padding: calc(var(--unit) * 1.5) calc(var(--unit) * 2);
  font-family: var(--font-body); font-size: 12px; color: var(--ink); cursor: pointer; text-align: left;
}
.strip-head .count { color: var(--steel); font-family: var(--font-mono); }
.strip-head .chevron { margin-left: auto; color: var(--steel); }
.fields { padding: var(--unit) 0; }
.field {
  display: grid; grid-template-columns: 20px 1fr auto 14px; align-items: center; gap: var(--unit);
  padding: 2px calc(var(--unit) * 2); font-family: var(--font-mono); font-size: 12.5px; line-height: 22px;
}
.sigil { color: var(--steel); }
.fname { color: var(--ink); }
.ftype { color: var(--steel); text-align: right; }
.port { color: var(--steel); }
</style>
