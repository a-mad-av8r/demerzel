<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { HomeSubscriptionAccountsDto } from '@/app/resources/home'

import HomeSubscriptionAccountMiniCard from './HomeSubscriptionAccountMiniCard.vue'
import HomeSectionHeading from './HomeSectionHeading.vue'

defineProps<{ accounts: HomeSubscriptionAccountsDto }>()

const { t } = useI18n()
</script>

<template>
  <section
    v-if="accounts.items.length > 0"
    class="home-subscription-accounts"
    aria-labelledby="home-subscription-accounts-title"
  >
    <HomeSectionHeading
      id="home-subscription-accounts-title"
      :title="t('home.ledger.subscriptionAccounts.title')"
    />
    <div class="home-subscription-accounts__row">
      <HomeSubscriptionAccountMiniCard
        v-for="account in accounts.items"
        :key="`${account.channel_id}-${account.credential.credential_id}`"
        :account="account"
      />
    </div>
  </section>
</template>

<style scoped>
.home-subscription-accounts {
  display: grid;
  gap: var(--space-3);
  margin-top: 36px;
  border-top: 1px solid var(--color-border-subtle);
  padding-top: 20px;
}

.home-subscription-accounts__row {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(232px, 1fr));
  gap: 10px;
}

.home-subscription-accounts__row > * {
  min-width: 0;
}
</style>
