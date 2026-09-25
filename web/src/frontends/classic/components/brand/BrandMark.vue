<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    size?: number | string
    label?: string
  }>(),
  {
    size: 40,
    label: undefined,
  },
)

const dimension = computed(() => (typeof props.size === 'number' ? String(props.size) : props.size))
</script>

<template>
  <img
    class="brand-mark"
    src="/assets/demerzel_logo.png?v=eb7f04c2e92e"
    :width="dimension"
    :height="dimension"
    :alt="label ?? ''"
  />
</template>

<style scoped>
.brand-mark {
  display: block;
  flex: none;
  object-fit: contain;
}
</style>
