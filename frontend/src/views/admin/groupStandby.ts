import type { Account, AccountGroupMembership, AccountPlatform } from '@/types'

export type StandbyProvider = 'anthropic' | 'openai'
export type AccountWithGroupMemberships = Account

export interface StandbyMembershipDraft {
  accountId: number
  enabled: boolean
  priority: number
  modelMapping: Record<string, string>
}

export const STANDBY_ACCOUNT_DEFAULTS = {
  concurrency: 50,
  baseRpm: 100,
  quota: null
} as const

export function isStandbyProvider(platform: AccountPlatform | string): platform is StandbyProvider {
  return platform === 'anthropic' || platform === 'openai'
}

export function isProviderCompatible(account: Pick<Account, 'platform'>, groupPlatform: string): boolean {
  return isStandbyProvider(groupPlatform) && account.platform === groupPlatform
}

function normalizeMemberships(account: Pick<AccountWithGroupMemberships, 'account_groups' | 'group_ids'>): AccountGroupMembership[] {
  const memberships = [...(account.account_groups ?? [])]
  const knownGroups = new Set(memberships.map((membership) => membership.group_id))

  for (const groupId of account.group_ids ?? []) {
    if (!knownGroups.has(groupId)) {
      memberships.push({ group_id: groupId, role: 'primary', enabled: true, priority: 1 })
    }
  }

  return memberships
}

export function getStandbyMembership(
  account: Pick<AccountWithGroupMemberships, 'account_groups' | 'group_ids'>,
  groupId: number
): AccountGroupMembership | undefined {
  return normalizeMemberships(account).find(
    (membership) => membership.group_id === groupId && membership.role === 'standby'
  )
}

export function hasPrimaryMembership(
  account: Pick<AccountWithGroupMemberships, 'account_groups' | 'group_ids'>,
  groupId: number
): boolean {
  return normalizeMemberships(account).some(
    (membership) => membership.group_id === groupId && (membership.role === 'primary' || !membership.role)
  )
}

export function mergeSelectedGroupMemberships(
  account: Pick<AccountWithGroupMemberships, 'account_groups' | 'group_ids'>,
  selectedGroupIds: number[]
): AccountGroupMembership[] {
  const existingByGroupId = new Map(
    normalizeMemberships(account).map((membership) => [membership.group_id, membership])
  )

  return selectedGroupIds.map((groupId, index) => {
    const existing = existingByGroupId.get(groupId)
    if (existing) return { ...existing, model_mapping: { ...(existing.model_mapping ?? {}) } }
    return {
      group_id: groupId,
      role: 'primary',
      enabled: true,
      priority: index + 1,
      model_mapping: {}
    }
  })
}

export function mergeStandbyMembership(
  account: Pick<AccountWithGroupMemberships, 'account_groups' | 'group_ids'>,
  groupId: number,
  draft: Omit<StandbyMembershipDraft, 'accountId'>
): AccountGroupMembership[] {
  const memberships = normalizeMemberships(account)
  const existing = memberships.find((membership) => membership.group_id === groupId)

  if (existing && (existing.role === 'primary' || !existing.role)) {
    throw new Error('PRIMARY_MEMBERSHIP_CONFLICT')
  }

  const nextMembership: AccountGroupMembership = {
    ...(existing ?? {}),
    group_id: groupId,
    role: 'standby',
    enabled: draft.enabled,
    priority: draft.priority,
    model_mapping: { ...draft.modelMapping }
  }

  return existing
    ? memberships.map((membership) => membership === existing ? nextMembership : membership)
    : [...memberships, nextMembership]
}

export function removeMembershipFromGroup(
  account: Pick<AccountWithGroupMemberships, 'account_groups' | 'group_ids'>,
  groupId: number
): AccountGroupMembership[] {
  return normalizeMemberships(account).filter((membership) => membership.group_id !== groupId)
}

export function validateModelMapping(
  mapping: Record<string, string>,
  provider: string
): string | null {
  const entries = Object.entries(mapping)
  if (entries.length === 0) return 'MAPPING_REQUIRED'

  const patterns = new Set<string>()
  for (const [rawPattern, rawTarget] of entries) {
    const pattern = rawPattern.trim()
    const target = rawTarget.trim()
    if (!pattern || !target) return 'MAPPING_FIELDS_REQUIRED'
    if (patterns.has(pattern)) return 'MAPPING_DUPLICATE_PATTERN'
    patterns.add(pattern)
    if (/\s/.test(pattern) || /[?\[\]]/.test(pattern) || pattern === '*') return 'MAPPING_PATTERN_INVALID'
    if (!isModelInProviderFamily(pattern, provider)) return 'MAPPING_SOURCE_PROVIDER_INVALID'
    if (!isModelInProviderFamily(target, provider)) return 'MAPPING_TARGET_PROVIDER_INVALID'
  }

  return null
}

export function isModelInProviderFamily(model: string, provider: string): boolean {
  const normalized = model.trim().toLowerCase()
  if (provider === 'anthropic') return normalized.startsWith('claude-')
  if (provider === 'openai') {
    return normalized.startsWith('gpt-') || /^o\d(?:-|$)/.test(normalized) || normalized.startsWith('chatgpt-') || normalized.startsWith('codex-')
  }
  return false
}

export function mappingToRows(mapping: Record<string, string> | undefined): Array<{ pattern: string; target: string }> {
  return Object.entries(mapping ?? {}).map(([pattern, target]) => ({ pattern, target }))
}

export function rowsToMapping(rows: Array<{ pattern: string; target: string }>): Record<string, string> {
  return rows.reduce<Record<string, string>>((mapping, row) => {
    const pattern = row.pattern.trim()
    const target = row.target.trim()
    if (pattern || target) mapping[pattern] = target
    return mapping
  }, {})
}

export function validateMappingRows(
  rows: Array<{ pattern: string; target: string }>,
  provider: string
): string | null {
  const normalizedPatterns = rows.map((row) => row.pattern.trim()).filter(Boolean)
  if (new Set(normalizedPatterns).size !== normalizedPatterns.length) {
    return 'MAPPING_DUPLICATE_PATTERN'
  }
  return validateModelMapping(rowsToMapping(rows), provider)
}
