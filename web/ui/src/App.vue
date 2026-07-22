<script setup lang="ts">
import Toolbar from "./components/Toolbar.vue";
import Canvas from "./components/Canvas.vue";
</script>

<template>
  <div class="app-shell">
    <header class="titlebar">Cartographer</header>
    <Toolbar />
    <main class="stage"><Canvas /></main>
  </div>
</template>

<style scoped>
.app-shell { display: flex; flex-direction: column; height: 100%; }
.titlebar {
  font-family: var(--font-display);
  font-weight: 600;
  padding: calc(var(--unit) * 2) calc(var(--unit) * 3);
  border-bottom: 1px solid var(--hairline);
  background: var(--body);
}
.stage { flex: 1; position: relative; }
</style>
