/**
 * User Groups API endpoints (non-admin)
 * Handles group-related operations for regular users
 */

import { apiClient } from './client'
import type { Group } from '@/types'

/**
 * Get available groups that the current user can bind to API keys
 * This returns groups based on user's permissions:
 * - Standard groups: public (non-exclusive) or explicitly allowed
 * - Subscription groups: user has active subscription
 * @returns List of available groups
 */
export async function getAvailable(): Promise<Group[]> {
  const { data } = await apiClient.get<Group[]>('/groups/available')
  return data
}

/**
 * Get current user's custom group rate multipliers
 * @returns Map of group_id to custom rate_multiplier
 */
export async function getUserGroupRates(): Promise<Record<number, number>> {
  const { data } = await apiClient.get<Record<number, number> | null>('/groups/rates')
  return data || {}
}

export interface MarketplacePricing {
  billing_mode: 'token' | 'per_request' | 'image'
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
  image_output_price: number | null
  per_request_price: number | null
}

export interface MarketplaceModel {
  name: string
  platform: string
  pricing: MarketplacePricing | null
}

export interface MarketplaceGroup {
  id: number
  name: string
  platform: string
  rate_multiplier: number
  is_exclusive: boolean
  models: MarketplaceModel[]
}

export async function getModelMarketplace(): Promise<MarketplaceGroup[]> {
  const { data } = await apiClient.get<MarketplaceGroup[]>('/groups/model-marketplace')
  return data
}

export const userGroupsAPI = {
  getAvailable,
  getUserGroupRates,
  getModelMarketplace
}

export default userGroupsAPI
