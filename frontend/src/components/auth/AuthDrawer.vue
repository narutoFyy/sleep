<template>
  <Teleport to="body">
    <Transition
      :css="false"
      @enter="onEnter"
      @leave="onLeave"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="open"
        class="auth-drawer-overlay"
        role="dialog"
        aria-modal="true"
        :aria-label="activeMode === 'login' ? t('auth.signIn') : t('auth.createAccount')"
        @click.self="emit('close')"
      >
        <aside ref="panelRef" class="auth-drawer-panel">
          <div class="auth-drawer-topline"></div>
          <div class="flex items-center justify-between gap-4 border-b border-white/10 px-5 py-4 sm:px-7">
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.28em] text-primary-300/80">
                STONE ACCESS
              </p>
              <h2 class="mt-1 text-lg font-semibold text-white">
                {{ activeMode === 'login' ? t('auth.signIn') : t('auth.createAccount') }}
              </h2>
            </div>
            <button
              type="button"
              class="btn btn-ghost btn-icon shrink-0"
              :aria-label="t('common.close')"
              @click="emit('close')"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.7">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div class="auth-drawer-body">
            <div class="auth-drawer-switch" role="tablist" :aria-label="t('auth.signIn')">
              <button
                type="button"
                class="auth-drawer-switch-button"
                :class="{ 'is-active': activeMode === 'login' }"
                @click="emit('update:mode', 'login')"
              >
                {{ t('auth.signIn') }}
              </button>
              <button
                type="button"
                class="auth-drawer-switch-button"
                :class="{ 'is-active': activeMode === 'register' }"
                @click="emit('update:mode', 'register')"
              >
                {{ t('auth.signUp') }}
              </button>
            </div>

            <LoginForm
              v-if="activeMode === 'login'"
              mode="drawer"
              @switch-mode="emit('update:mode', $event)"
              @success="emit('close')"
            />
            <RegisterForm
              v-else
              mode="drawer"
              @switch-mode="emit('update:mode', $event)"
              @success="emit('close')"
            />
          </div>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import gsap from 'gsap'
import LoginForm from '@/components/auth/LoginForm.vue'
import RegisterForm from '@/components/auth/RegisterForm.vue'

type AuthMode = 'login' | 'register'

const props = defineProps<{
  open: boolean
  mode: AuthMode
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'update:mode', mode: AuthMode): void
}>()

const { t } = useI18n()
const panelRef = ref<HTMLElement | null>(null)
const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)')

const activeMode = computed(() => props.mode)

watch(
  () => props.open,
  (isOpen) => {
    document.body.classList.toggle('modal-open', isOpen)
  }
)

onBeforeUnmount(() => {
  document.body.classList.remove('modal-open')
})

function onEnter(element: Element, done: () => void) {
  const overlay = element as HTMLElement
  const panel = panelRef.value
  if (!panel || reduceMotion.matches) {
    done()
    return
  }

  gsap.set(overlay, { opacity: 0 })
  gsap.set(panel, { xPercent: 105, opacity: 0 })
  gsap.timeline({ onComplete: done })
    .to(overlay, { opacity: 1, duration: 0.18, ease: 'power2.out' })
    .to(panel, { xPercent: 0, opacity: 1, duration: 0.46, ease: 'power3.out' }, '<0.02')
}

function onLeave(element: Element, done: () => void) {
  const overlay = element as HTMLElement
  const panel = panelRef.value
  if (!panel || reduceMotion.matches) {
    done()
    return
  }

  gsap.timeline({ onComplete: done })
    .to(panel, { xPercent: 105, opacity: 0, duration: 0.28, ease: 'power2.in' })
    .to(overlay, { opacity: 0, duration: 0.18, ease: 'power2.in' }, '<0.05')
}

function onAfterLeave(element: Element) {
  gsap.set(element, { clearProps: 'all' })
}
</script>
