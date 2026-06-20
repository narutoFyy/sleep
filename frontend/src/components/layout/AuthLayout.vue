<template>
  <div class="auth-tech-shell relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <div class="auth-tech-grid" aria-hidden="true"></div>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="mb-8 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div class="auth-logo-chip mb-4 inline-flex h-20 w-20 items-center justify-center overflow-hidden">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-cover" />
          </div>
          <h1 class="text-gradient mb-2 text-3xl font-bold tracking-tight">
            {{ siteName }}
          </h1>
          <p class="text-sm text-[var(--stone-text-soft)]">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div ref="cardRef" class="auth-card-glass p-8">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import gsap from 'gsap'
import { useAppStore } from '@/stores'
import { normalizeSiteName } from '@/stores/app'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const cardRef = ref<HTMLElement | null>(null)
let authAnimation: ReturnType<typeof gsap.context> | null = null

const siteName = computed(() => normalizeSiteName(appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

function animateAuthCard() {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches || !cardRef.value) return

  authAnimation = gsap.context(() => {
    gsap.from(cardRef.value, {
      y: 14,
      opacity: 0,
      duration: 0.42,
      ease: 'power3.out',
      clearProps: 'transform,opacity'
    })
  }, cardRef.value)
}

onMounted(async () => {
  appStore.fetchPublicSettings()
  await nextTick()
  animateAuthCard()
})

onBeforeUnmount(() => {
  authAnimation?.revert()
})
</script>

<style scoped>
.text-gradient {
  @apply text-primary-600 dark:text-primary-300;
}
</style>
