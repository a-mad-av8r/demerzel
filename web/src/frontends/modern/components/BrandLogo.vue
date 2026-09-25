<script setup lang="ts">
defineProps<{ compact?: boolean }>()
</script>

<template>
  <span class="modern-brand-logo" :class="{ 'is-compact': compact }">
    <img
      class="modern-brand-logo__image"
      src="/assets/demerzel_logo.png?v=eb7f04c2e92e"
      alt=""
      width="40"
      height="40"
    />
    <span v-if="!compact" class="modern-brand-logo__name">Demerzel</span>
  </span>
</template>

<style scoped>
.modern-brand-logo {
  display: inline-flex;
  min-width: 0;
  max-width: 100%;
  flex: none;
  align-items: center;
  gap: var(--modern-space-2);
}
.modern-brand-logo__image {
  display: block;
  width: 40px;
  height: 40px;
  flex: none;
  object-fit: contain;
}
.is-compact .modern-brand-logo__image {
  width: var(--modern-control-sm);
  height: var(--modern-control-sm);
}
.modern-brand-logo__name {
  min-width: 0;
  color: var(--modern-text);
  font-size: var(--modern-font-size-section);
  font-weight: var(--modern-weight-semibold);
  line-height: var(--modern-leading-compact);
  white-space: nowrap;
}
</style>
