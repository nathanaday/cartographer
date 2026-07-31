import { computed, ref } from "vue";
import { apiGet, apiPut } from "../lib/api";
import { autoLayout } from "../lib/autolayout";
import { edgesFrom } from "../lib/edges";
import type { Entity, MapResponse, ProjectRef, XY } from "../types/api";

const entities = ref<Entity[]>([]);
const projects = ref<ProjectRef[]>([]);
const pinned = ref<Record<string, XY>>({});
const kindFilter = ref<string | null>(null);
const positions = computed(() => autoLayout(entities.value, pinned.value, kindFilter.value));
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
  pinned.value = await loadLayouts(map.projects.map((p) => p.slug));
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

async function putPinned(slug: string): Promise<void> {
  const perProject: Record<string, XY> = {};
  for (const [id, pos] of Object.entries(pinned.value)) {
    if (id.startsWith(`${slug}/`)) perProject[id.slice(slug.length + 1)] = pos;
  }
  await apiPut(`/api/projects/${slug}/layout`, perProject);
}

// pinPositions merges every update into pinned.value in one assignment, then
// PUTs each affected project's pinned map once.
async function pinPositions(updates: Record<string, XY>): Promise<void> {
  pinned.value = { ...pinned.value, ...updates };
  const slugs = new Set(Object.keys(updates).map((id) => id.split("/")[0]));
  for (const slug of slugs) await putPinned(slug);
}

// savePosition pins one card (user-moved) and persists only pinned entries.
async function savePosition(nodeId: string, xy: XY): Promise<void> {
  await pinPositions({ [nodeId]: xy });
}

// resetLayout unpins every card and clears each project's layout file.
async function resetLayout(): Promise<void> {
  pinned.value = {};
  for (const p of projects.value) await putPinned(p.slug);
}

export function useMap() {
  return {
    entities,
    projects,
    positions,
    pinned,
    edges,
    kindFilter,
    fingerprint,
    load,
    startPolling,
    stopPolling,
    savePosition,
    pinPositions,
    resetLayout,
  };
}
