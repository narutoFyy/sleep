<template>
  <!-- Stone 外壳（原型）：细导航轨 + 命令栏 + 锐利面板内容区 -->
  <div v-if="stoneShell" class="tech-shell stone-shell min-h-screen">
    <TechBackground />
    <AppRail />
    <div class="relative min-h-screen md:ml-[var(--stone-rail-w)]">
      <AppCommandBar />
      <main ref="mainRef" class="stone-content">
        <slot />
      </main>
    </div>
  </div>

  <!-- 旧版外壳：宽侧栏 + 顶栏（保留，可通过开关回退） -->
  <div v-else class="tech-shell min-h-screen">
    <TechBackground />

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main ref="mainRef" class="tech-main p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import '@/styles/glass-panels.css'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import gsap from 'gsap'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import TechBackground from './TechBackground.vue'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import AppRail from './AppRail.vue'
import AppCommandBar from './AppCommandBar.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const stoneShell = computed(() => appStore.stoneShell)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const mainRef = ref<HTMLElement | null>(null)
let mainAnimation: ReturnType<typeof gsap.context> | null = null
let domObserver: MutationObserver | null = null
let updateTimer: number | null = null

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

function shouldReduceMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function initLiquidGlass() {
  if (typeof (window as any).liquidGL !== 'function') return

  // Find card-like elements globally to apply liquidGL
  const potentialTargets = document.querySelectorAll('.app-header, .sidebar, .card, .card-glass, .glass-card, .stat-card, .tech-main div.bg-white, .tech-main div.dark\\:bg-dark-800')

  if (!potentialTargets || potentialTargets.length === 0) return

  potentialTargets.forEach((el) => {
    // Strictly exclude small UI elements, dropdowns, inputs, and toggle thumbs
    if (el.clientWidth < 200 || el.clientHeight < 40) return
    if (el.closest('.dropdown, .modal-content, [role="dialog"], [role="menu"]')) return
    if (el.classList.contains('liquid-target')) return

    el.classList.add('liquid-target')
  })

  // Prevent SPA memory leak and ghost elements by clearing old lenses
  if ((window as any).__liquidGLRenderer__) {
    const renderer = (window as any).__liquidGLRenderer__
    renderer.lenses.forEach((lens: any) => {
      if (lens._shadowEl) lens._shadowEl.remove()
      if (lens._onMouseMove) lens.el.removeEventListener('mousemove', lens._onMouseMove)
      if (lens._onMouseLeave) lens.el.removeEventListener('mouseleave', lens._onMouseLeave)
      if (lens._onMouseEnter) lens.el.removeEventListener('mouseenter', lens._onMouseEnter)
    })
    renderer.lenses = []
  }

  (window as any).liquidGL({
    target: '.liquid-target',
    snapshot: 'body',
    resolution: 1.5,
    refraction: 0.05,
    bevelDepth: 0.1,
    bevelWidth: 0.1,
    frost: 5,
    shadow: true,
    specular: true,
    tilt: false,
    reveal: 'none'
  })
}

function observeDOM() {
  if (!mainRef.value) return
  if (domObserver) domObserver.disconnect()

  domObserver = new MutationObserver((mutations) => {
    let shouldUpdate = false
    mutations.forEach(mut => {
      if (mut.addedNodes.length > 0 || mut.removedNodes.length > 0) {
        shouldUpdate = true
      }
    })

    if (shouldUpdate) {
      if (updateTimer) clearTimeout(updateTimer)
      updateTimer = window.setTimeout(() => {
        initLiquidGlass()
      }, 50) // Reduced debounce to make glass apply faster
    }
  })

  domObserver.observe(mainRef.value, { childList: true, subtree: true })
}

function animateMainContent() {
  if (shouldReduceMotion() || !mainRef.value) return

  mainAnimation?.revert()
  mainAnimation = gsap.context(() => {
    initLiquidGlass() // Initialize glass immediately before GSAP animation starts

    gsap.from(mainRef.value?.children ?? [], {
      y: 20,
      scale: 0.98,
      opacity: 0,
      duration: 0.5,
      stagger: 0.05,
      ease: 'power4.out',
      clearProps: 'transform,opacity,scale'
    })
  }, mainRef.value)
}

onMounted(async () => {
  onboardingStore.setReplayCallback(replayTour)
  await nextTick()
  animateMainContent()
  observeDOM()
})

watch(
  () => route.fullPath,
  async () => {
    await nextTick()
    animateMainContent()
  }
)

onBeforeUnmount(() => {
  mainAnimation?.revert()
  if (domObserver) domObserver.disconnect()
  if (updateTimer) clearTimeout(updateTimer)
})

defineExpose({ replayTour })
</script>
