import axios from 'axios'
import { apiClient } from './client'

export interface ImageGenerationRequest {
  prompt: string
  model: string
  size?: string
  quality?: 'auto' | 'low' | 'medium' | 'high'
  n?: number
}

export interface ImageEditRequest {
  prompt: string
  model: string
  image: File
  size?: string
  quality?: 'auto' | 'low' | 'medium' | 'high'
  n?: number
}

export interface ImageResultItem {
  b64_json?: string
  revised_prompt?: string
  url?: string
}

export interface ImageGenerationResponse {
  created: number
  data: ImageResultItem[]
}

export interface ImageRequestDebugInfo {
  endpoint: string
  hasBearer: boolean
  hasXApiKey: boolean
  model: string
  size?: string
  quality?: string
  count?: number
}

export class ImageRequestError extends Error {
  status?: number
  responseBody?: unknown
  debug?: ImageRequestDebugInfo

  constructor(message: string, options?: { status?: number; responseBody?: unknown; debug?: ImageRequestDebugInfo }) {
    super(message)
    this.name = 'ImageRequestError'
    this.status = options?.status
    this.responseBody = options?.responseBody
    this.debug = options?.debug
  }
}

export async function generateImage(
  apiKey: string,
  payload: ImageGenerationRequest,
  options?: { signal?: AbortSignal },
): Promise<ImageGenerationResponse> {
  const baseUrl = (apiClient.defaults.baseURL as string | undefined) || '/api/v1'
  const gatewayBase = baseUrl.endsWith('/api/v1') ? baseUrl.slice(0, -'/api/v1'.length) : baseUrl
  const endpoint = `${gatewayBase || ''}/v1/images/generations`
  const debug: ImageRequestDebugInfo = {
    endpoint,
    hasBearer: true,
    hasXApiKey: true,
    model: payload.model,
    size: payload.size,
    quality: payload.quality,
    count: payload.n,
  }

  try {
    const { data } = await axios.post<ImageGenerationResponse>(endpoint, payload, {
      signal: options?.signal,
      withCredentials: true,
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': apiKey,
        Authorization: `Bearer ${apiKey}`,
        'Accept-Language': apiClient.defaults.headers.common?.['Accept-Language'] as string | undefined,
      },
    })
    return data
  } catch (error: any) {
    const status = error?.response?.status as number | undefined
    const responseBody = error?.response?.data
    const message = responseBody?.message || error?.message || '生成失败，请稍后重试。'
    throw new ImageRequestError(message, { status, responseBody, debug })
  }
}

export async function editImage(
  apiKey: string,
  payload: ImageEditRequest,
  options?: { signal?: AbortSignal },
): Promise<ImageGenerationResponse> {
  const baseUrl = (apiClient.defaults.baseURL as string | undefined) || '/api/v1'
  const gatewayBase = baseUrl.endsWith('/api/v1') ? baseUrl.slice(0, -'/api/v1'.length) : baseUrl
  const endpoint = `${gatewayBase || ''}/v1/images/edits`
  const debug: ImageRequestDebugInfo = {
    endpoint,
    hasBearer: true,
    hasXApiKey: true,
    model: payload.model,
    size: payload.size,
    quality: payload.quality,
    count: payload.n,
  }

  const formData = new FormData()
  formData.append('prompt', payload.prompt)
  formData.append('model', payload.model)
  formData.append('image', payload.image)
  if (payload.size) formData.append('size', payload.size)
  if (payload.quality) formData.append('quality', payload.quality)
  if (typeof payload.n === 'number') formData.append('n', String(payload.n))

  try {
    const { data } = await axios.post<ImageGenerationResponse>(endpoint, formData, {
      signal: options?.signal,
      withCredentials: true,
      headers: {
        'X-API-Key': apiKey,
        Authorization: `Bearer ${apiKey}`,
        'Accept-Language': apiClient.defaults.headers.common?.['Accept-Language'] as string | undefined,
      },
    })
    return data
  } catch (error: any) {
    const status = error?.response?.status as number | undefined
    const responseBody = error?.response?.data
    const message = responseBody?.message || error?.message || '图生图失败，请稍后重试。'
    throw new ImageRequestError(message, { status, responseBody, debug })
  }
}

export async function proxyImage(sourceUrl: string, options?: { signal?: AbortSignal }): Promise<Blob> {
  const { data } = await apiClient.post<Blob>(
    '/user/image-proxy',
    { url: sourceUrl },
    {
      signal: options?.signal,
      responseType: 'blob',
      timeout: 60000,
    },
  )
  return data
}

export const imageAPI = {
  generateImage,
  editImage,
  proxyImage,
}

export default imageAPI
