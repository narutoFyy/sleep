<script setup lang="ts">
import { RouterView, useRouter, useRoute } from 'vue-router'
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import Toast from '@/components/common/Toast.vue'
import NavigationProgress from '@/components/common/NavigationProgress.vue'
import AdminComplianceDialog from '@/components/admin/AdminComplianceDialog.vue'
import { resolveDocumentTitle } from '@/router/title'
import AnnouncementPopup from '@/components/common/AnnouncementPopup.vue'
import AuthDrawer from '@/components/auth/AuthDrawer.vue'
import { useAppStore, useAuthStore, useSubscriptionStore, useAnnouncementStore, useAdminComplianceStore } from '@/stores'
import { getSetupStatus } from '@/api/setup'

type AuthMode = 'login' | 'register'

const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()
const announcementStore = useAnnouncementStore()
const adminComplianceStore = useAdminComplianceStore()
const authDrawerOpen = ref(false)
const authDrawerMode = ref<AuthMode>('login')

/**
 * Update favicon dynamically
 * @param logoUrl - URL of the logo to use as favicon
 */
function updateFavicon(logoUrl: string) {
  // Find existing favicon link or create new one
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.type = logoUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/x-icon'
  link.href = logoUrl
}

// Watch for site settings changes and update favicon/title
watch(
  () => appStore.siteLogo,
  (newLogo) => {
    if (newLogo) {
      updateFavicon(newLogo)
    }
  },
  { immediate: true }
)

// Watch for authentication state and manage subscription data + announcements
function onVisibilityChange() {
  if (document.visibilityState === 'visible' && authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
}

function onAdminComplianceRequired(event: Event) {
  const detail = (event as CustomEvent<Record<string, string>>).detail || {}
  adminComplianceStore.requireAcknowledgement(detail)
}

function isAuthMode(value: unknown): value is AuthMode {
  return value === 'login' || value === 'register'
}

function setAuthDrawerMode(mode: AuthMode) {
  authDrawerMode.value = mode
  if (route.path === '/home' && route.query.auth !== mode) {
    router.replace({
      path: route.path,
      query: { ...route.query, auth: mode }
    })
  }
}

function openAuthDrawer(mode: AuthMode) {
  if (authStore.isAuthenticated) {
    router.push(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  authDrawerMode.value = mode
  authDrawerOpen.value = true
}

function closeAuthDrawer() {
  authDrawerOpen.value = false
  if (route.query.auth) {
    const { auth: _auth, ...query } = route.query
    router.replace({ path: route.path, query })
  }
}

function handleStoneAuthMessage(event: MessageEvent) {
  if (event.origin !== window.location.origin) return

  const payload = event.data as { type?: string; mode?: unknown } | null
  if (payload?.type === 'stone-auth' && isAuthMode(payload.mode)) {
    if (route.path !== '/home') {
      router.push({ path: '/home', query: { auth: payload.mode } })
      return
    }
    openAuthDrawer(payload.mode)
  }
}

watch(
  () => authStore.isAuthenticated,
  (isAuthenticated, oldValue) => {
    if (isAuthenticated) {
      if (authStore.isAdmin) {
        adminComplianceStore.fetchStatus().catch((error) => {
          console.error('Failed to fetch admin compliance status:', error)
        })
      }

      // User logged in: preload subscriptions and start polling
      subscriptionStore.fetchActiveSubscriptions().catch((error) => {
        console.error('Failed to preload subscriptions:', error)
      })
      subscriptionStore.startPolling()

      // Announcements: new login vs page refresh restore
      if (oldValue === false) {
        // New login: delay 3s then force fetch
        setTimeout(() => announcementStore.fetchAnnouncements(true), 3000)
      } else {
        // Page refresh restore (oldValue was undefined)
        announcementStore.fetchAnnouncements()
      }

      // Register visibility change listener
      document.addEventListener('visibilitychange', onVisibilityChange)
    } else {
      // User logged out: clear data and stop polling
      subscriptionStore.clear()
      announcementStore.reset()
      adminComplianceStore.reset()
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  },
  { immediate: true }
)

// Route change trigger (throttled by store)
router.afterEach(() => {
  if (authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
  window.removeEventListener('admin-compliance-required', onAdminComplianceRequired)
  window.removeEventListener('message', handleStoneAuthMessage)
})

watch(
  () => route.query.auth,
  (value) => {
    const authValue = Array.isArray(value) ? value[0] : value
    if (route.path === '/home' && isAuthMode(authValue)) {
      openAuthDrawer(authValue)
      return
    }

    if (authDrawerOpen.value) {
      authDrawerOpen.value = false
    }
  },
  { immediate: true }
)

onMounted(async () => {
  window.addEventListener('admin-compliance-required', onAdminComplianceRequired)
  window.addEventListener('message', handleStoneAuthMessage)

  // Check if setup is needed
  try {
    const status = await getSetupStatus()
    if (status.needs_setup && route.path !== '/setup') {
      router.replace('/setup')
      return
    }
  } catch {
    // If setup endpoint fails, assume normal mode and continue
  }

  // Load public settings into appStore (will be cached for other components)
  await appStore.fetchPublicSettings()

  // Re-resolve document title now that siteName is available
  document.title = resolveDocumentTitle(route.meta.title, appStore.siteName, route.meta.titleKey as string)
})
</script>

<template>
  <NavigationProgress />
  <RouterView />
  <AuthDrawer
    :open="authDrawerOpen"
    :mode="authDrawerMode"
    @close="closeAuthDrawer"
    @update:mode="setAuthDrawerMode"
  />
  <Toast />
  <AnnouncementPopup />
  <AdminComplianceDialog />
</template>
