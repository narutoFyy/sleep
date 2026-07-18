import { describe, expect, it } from 'vitest'
import type { Account } from '@/types'
import {
  STANDBY_ACCOUNT_DEFAULTS,
  hasPrimaryMembership,
  isProviderCompatible,
  mergeStandbyMembership,
  removeMembershipFromGroup,
  validateMappingRows
} from '../groupStandby'

function account(overrides: Partial<Account> = {}): Account {
  return {
    id: 10,
    name: 'standby-10',
    platform: 'anthropic',
    type: 'apikey',
    proxy_id: null,
    concurrency: 10,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '',
    updated_at: '',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  }
}

describe('group standby memberships', () => {
  it('preserves memberships in other groups when adding standby', () => {
    const source = account({
      account_groups: [
        { group_id: 1, role: 'primary', enabled: true, priority: 2 },
        { group_id: 2, role: 'standby', enabled: false, priority: 4, model_mapping: { 'claude-*': 'claude-haiku-4-5' } }
      ]
    })

    const result = mergeStandbyMembership(source, 3, {
      enabled: true,
      priority: 1,
      modelMapping: { 'claude-sonnet-*': 'claude-sonnet-4-5' }
    })

    expect(result).toHaveLength(3)
    expect(result[0]).toEqual(source.account_groups?.[0])
    expect(result[1]).toEqual(source.account_groups?.[1])
    expect(result[2]).toMatchObject({ group_id: 3, role: 'standby', enabled: true, priority: 1 })
  })

  it('refuses to silently convert a primary membership', () => {
    const source = account({ group_ids: [7] })
    expect(hasPrimaryMembership(source, 7)).toBe(true)
    expect(() => mergeStandbyMembership(source, 7, {
      enabled: true,
      priority: 1,
      modelMapping: { 'claude-*': 'claude-sonnet-4-5' }
    })).toThrow('PRIMARY_MEMBERSHIP_CONFLICT')
  })

  it('removes only the selected group membership', () => {
    const source = account({
      account_groups: [
        { group_id: 4, role: 'standby', enabled: true },
        { group_id: 5, role: 'standby', enabled: true }
      ]
    })
    expect(removeMembershipFromGroup(source, 4)).toEqual([
      { group_id: 5, role: 'standby', enabled: true }
    ])
  })
})

describe('group standby validation', () => {
  it('filters accounts by exact provider', () => {
    expect(isProviderCompatible(account({ platform: 'anthropic' }), 'anthropic')).toBe(true)
    expect(isProviderCompatible(account({ platform: 'openai' }), 'anthropic')).toBe(false)
  })

  it('accepts exact and wildcard mappings within a provider family', () => {
    expect(validateMappingRows([
      { pattern: 'claude-sonnet-*', target: 'claude-sonnet-4-5' },
      { pattern: 'claude-opus-4-1', target: 'claude-opus-4-1' }
    ], 'anthropic')).toBeNull()
    expect(validateMappingRows([
      { pattern: 'gpt-*', target: 'gpt-5.4' },
      { pattern: 'o3-*', target: 'o3-mini' }
    ], 'openai')).toBeNull()
  })

  it('rejects duplicate and cross-provider mappings', () => {
    expect(validateMappingRows([
      { pattern: 'claude-*', target: 'claude-sonnet-4-5' },
      { pattern: 'claude-*', target: 'claude-haiku-4-5' }
    ], 'anthropic')).toBe('MAPPING_DUPLICATE_PATTERN')
    expect(validateMappingRows([
      { pattern: 'claude-*', target: 'gpt-5.4' }
    ], 'anthropic')).toBe('MAPPING_TARGET_PROVIDER_INVALID')
    expect(validateMappingRows([
      { pattern: 'gpt-*', target: 'claude-sonnet-4-5' }
    ], 'anthropic')).toBe('MAPPING_SOURCE_PROVIDER_INVALID')
  })

  it('keeps the requested standby account defaults', () => {
    expect(STANDBY_ACCOUNT_DEFAULTS).toEqual({ concurrency: 50, baseRpm: 100, quota: null })
  })
})
