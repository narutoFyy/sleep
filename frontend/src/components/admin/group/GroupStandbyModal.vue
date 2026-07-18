<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.standby.title', { name: group?.name || '' })"
    width="extra-wide"
    @close="emit('close')"
  >
    <div v-if="loading" class="flex min-h-48 items-center justify-center text-sm text-gray-500">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="group && isStandbyProvider(group.platform)" class="space-y-6">
      <section class="border-b border-gray-200 pb-6 dark:border-dark-600">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.groups.standby.addAccounts') }}
          </h4>
          <button type="button" class="btn btn-secondary" @click="openCreateAccount">
            <Icon name="plus" size="sm" class="mr-2" />
            {{ t('admin.groups.standby.createAccount') }}
          </button>
        </div>

        <div class="mb-3">
          <input
            v-model="search"
            type="search"
            class="input"
            :placeholder="t('admin.groups.standby.searchAccounts')"
          />
        </div>

        <div
          v-if="availableAccounts.length"
          class="max-h-48 divide-y divide-gray-200 overflow-y-auto border-y border-gray-200 dark:divide-dark-600 dark:border-dark-600"
        >
          <label
            v-for="account in availableAccounts"
            :key="account.id"
            class="flex cursor-pointer items-center gap-3 px-2 py-3 hover:bg-gray-50 dark:hover:bg-dark-700"
          >
            <input v-model="selectedAccountIds" type="checkbox" :value="account.id" class="h-4 w-4" />
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ account.name }}</span>
              <span class="block text-xs text-gray-500 dark:text-gray-400">#{{ account.id }} · {{ account.type }}</span>
            </span>
            <span :class="['badge', account.status === 'active' ? 'badge-success' : 'badge-gray']">
              {{ t(`admin.accounts.status.${account.status}`) }}
            </span>
          </label>
        </div>
        <p v-else class="py-4 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.standby.noAvailableAccounts') }}
        </p>

        <div class="mt-3 flex justify-end">
          <button
            type="button"
            class="btn btn-primary"
            :disabled="selectedAccountIds.length === 0"
            @click="addSelectedAccounts"
          >
            <Icon name="plus" size="sm" class="mr-2" />
            {{ t('admin.groups.standby.addSelected', { count: selectedAccountIds.length }) }}
          </button>
        </div>
      </section>

      <section>
        <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.groups.standby.configuredAccounts') }}
        </h4>

        <div v-if="entries.length" class="divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-600 dark:border-dark-600">
          <article v-for="entry in entries" :key="entry.accountId" class="space-y-4 py-5">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h5 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ entry.account.name }}</h5>
                  <span class="badge badge-primary">{{ t('admin.groups.standby.role') }}</span>
                  <span :class="['badge', entry.account.status === 'active' ? 'badge-success' : 'badge-gray']">
                    {{ t(`admin.accounts.status.${entry.account.status}`) }}
                  </span>
                </div>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">#{{ entry.accountId }} · {{ entry.account.type }}</p>
              </div>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  class="rounded-lg p-2 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700"
                  :disabled="testingIds.has(entry.accountId)"
                  :title="t('admin.groups.standby.testAccount')"
                  @click="testAccount(entry.accountId)"
                >
                  <Icon name="bolt" size="sm" :class="testingIds.has(entry.accountId) ? 'animate-pulse' : ''" />
                </button>
                <button
                  type="button"
                  class="rounded-lg p-2 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                  :title="t('admin.groups.standby.removeAccount')"
                  @click="removeEntry(entry.accountId)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-[minmax(0,1fr)_10rem]">
              <div>
                <label class="input-label">{{ t('admin.groups.standby.enabled') }}</label>
                <button
                  type="button"
                  role="switch"
                  :aria-checked="entry.enabled"
                  :class="[
                    'relative mt-2 inline-flex h-6 w-11 items-center rounded-full transition-colors',
                    entry.enabled ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'
                  ]"
                  @click="entry.enabled = !entry.enabled"
                >
                  <span :class="['inline-block h-4 w-4 rounded-full bg-white transition-transform', entry.enabled ? 'translate-x-6' : 'translate-x-1']" />
                </button>
              </div>
              <div>
                <label class="input-label">{{ t('admin.groups.standby.priority') }}</label>
                <input v-model.number="entry.priority" type="number" min="1" step="1" class="input mt-2" />
              </div>
            </div>

            <div>
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                <label class="input-label">{{ t('admin.groups.standby.modelMappings') }}</label>
                <button type="button" class="btn btn-secondary btn-sm" @click="addMappingRow(entry)">
                  <Icon name="plus" size="sm" class="mr-1" />
                  {{ t('admin.groups.standby.addMapping') }}
                </button>
              </div>

              <div class="space-y-2">
                <div
                  v-for="(row, index) in entry.rows"
                  :key="row.key"
                  class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.5rem]"
                >
                  <input
                    v-model="row.pattern"
                    class="input min-w-0"
                    :placeholder="sourcePlaceholder"
                    :aria-label="t('admin.groups.standby.requestModel')"
                  />
                  <input
                    v-model="row.target"
                    class="input min-w-0"
                    :placeholder="targetPlaceholder"
                    :aria-label="t('admin.groups.standby.upstreamModel')"
                  />
                  <button
                    type="button"
                    class="h-10 w-10 rounded-lg text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                    :title="t('common.delete')"
                    @click="entry.rows.splice(index, 1)"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
              <p v-if="errors[entry.accountId]" class="mt-2 text-xs text-red-600 dark:text-red-400">
                {{ errors[entry.accountId] }}
              </p>
            </div>
          </article>
        </div>
        <p v-else class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.standby.noConfiguredAccounts') }}
        </p>
      </section>
    </div>

    <div v-else class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.groups.standby.unsupportedProvider') }}
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <CreateAccountModal
    :show="showCreateAccount"
    :proxies="proxies"
    :groups="[]"
    :default-platform="group?.platform"
    :default-concurrency="STANDBY_ACCOUNT_DEFAULTS.concurrency"
    :default-base-rpm="STANDBY_ACCOUNT_DEFAULTS.baseRpm"
    @close="showCreateAccount = false"
    @created="handleAccountCreated"
  />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account, AdminGroup, Proxy } from '@/types'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import CreateAccountModal from '@/components/account/CreateAccountModal.vue'
import {
  STANDBY_ACCOUNT_DEFAULTS,
  getStandbyMembership,
  hasPrimaryMembership,
  isStandbyProvider,
  mappingToRows,
  mergeStandbyMembership,
  removeMembershipFromGroup,
  rowsToMapping,
  validateMappingRows
} from '@/views/admin/groupStandby'

interface MappingRow {
  key: number
  pattern: string
  target: string
}

interface StandbyEntry {
  accountId: number
  account: Account
  enabled: boolean
  priority: number
  rows: MappingRow[]
}

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const accounts = ref<Account[]>([])
const proxies = ref<Proxy[]>([])
const entries = ref<StandbyEntry[]>([])
const removedAccountIds = ref(new Set<number>())
const selectedAccountIds = ref<number[]>([])
const testingIds = ref(new Set<number>())
const errors = ref<Record<number, string>>({})
const search = ref('')
const loading = ref(false)
const saving = ref(false)
const showCreateAccount = ref(false)
const accountIdsBeforeCreate = ref(new Set<number>())
let rowKey = 0

const sourcePlaceholder = computed(() => props.group?.platform === 'openai' ? 'gpt-*' : 'claude-*')
const targetPlaceholder = computed(() => props.group?.platform === 'openai' ? 'gpt-5.4' : 'claude-sonnet-4-5')

const availableAccounts = computed(() => {
  const query = search.value.trim().toLowerCase()
  const configured = new Set(entries.value.map((entry) => entry.accountId))
  return accounts.value.filter((account) => {
    if (!props.group || account.platform !== props.group.platform || configured.has(account.id)) return false
    if (hasPrimaryMembership(account, props.group.id)) return false
    if (query && !`${account.name} ${account.id} ${account.type}`.toLowerCase().includes(query)) return false
    return true
  })
})

function toRows(mapping?: Record<string, string>): MappingRow[] {
  return mappingToRows(mapping).map((row) => ({ key: ++rowKey, ...row }))
}

function createEntry(account: Account, priority: number): StandbyEntry {
  const membership = props.group ? getStandbyMembership(account, props.group.id) : undefined
  return {
    accountId: account.id,
    account,
    enabled: membership?.enabled ?? true,
    priority: membership?.priority ?? priority,
    rows: toRows(membership?.model_mapping)
  }
}

async function load() {
  if (!props.group || !isStandbyProvider(props.group.platform)) return
  loading.value = true
  errors.value = {}
  try {
    const [loadedAccounts, loadedProxies] = await Promise.all([
      adminAPI.accounts.listAll({ platform: props.group.platform }),
      adminAPI.proxies.getAll()
    ])
    accounts.value = loadedAccounts
    proxies.value = loadedProxies
    entries.value = loadedAccounts
      .filter((account) => getStandbyMembership(account, props.group!.id))
      .map((account, index) => createEntry(account, index + 1))
      .sort((a, b) => a.priority - b.priority || a.accountId - b.accountId)
    removedAccountIds.value = new Set()
    selectedAccountIds.value = []
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || t('admin.groups.standby.loadFailed'))
  } finally {
    loading.value = false
  }
}

function addSelectedAccounts() {
  let nextPriority = entries.value.reduce((max, entry) => Math.max(max, entry.priority), 0) + 1
  for (const accountId of selectedAccountIds.value) {
    const account = accounts.value.find((item) => item.id === accountId)
    if (!account || !props.group || hasPrimaryMembership(account, props.group.id)) continue
    if (!entries.value.some((entry) => entry.accountId === accountId)) {
      entries.value.push(createEntry(account, nextPriority++))
    }
    removedAccountIds.value.delete(accountId)
  }
  selectedAccountIds.value = []
}

function removeEntry(accountId: number) {
  const account = accounts.value.find((item) => item.id === accountId)
  if (account && props.group && getStandbyMembership(account, props.group.id)) {
    removedAccountIds.value.add(accountId)
  }
  entries.value = entries.value.filter((entry) => entry.accountId !== accountId)
  const nextErrors = { ...errors.value }
  delete nextErrors[accountId]
  errors.value = nextErrors
}

function addMappingRow(entry: StandbyEntry) {
  entry.rows.push({ key: ++rowKey, pattern: '', target: '' })
}

function mappingErrorMessage(code: string): string {
  return t(`admin.groups.standby.errors.${code}`)
}

function validate(): boolean {
  if (!props.group) return false
  const nextErrors: Record<number, string> = {}
  for (const entry of entries.value) {
    if (!Number.isInteger(entry.priority) || entry.priority < 1) {
      nextErrors[entry.accountId] = t('admin.groups.standby.errors.PRIORITY_INVALID')
      continue
    }
    const code = validateMappingRows(entry.rows, props.group.platform)
    if (code) nextErrors[entry.accountId] = mappingErrorMessage(code)
  }
  errors.value = nextErrors
  return Object.keys(nextErrors).length === 0
}

async function save() {
  if (!props.group || !validate()) return
  saving.value = true
  try {
    const updates = entries.value.map((entry) => {
      const memberships = mergeStandbyMembership(entry.account, props.group!.id, {
        enabled: entry.enabled,
        priority: entry.priority,
        modelMapping: rowsToMapping(entry.rows)
      })
      return adminAPI.accounts.updateGroupMemberships(entry.accountId, memberships)
    })
    for (const accountId of removedAccountIds.value) {
      const account = accounts.value.find((item) => item.id === accountId)
      if (account) {
        updates.push(adminAPI.accounts.updateGroupMemberships(accountId, removeMembershipFromGroup(account, props.group.id)))
      }
    }
    await Promise.all(updates)
    appStore.showSuccess(t('admin.groups.standby.saved'))
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.groups.standby.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function testAccount(accountId: number) {
  testingIds.value = new Set(testingIds.value).add(accountId)
  try {
    const result = await adminAPI.accounts.testAccount(accountId)
    if (result.success) {
      appStore.showSuccess(result.latency_ms ? `${result.message} (${result.latency_ms} ms)` : result.message)
    } else {
      appStore.showError(result.message)
    }
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.groups.standby.testFailed'))
  } finally {
    const next = new Set(testingIds.value)
    next.delete(accountId)
    testingIds.value = next
  }
}

function openCreateAccount() {
  accountIdsBeforeCreate.value = new Set(accounts.value.map((account) => account.id))
  showCreateAccount.value = true
}

async function handleAccountCreated() {
  if (!props.group) return
  try {
    const loadedAccounts = await adminAPI.accounts.listAll({ platform: props.group.platform })
    accounts.value = loadedAccounts
    const newIds = loadedAccounts
      .filter((account) => !accountIdsBeforeCreate.value.has(account.id) && !hasPrimaryMembership(account, props.group!.id))
      .map((account) => account.id)
    selectedAccountIds.value = newIds
    addSelectedAccounts()
  } catch {
    appStore.showError(t('admin.groups.standby.loadFailed'))
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) load()
    else showCreateAccount.value = false
  }
)
</script>
