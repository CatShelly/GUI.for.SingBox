<script setup lang="ts">
import FeatureSettings from './components/FeatureSettings.vue'
import NetworkSettings from './components/NetworkSettings.vue'
import PersonalizationSettings from './components/PersonalizationSettings.vue'
</script>

<template>
  <div class="flex flex-col gap-8">
    <PersonalizationSettings />
    <NetworkSettings />
    <FeatureSettings />
  </div>
</template>
