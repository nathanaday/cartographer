import { computed, ref } from "vue";
import { apiGet, apiPut } from "../lib/api";
import { autoLayout } from "../lib/autolayout";
import { edgesFrom } from "../lib/edges";
import type { Entity, MapResponse, ProjectRef, XY } from "../types/api";

const entities = ref<Entity[]>([]);
const projects = ref<ProjectRef[]>([]);
const positions = ref<Record<string, XY>>({});
const kindFilter = ref<string | null>(null);
const fingerprint = ref<string>("");

let timer: number | null = null;

const edges = computed(() => edgesFrom(entities.value));

async function loadLayouts(slugs: string[]): Promise<Record<string, XY>> {
  const saved: Record<string, XY> = {};
  for (const slug of slugs) {
    try {
      const perProject = await apiGet<Record<string, XY>>(`/api/projects/${slug}/layout`);
      for (const [name, xy] of Object.entries(perProject)) saved[`${slug}/${name}`] = xy;
    } catch {
      // missing layout is fine
    }
  }
  return saved;
}

async function load(): Promise<void> {
  const map = await apiGet<MapResponse>("/api/map");
  fingerprint.value = map.fingerprint;
  projects.value = map.projects;
  entities.value = map.entities;
  const saved = await loadLayouts(map.projects.map((p) => p.slug));
  positions.value = autoLayout(map.entities, saved);
}

function startPolling(intervalMs = 2500): void {
  stopPolling();
  timer = window.setInterval(async () => {
    try {
      const v = await apiGet<{ fingerprint: string }>("/api/version");
      if (v.fingerprint !== fingerprint.value) await load();
    } catch {
      // transient; try again next tick
    }
  }, intervalMs);
}

function stopPolling(): void {
  if (timer !== null) {
    window.clearInterval(timer);
    timer = null;
  }
}

// savePosition updates in-memory position and persists the project's whole
// layout map (debounced by the caller). Keyed by "slug/EntityName".
async function savePosition(nodeId: string, xy: XY): Promise<void> {
  positions.value = { ...positions.value, [nodeId]: xy };
  const slug = nodeId.split("/")[0];
  const perProject: Record<string, XY> = {};
  for (const [id, pos] of Object.entries(positions.value)) {
    if (id.startsWith(`${slug}/`)) perProject[id.slice(slug.length + 1)] = pos;
  }
  await apiPut(`/api/projects/${slug}/layout`, perProject);
}

export function useMap() {
  return { entities, projects, positions, edges, kindFilter, fingerprint, load, startPolling, stopPolling, savePosition };
}
