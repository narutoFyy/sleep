<template>
  <header class="stone-topbar">
    <!-- 左：移动端菜单 + 面包屑标题 -->
    <div class="flex min-w-0 items-center gap-3">
      <button class="stone-pill md:hidden" aria-label="Menu" @click="appStore.toggleMobileSidebar()">
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
        </svg>
      </button>
      <div class="stone-breadcrumb">
        <span class="stone-breadcrumb-title">{{ pageTitle || siteName }}</span>
        <span v-if="pageDescription" class="stone-breadcrumb-sub hidden sm:inline">{{ pageDescription }}</span>
      </div>
    </div>

    <!-- 右：公告 + 文档 + 语言 + 订阅 + 余额 + 用户 -->
    <div class="flex items-center gap-2.5">
      <AnnouncementBell v-if="user" />

      <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="stone-pill hidden sm:inline-flex">
        {{ t('nav.docs') }}
      </a>

      <LocaleSwitcher />

      <SubscriptionProgressMini v-if="user" />

      <span v-if="user" class="stone-pill hidden sm:inline-flex">
        <span class="text-[var(--stone-accent)]">$</span>{{ user.balance?.toFixed(2) || '0.00' }}
      </span>

      <!-- 用户下拉 -->
      <div v-if="user" class="relative" ref="dropdownRef">
        <button class="stone-rail-link !h-9 !w-9" aria-label="User Menu" @click="dropdownOpen = !dropdownOpen">
          <img v-if="avatarUrl" :src="avatarUrl" class="h-8 w-8 rounded-[var(--stone-radius-sm)] object-cover" alt="" />
          <span v-else class="grid h-8 w-8 place-items-center rounded-[var(--stone-radius-sm)] bg-gradient-to-br from-primary-500 to-primary-600 text-sm font-medium text-white">
            {{ userInitials }}
          </span>
        </button>

        <transition name="dropdown">
          <div v-if="dropdownOpen" class="dropdown right-0 mt-2 w-56">
            <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ displayName }}</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">{{ user.email }}</div>
            </div>
            <div class="py-1">
              <router-link to="/profile" @click="dropdownOpen = false" class="dropdown-item">{{ t('nav.profile') }}</router-link>
              <router-link to="/keys" @click="dropdownOpen = false" class="dropdown-item">{{ t('nav.apiKeys') }}</router-link>
            </div>
            <div class="border-t border-gray-100 py-1 dark:border-dark-700">
              <button class="dropdown-item w-full text-left" @click="handleLogout">{{ t('nav.logout') }}</button>
            </div>
          </div>
        </transition>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()

const user = computed(() => authStore.user)
const siteName = computed(() => appStore.siteName)
const docUrl = computed(() => appStore.docUrl)
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

const displayName = computed(() => user.value?.username || user.value?.email || '')
const userInitials = computed(() => {
  const name = user.value?.username || user.value?.email || ''
  return name.slice(0, 2).toUpperCase()
})

const pageTitle = computed(() => {
  if (route.name === 'CustomPage') {
    const id = route.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }
  const titleKey = route.meta.titleKey as string
  if (titleKey) return t(titleKey)
  return (route.meta.title as string) || ''
})

const pageDescription = computed(() => {
  const descKey = route.meta.descriptionKey as string
  if (descKey) return t(descKey)
  return (route.meta.description as string) || ''
})

async function handleLogout() {
  dropdownOpen.value = false
  try {
    await authStore.logout()
  } catch (error) {
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    dropdownOpen.value = false
  }
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', handleClickOutside))
</script>
