<template>
  <AppLayout>
    <div class="marketplace-page">
      <aside class="exchange-note" aria-label="Marketplace currency conversion">
        <span>{{ t('modelMarketplace.exchangeRecharge') }}</span>
        <strong>¥1 = $1</strong>
        <span class="exchange-divider" aria-hidden="true"></span>
        <span>{{ t('modelMarketplace.exchangeMarketplace') }}</span>
        <strong>$1 = ¥1</strong>
      </aside>

      <section class="marketplace-toolbar" aria-label="Model filters">
        <div class="group-strip">
          <button
            v-for="group in groupOptions"
            :key="group.key"
            type="button"
            class="group-chip"
            :class="{ active: selectedGroup === group.key }"
            @click="selectedGroup = group.key"
          >
            <span>{{ group.label }}</span>
            <small v-if="group.rate != null">x{{ formatRate(group.rate) }}</small>
          </button>
        </div>

        <div class="toolbar-row">
          <div class="market-search">
            <Icon name="search" size="md" />
            <input v-model="searchQuery" :placeholder="t('modelMarketplace.searchPlaceholder')" />
          </div>
          <div class="market-stats">
            <span>{{ t('modelMarketplace.models', { count: filteredModels.length }) }}</span>
            <span class="stat-divider"></span>
            <span>{{ t('modelMarketplace.channels', { count: marketplaceGroups.length }) }}</span>
          </div>
          <button
            type="button"
            class="refresh-button"
            :title="t('modelMarketplace.refresh')"
            :disabled="loading"
            @click="loadMarketplace"
          >
            <Icon name="refresh" size="md" :class="{ 'animate-spin': loading }" />
          </button>
        </div>
      </section>

      <div v-if="loading && models.length === 0" class="market-state">
        <span class="loading-ring"></span>
        <p>{{ t('modelMarketplace.loading') }}</p>
      </div>

      <div v-else-if="filteredModels.length === 0" class="market-state">
        <div class="empty-mark"><Icon name="search" size="lg" /></div>
        <p>{{ t('modelMarketplace.empty') }}</p>
      </div>

      <section v-else class="platform-section-list">
        <section v-for="section in platformSections" :key="section.platform" class="platform-section">
          <header class="section-heading">
            <div>
              <p>{{ platformLabel(section.platform) }}</p>
              <span>{{ t('modelMarketplace.models', { count: section.models.length }) }}</span>
            </div>
            <span class="section-rule"></span>
          </header>

          <div class="model-grid">
            <article v-for="model in section.models" :key="model.key" class="model-card">
              <header class="model-card-header">
                <div class="model-identity">
                  <span class="model-icon-shell"><ModelIcon :model="model.name" size="28px" /></span>
                  <div class="model-title-block">
                    <h2 :title="model.name">{{ model.name }}</h2>
                    <div class="model-badges">
                      <span>{{ platformLabel(model.platform) }}</span>
                      <span class="kind-badge">{{ billingLabel(model.pricing?.billing_mode) }}</span>
                    </div>
                  </div>
                </div>
                <div class="best-price">
                  <span>{{ t('modelMarketplace.bestPrice') }}</span>
                  <strong>{{ model.groupName }}</strong>
                </div>
              </header>

              <div class="model-id-row">
                <div><span>{{ t('modelMarketplace.modelId') }}</span><code>{{ model.name }}</code></div>
                <button type="button" :title="t('modelMarketplace.copyId')" @click="copyModelId(model.name)">
                  <Icon :name="copiedId === model.name ? 'check' : 'copy'" size="sm" />
                  <span>{{ copiedId === model.name ? t('modelMarketplace.copied') : t('modelMarketplace.copyId') }}</span>
                </button>
              </div>

              <div v-if="priceMetrics(model).length" class="price-grid" :class="{ compact: priceMetrics(model).length > 3 }">
                <div v-for="metric in priceMetrics(model)" :key="metric.label" class="price-cell">
                  <p>{{ metric.label }}</p>
                  <div class="price-line"><span>{{ t('modelMarketplace.current') }}</span><strong>{{ metric.current }}</strong></div>
                  <div v-if="metric.base !== metric.current" class="base-line"><span>{{ t('modelMarketplace.base') }}</span><span>{{ metric.base }}</span></div>
                </div>
              </div>
              <div v-else class="no-pricing">{{ t('modelMarketplace.noPricing') }}</div>

              <footer class="model-card-footer">
                <span>{{ t('modelMarketplace.effectiveRate') }}</span>
                <strong>x{{ formatRate(model.effectiveRate) }}</strong>
              </footer>
            </article>
          </div>
        </section>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import userGroupsAPI, { type MarketplaceGroup, type MarketplacePricing } from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatScaled } from '@/utils/pricing'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST } from '@/constants/channel'

interface MarketModel {
  key: string
  name: string
  platform: string
  pricing: MarketplacePricing | null
  groupId: number
  groupName: string
  effectiveRate: number
}

interface PriceMetric {
  label: string
  current: string
  base: string
}

const { t } = useI18n()
const appStore = useAppStore()
const marketplaceGroups = ref<MarketplaceGroup[]>([])
const loading = ref(false)
const searchQuery = ref('')
const selectedGroup = ref('all')
const copiedId = ref('')

const accessibleGroups = computed(() => {
  return marketplaceGroups.value
    .map((group) => ({ id: group.id, name: group.name, rate: group.rate_multiplier }))
    .sort((a, b) => a.rate - b.rate || a.name.localeCompare(b.name))
})

const groupOptions = computed(() => [
  { key: 'all', label: t('modelMarketplace.allGroups'), rate: null },
  ...accessibleGroups.value.map((group) => ({ key: String(group.id), label: group.name, rate: group.rate })),
])

const models = computed<MarketModel[]>(() => {
  const best = new Map<string, MarketModel>()
  const eligibleGroups = marketplaceGroups.value.filter(
    (group) => selectedGroup.value === 'all' || String(group.id) === selectedGroup.value,
  )
  for (const group of eligibleGroups) {
    for (const model of group.models) {
      const key = `${model.platform}:${model.name}`.toLowerCase()
      const candidate: MarketModel = {
        key,
        name: model.name,
        platform: model.platform,
        pricing: model.pricing,
        groupId: group.id,
        groupName: group.name,
        effectiveRate: group.rate_multiplier,
      }
      const current = best.get(key)
      if (!current || candidate.effectiveRate < current.effectiveRate) best.set(key, candidate)
    }
  }
  return [...best.values()].sort((a, b) => a.platform.localeCompare(b.platform) || a.name.localeCompare(b.name))
})

const filteredModels = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return models.value
  return models.value.filter((model) =>
    model.name.toLowerCase().includes(q) ||
    model.platform.toLowerCase().includes(q) ||
    model.groupName.toLowerCase().includes(q),
  )
})

const platformSections = computed(() => {
  const sections = new Map<string, MarketModel[]>()
  for (const model of filteredModels.value) {
    const list = sections.get(model.platform) ?? []
    list.push(model)
    sections.set(model.platform, list)
  }
  return [...sections.entries()].map(([platform, sectionModels]) => ({ platform, models: sectionModels }))
})

function platformLabel(platform: string): string {
  const labels: Record<string, string> = {
    openai: 'OpenAI', anthropic: 'Anthropic', claude: 'Anthropic', gemini: 'Google Gemini',
    antigravity: 'Google Antigravity', bedrock: 'AWS Bedrock', vertex: 'Google Vertex AI',
  }
  return labels[platform.toLowerCase()] ?? platform
}

function billingLabel(mode?: string | null): string {
  if (mode === BILLING_MODE_IMAGE) return t('modelMarketplace.image')
  if (mode === BILLING_MODE_PER_REQUEST) return t('modelMarketplace.request')
  return t('modelMarketplace.token')
}

function formatRate(rate: number): string {
  return Number(rate.toFixed(4)).toString()
}

function scaledPrice(value: number | null, rate: number, scale: number): string {
  return formatScaled(value == null ? null : value * rate, scale)
}

function metric(label: string, value: number | null, rate: number, scale: number): PriceMetric | null {
  if (value == null) return null
  return { label, current: scaledPrice(value, rate, scale), base: formatScaled(value, scale) }
}

function priceMetrics(model: MarketModel): PriceMetric[] {
  const pricing = model.pricing
  if (!pricing) return []
  const metrics: Array<PriceMetric | null> = []
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    metrics.push(metric(t('modelMarketplace.perRequest'), pricing.per_request_price, model.effectiveRate, 1))
  } else if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    if (pricing.per_request_price != null) {
      metrics.push(metric(t('modelMarketplace.perImage'), pricing.per_request_price, model.effectiveRate, 1))
    } else {
      metrics.push(metric(t('modelMarketplace.imageOutput'), pricing.image_output_price, model.effectiveRate, 1))
      metrics.push(metric(t('modelMarketplace.input'), pricing.input_price, model.effectiveRate, 1_000_000))
      metrics.push(metric(t('modelMarketplace.output'), pricing.output_price, model.effectiveRate, 1_000_000))
    }
  } else {
    metrics.push(metric(t('modelMarketplace.input'), pricing.input_price, model.effectiveRate, 1_000_000))
    metrics.push(metric(t('modelMarketplace.cacheWrite'), pricing.cache_write_price, model.effectiveRate, 1_000_000))
    metrics.push(metric(t('modelMarketplace.cacheRead'), pricing.cache_read_price, model.effectiveRate, 1_000_000))
    metrics.push(metric(t('modelMarketplace.output'), pricing.output_price, model.effectiveRate, 1_000_000))
  }
  return metrics.filter((item): item is PriceMetric => item !== null)
}

async function copyModelId(modelId: string) {
  await navigator.clipboard.writeText(modelId)
  copiedId.value = modelId
  window.setTimeout(() => { if (copiedId.value === modelId) copiedId.value = '' }, 1400)
}

async function loadMarketplace() {
  loading.value = true
  try {
    marketplaceGroups.value = await userGroupsAPI.getModelMarketplace()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('modelMarketplace.loadFailed')))
  } finally {
    loading.value = false
  }
}

onMounted(loadMarketplace)
</script>

<style scoped>
.marketplace-page { max-width: 1480px; margin: 0 auto; padding: 18px 20px 48px; color: #172033; }
.exchange-note { display: flex; min-height: 38px; align-items: center; gap: 8px; margin-bottom: 10px; padding: 8px 12px; border: 1px solid #b9e4df; border-left: 3px solid #079688; border-radius: 7px; background: #f0fbf9; color: #526071; font-size: 12px; }
.exchange-note strong { color: #06776d; font-size: 14px; font-variant-numeric: tabular-nums; }
.exchange-divider { width: 1px; height: 16px; margin: 0 4px; background: #b9d9d6; }
.marketplace-toolbar { position: sticky; top: 68px; z-index: 20; margin-bottom: 24px; padding: 12px; border: 1px solid rgba(15,118,110,.16); border-radius: 8px; background: rgba(255,255,255,.94); box-shadow: 0 10px 30px rgba(20,49,61,.08); backdrop-filter: blur(18px); }
.group-strip { display: flex; gap: 7px; padding-bottom: 11px; overflow-x: auto; scrollbar-width: thin; }
.group-chip { display: inline-flex; min-height: 35px; flex: 0 0 auto; align-items: center; gap: 7px; padding: 7px 11px; border: 1px solid #dce3e8; border-radius: 7px; background: #f8fafb; color: #445064; font-size: 13px; transition: border-color .18s, background .18s, color .18s; }
.group-chip:hover { border-color: #73c9c0; color: #087e72; }
.group-chip.active { border-color: #079688; background: #079688; color: white; }
.group-chip small { color: inherit; opacity: .72; font-variant-numeric: tabular-nums; }
.toolbar-row { display: flex; align-items: center; gap: 12px; border-top: 1px solid #edf1f3; padding-top: 11px; }
.market-search { display: flex; width: min(360px, 100%); align-items: center; gap: 8px; padding: 0 11px; border: 1px solid #dce3e8; border-radius: 7px; background: white; color: #8b96a5; }
.market-search input { width: 100%; height: 36px; border: 0; outline: 0; background: transparent; color: #172033; font-size: 13px; }
.market-stats { display: flex; align-items: center; gap: 9px; margin-left: auto; color: #6d7787; font-size: 12px; }
.stat-divider { width: 1px; height: 14px; background: #dce3e8; }
.refresh-button { display: grid; width: 36px; height: 36px; place-items: center; border: 1px solid #dce3e8; border-radius: 7px; color: #526071; background: white; }
.refresh-button:hover { color: #087e72; border-color: #73c9c0; }
.platform-section-list { display: grid; gap: 29px; }
.section-heading { display: flex; align-items: end; gap: 16px; margin-bottom: 12px; }
.section-heading p { margin: 0; font-size: 19px; font-weight: 750; color: #172033; }
.section-heading span { color: #8791a0; font-size: 12px; }
.section-rule { height: 1px; flex: 1; margin-bottom: 5px; background: linear-gradient(90deg, #d9e5e5, transparent); }
.model-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.model-card { min-width: 0; overflow: hidden; border: 1px solid #dce3e8; border-radius: 8px; background: rgba(255,255,255,.92); box-shadow: 0 2px 8px rgba(22,42,55,.045); transition: transform .18s, border-color .18s, box-shadow .18s; }
.model-card:hover { transform: translateY(-2px); border-color: #8acfc7; box-shadow: 0 10px 24px rgba(22,78,75,.10); }
.model-card-header { display: flex; min-height: 80px; align-items: flex-start; justify-content: space-between; gap: 10px; padding: 14px 14px 10px; }
.model-identity { display: flex; min-width: 0; align-items: flex-start; gap: 10px; }
.model-icon-shell { display: grid; width: 38px; height: 38px; flex: 0 0 38px; place-items: center; border: 1px solid #dce3e8; border-radius: 8px; background: white; }
.model-title-block { min-width: 0; }
.model-title-block h2 { max-width: 235px; margin: 0 0 5px; overflow: hidden; color: #172033; font-size: 16px; font-weight: 760; text-overflow: ellipsis; white-space: nowrap; }
.model-badges { display: flex; flex-wrap: wrap; gap: 5px; }
.model-badges span { padding: 2px 7px; border-radius: 999px; background: #f0f3f5; color: #5d6878; font-size: 11px; }
.model-badges .kind-badge { background: #e8f8f6; color: #087e72; }
.best-price { display: flex; min-width: 0; flex: 0 1 155px; flex-direction: column; align-items: flex-end; gap: 2px; }
.best-price span { color: #8b95a3; font-size: 10px; }
.best-price strong { max-width: 155px; overflow: hidden; color: #087e72; font-size: 11px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.model-id-row { display: flex; min-height: 38px; align-items: center; justify-content: space-between; gap: 8px; margin: 0 12px 10px; padding: 6px 8px; border-radius: 6px; background: #f7f9fa; }
.model-id-row > div { min-width: 0; display: flex; align-items: center; gap: 7px; }
.model-id-row span { flex: 0 0 auto; color: #8a94a3; font-size: 10px; }
.model-id-row code { overflow: hidden; color: #334155; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.model-id-row button { display: flex; flex: 0 0 auto; align-items: center; gap: 4px; padding: 4px 6px; border: 1px solid #dce3e8; border-radius: 5px; background: white; color: #596577; }
.model-id-row button span { color: inherit; }
.price-grid { display: grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap: 5px; padding: 0 12px 12px; }
.price-grid.compact { grid-template-columns: repeat(2, minmax(0,1fr)); }
.price-cell { min-width: 0; padding: 8px; border: 1px solid #edf0f2; border-radius: 6px; background: #fbfcfc; }
.price-cell p { margin: 0 0 6px; overflow: hidden; color: #707b8b; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.price-line,.base-line { display: flex; align-items: baseline; justify-content: space-between; gap: 5px; }
.price-line span,.base-line span { color: #8993a1; font-size: 10px; }
.price-line strong { overflow: hidden; color: #172033; font-size: 13px; font-variant-numeric: tabular-nums; text-overflow: ellipsis; }
.base-line { margin-top: 3px; }
.base-line span:last-child { color: #a2aab5; text-decoration: line-through; font-variant-numeric: tabular-nums; }
.no-pricing { margin: 0 12px 12px; padding: 20px 12px; border: 1px dashed #dce3e8; border-radius: 6px; color: #8b95a3; text-align: center; font-size: 12px; }
.model-card-footer { display: flex; align-items: center; justify-content: space-between; padding: 9px 13px; border-top: 1px solid #edf0f2; color: #7e8998; font-size: 11px; }
.model-card-footer strong { color: #172033; font-size: 13px; font-variant-numeric: tabular-nums; }
.market-state { display: grid; min-height: 340px; place-items: center; align-content: center; gap: 12px; color: #7e8998; font-size: 13px; }
.loading-ring { width: 28px; height: 28px; border: 2px solid #dbe8e7; border-top-color: #079688; border-radius: 50%; animation: spin .8s linear infinite; }
.empty-mark { display: grid; width: 44px; height: 44px; place-items: center; border: 1px solid #dce3e8; border-radius: 8px; background: white; }
@keyframes spin { to { transform: rotate(360deg); } }
.dark .marketplace-page { color: #e7edf4; }
.dark .exchange-note { border-color: #28564f; border-left-color: #0aa899; background: #102824; color: #aab5c1; }
.dark .exchange-note strong { color: #5ee2d3; }
.dark .exchange-divider { background: #315d57; }
.dark .marketplace-toolbar,.dark .model-card { border-color: rgba(116,136,151,.25); background: rgba(17,24,32,.92); }
.dark .group-chip,.dark .market-search,.dark .refresh-button,.dark .model-icon-shell { border-color: #34404c; background: #18212a; color: #aab5c1; }
.dark .group-chip.active { border-color: #0aa899; background: #0a8f83; color: white; }
.dark .toolbar-row,.dark .model-card-footer { border-color: #29343e; }
.dark .market-search input,.dark .section-heading p,.dark .model-title-block h2,.dark .price-line strong,.dark .model-card-footer strong { color: #eef3f7; }
.dark .section-rule { background: linear-gradient(90deg, #34434d, transparent); }
.dark .model-id-row,.dark .price-cell { background: #151e26; }
.dark .model-id-row button { border-color: #34404c; background: #1d2832; color: #bdc7d0; }
.dark .model-id-row code { color: #d4dce3; }
.dark .price-cell,.dark .no-pricing { border-color: #2a3540; }
@media (max-width: 1120px) { .model-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }
@media (max-width: 700px) {
  .marketplace-page { padding: 12px 10px 32px; }
  .exchange-note { flex-wrap: wrap; }
  .exchange-divider { display: none; }
  .marketplace-toolbar { top: 58px; }
  .toolbar-row { flex-wrap: wrap; }
  .market-search { order: 1; width: calc(100% - 48px); }
  .refresh-button { order: 2; }
  .market-stats { order: 3; width: 100%; margin-left: 0; }
  .model-grid { grid-template-columns: 1fr; }
  .model-card-header { align-items: flex-start; }
  .best-price { flex-basis: 112px; }
  .best-price strong { max-width: 112px; }
}
</style>
