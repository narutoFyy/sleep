<template>
  <aside class="stone-rail" :class="{ 'is-open': mobileOpen }">
    <!-- Logo -->
    <router-link :to="homePath" class="stone-rail-logo" :aria-label="siteName">
      <img v-if="settingsLoaded" :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
    </router-link>

    <!-- 导航图标 -->
    <nav class="stone-rail-scroll" aria-label="Primary">
      <component
        :is="item.expandOnly ? 'button' : 'router-link'"
        v-for="item in flatNav"
        :key="item.key"
        :to="item.expandOnly ? undefined : item.path"
        type="button"
        class="stone-rail-link"
        :class="{ 'is-active': isActive(item.path) }"
        @click="onNavClick(item)"
      >
        <span v-if="item.iconSvg" class="h-5 w-5 stone-svg" v-html="sanitizeSvg(item.iconSvg)"></span>
        <component v-else :is="item.icon" class="h-5 w-5" />
        <span class="stone-rail-tip">{{ item.label }}</span>
      </component>
    </nav>

    <!-- 底部：主题切换 -->
    <button class="stone-rail-link" :title="isDark ? t('nav.lightMode') : t('nav.darkMode')" @click="toggleTheme">
      <SunIcon v-if="isDark" class="h-5 w-5 text-amber-400" />
      <MoonIcon v-else class="h-5 w-5" />
      <span class="stone-rail-tip">{{ isDark ? t('nav.lightMode') : t('nav.darkMode') }}</span>
    </button>
  </aside>

  <!-- 移动端遮罩 -->
  <transition name="fade">
    <div v-if="mobileOpen" class="fixed inset-0 z-30 bg-black/60 md:hidden" @click="closeMobile"></div>
  </transition>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { sanitizeSvg } from '@/utils/sanitize'
import { useDashboardNav, type NavItem } from '@/composables/useDashboardNav'
import { MoonIcon, SunIcon } from './navIcons'

interface FlatNavItem extends NavItem {
  key: string
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { userNavItems, adminNavItems } = useDashboardNav()

const mobileOpen = computed(() => appStore.mobileOpen)
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => appStore.siteLogo)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const isAdmin = computed(() => authStore.isAdmin)
const homePath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const isDark = ref(document.documentElement.classList.contains('dark'))

// 细导航轨无层级展开，把带 children 的分组展平为子项（用子项图标直接呈现）。
function flatten(items: NavItem[]): FlatNavItem[] {
  const out: FlatNavItem[] = []
  for (const item of items) {
    if (item.children?.length) {
      for (const child of item.children) out.push({ ...child, key: `${item.path}>${child.path}` })
    } else {
      out.push({ ...item, key: item.path })
    }
  }
  return out
}

const flatNav = computed<FlatNavItem[]>(() =>
  isAdmin.value ? flatten(adminNavItems.value) : flatten(userNavItems.value),
)

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

function onNavClick(item: FlatNavItem) {
  if (mobileOpen.value) appStore.setMobileOpen(false)
  if (item.expandOnly) router.push(item.path)
}

function closeMobile() {
  appStore.setMobileOpen(false)
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}
</script>

<style scoped>
.stone-svg :deep(svg) {
  display: block;
  width: 1.25rem;
  height: 1.25rem;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
