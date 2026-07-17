<template>
  <AppLayout>
    <div class="image-studio-page">
    <section class="studio-hero">
      <div class="studio-hero-copy">
        <p class="studio-alert">文生图 · 图生图 · 灵感广场 · 多图参考</p>
        <span class="eyebrow">Stone Image Studio</span>
        <h1>石头生图，<br /><span>把想法快速变成可用画面。</span></h1>
        <p>面向设计、运营、内容创作和产品展示场景，提供文生图、图生图、灵感复用和最近生成记录。</p>
        <div class="studio-metrics" aria-label="生图能力指标">
          <div class="studio-metric"><strong>手动</strong><span>输入 Key</span></div>
          <div class="studio-metric"><strong>2</strong><span>生成模式</span></div>
          <div class="studio-metric"><strong>{{ ratioOptions.length }}</strong><span>尺寸</span></div>
          <div class="studio-metric"><strong>{{ recentResults.length }}</strong><span>最近结果</span></div>
        </div>
        <div class="hero-actions">
          <a class="landing-primary" href="#create">开始创作</a>
          <a class="ghost-action" href="#workflow">查看流程</a>
        </div>
      </div>
      <div class="creative-console">
        <div class="console-top"><span></span><span></span><span></span><strong>Stone Image Console</strong></div>
        <div class="console-screen">
          <div class="console-placeholder">
            <span class="console-tag left">AI IMAGE</span>
            <span class="console-tag right">TEXT / IMAGE TO IMAGE</span>
            <div class="console-caption"><span>提示词 · 质量 · 尺寸</span><strong>高清输出</strong></div>
          </div>
        </div>
      </div>
    </section>

    <section id="power" class="studio-section">
      <div class="section-heading"><span class="eyebrow">Core Power</span><h2>从灵感到成图，一套完整的创作台。</h2></div>
      <div class="studio-feature-grid">
        <article class="studio-feature-card"><div><span>4K</span></div><h3>高清画面输出</h3><p>支持常见比例，适合海报、封面、产品图和运营配图。</p></article>
        <article class="studio-feature-card"><div><span>Prompt</span></div><h3>提示词工作流</h3><p>围绕提示词创作、复用和收藏设计，让灵感变成可反复使用的素材。</p></article>
        <article class="studio-feature-card"><div><span>Edit</span></div><h3>图生图二次创作</h3><p>上传参考图后重新描述画面，直接调用编辑接口生成新的创意版本。</p></article>
        <article class="studio-feature-card"><div><span>Control</span></div><h3>质量参数可控</h3><p>尺寸、质量、模式和历史记录集中管理，适合反复试稿和持续优化。</p></article>
      </div>
    </section>

    <section id="workflow" class="studio-section">
      <div class="section-heading"><span class="eyebrow">Workflow</span><h2>文生图、图生图、灵感复用，走一条顺手的创作流程。</h2></div>
      <div class="workflow-grid">
        <article class="workflow-card"><span>文生图</span><h3>从一句描述生成完整画面</h3><p>输入主题、风格、镜头、光线和用途，快速得到可继续调整的初稿。</p></article>
        <article class="workflow-card"><span>图生图</span><h3>基于参考图做二次创作</h3><p>上传参考图后保留主体结构，再用提示词重塑风格、材质和构图。</p></article>
        <article class="workflow-card"><span>质量</span><h3>多档质量选择</h3><p>兼顾速度和画面完成度，适合草稿、预览和精修。</p></article>
        <article class="workflow-card"><span>沉淀</span><h3>灵感和作品统一管理</h3><p>灵感广场、生成记录和提示词复用放在一起，减少重复整理成本。</p></article>
      </div>
    </section>

    <section id="create" class="studio-section create-section">
      <div class="section-heading"><span class="eyebrow">Create Now</span><h2>在这里，把想法变成画面。</h2></div>
      <div class="image-generator-shell">
        <section class="generator-panel">
          <div class="mode-switch">
            <button type="button" :class="{ active: generationMode === 'text' }" @click="generationMode = 'text'">
              <strong>文生图</strong>
              <small>输入描述生成图片</small>
            </button>
            <button type="button" :class="{ active: generationMode === 'image' }" @click="generationMode = 'image'">
              <strong>图生图</strong>
              <small>上传参考图后二次创作</small>
            </button>
          </div>

          <div v-if="generationMode === 'image'" class="reference-panel">
            <div class="prompt-head">
              <label class="field-label" for="reference-image">参考图</label>
              <span>{{ referenceImageName || '未上传' }}</span>
            </div>
            <p class="prompt-guide">支持 PNG / JPG / WEBP，建议用主体清晰、构图明确的参考图。</p>
            <div class="reference-shell">
              <label class="upload-dropzone" for="reference-image">
                <input id="reference-image" class="sr-only" type="file" accept="image/png,image/jpeg,image/webp" @change="handleReferenceImageChange" />
                <template v-if="referencePreviewUrl">
                  <img :src="referencePreviewUrl" alt="reference-preview" class="reference-preview" />
                </template>
                <template v-else>
                  <div class="upload-placeholder">
                    <strong>点击上传参考图</strong>
                    <span>用于图生图二次创作</span>
                  </div>
                </template>
              </label>
              <div class="reference-actions">
                <button type="button" class="ghost-action small-action" @click="triggerReferencePicker">重新选择</button>
                <button type="button" class="ghost-action small-action" :disabled="!referenceImageFile" @click="clearReferenceImage">清除图片</button>
              </div>
            </div>
          </div>

          <div class="prompt-head"><label class="field-label" for="image-prompt">提示词</label><span>{{ prompt.length }} 字</span></div>
          <p class="prompt-guide">{{ generationMode === 'text' ? '建议写明主体、风格、镜头、光线、比例和用途。' : '描述你希望在参考图基础上怎么改：风格、材质、背景、光线、细节。' }}</p>
          <textarea id="image-prompt" v-model="prompt" class="input prompt-input" rows="5" :placeholder="promptPlaceholder" />
          <div class="quick-prompts"><button v-for="tag in quickPrompts" :key="tag" type="button" @click="appendPrompt(tag)">{{ tag }}</button></div>

          <label class="field-label field-block"><span>画面参数</span><small>手动 Key / 模型 / 比例 / 数量 / 质量</small></label>
          <div class="generation-console">
            <div class="generation-control-note studio-control-status">
              <span>当前 Key：<strong>{{ selectedKeyLabel }}</strong></span>
              <span>宽高比：<strong>{{ selectedRatio }}</strong></span>
            </div>
            <p class="model-resolution-note">这里按你的要求改成用户手动输入 Key，不走站内 Key 列表选择。</p>
            <div class="generation-toolbar form-grid">
              <label class="field-stack field-span-2">
                <span>API Key</span>
                <input v-model.trim="manualApiKey" class="input select-input" type="password" placeholder="请输入你自己的生图 API Key" />
              </label>
              <label class="field-stack">
                <span>模型</span>
                <select v-model="selectedModel" class="input select-input">
                  <option v-for="model in modelOptions" :key="model.value" :value="model.value">{{ model.label }}</option>
                </select>
              </label>
              <label class="field-stack">
                <span>比例</span>
                <select v-model="selectedRatio" class="input select-input">
                  <option v-for="ratio in ratioOptions" :key="ratio.value" :value="ratio.value">{{ ratio.label }}</option>
                </select>
              </label>
              <label class="field-stack">
                <span>张数</span>
                <select v-model.number="selectedCount" class="input select-input">
                  <option v-for="count in countOptions" :key="count" :value="count">{{ count }}张</option>
                </select>
              </label>
              <label class="field-stack">
                <span>质量</span>
                <select v-model="selectedQuality" class="input select-input">
                  <option v-for="quality in qualityOptions" :key="quality.value" :value="quality.value">{{ quality.label }}</option>
                </select>
              </label>
            </div>
          </div>

          <p class="input-hint">{{ selectedRatioHint }}</p>
          <p v-if="generationMode === 'image'" class="input-hint">图生图会调用 `/v1/images/edits`，提交前必须先上传参考图。</p>
          <p class="input-hint warning-hint">请确认这个 Key 对应分组已开启生图权限，否则接口会返回 403。</p>
          <p v-if="errorMessage" class="input-hint error-hint">{{ errorMessage }}</p>
          <div class="create-actions"><button type="button" class="generate-button" :disabled="generateDisabled" @click="handleGenerate">{{ generating ? '生成中…' : generationMode === 'text' ? '立即生成' : '开始图生图' }}</button></div>
        </section>

        <aside class="result-panel">
          <div class="result-header"><div><span class="eyebrow">OUTPUT</span><h2>生成结果</h2><p>{{ generationMode === 'text' ? '文生图' : '图生图' }} · {{ selectedRatio }}</p></div><span class="live-dot" aria-label="可用"></span></div>
          <div class="result-workspace">
            <div v-if="!resultImages.length" class="empty-workspace"><div class="empty-icon">4K</div><h3>等待你的第一张作品</h3><p>{{ generationMode === 'text' ? '输入你自己的 Key 后即可测试真实文生图。' : '上传参考图并输入要求后即可测试图生图。' }}</p></div>
            <div v-else class="result-grid">
              <article v-for="(image, index) in resultImages" :key="image.url" class="result-card">
                <img :src="image.url" :alt="`image-${index + 1}`" class="result-image" />
                <div class="result-card-actions">
                  <button type="button" class="ghost-action small-action" @click="openImage(image)">打开</button>
                  <button type="button" class="ghost-action small-action" @click="downloadImage(image, index)">下载</button>
                </div>
              </article>
            </div>
          </div>
          <div class="result-actions"><button type="button" class="ghost-action" @click="copyPrompt">复制提示词</button></div>
        </aside>
      </div>
    </section>

    <section class="studio-section inspiration-section">
      <div class="section-heading split"><div><span class="eyebrow">Inspiration</span><h2>灵感广场，为下一次创作准备好提示词。</h2></div></div>
      <div class="inspiration-grid">
        <article v-for="card in inspirationCards" :key="card.title" class="inspiration-card">
          <div>
            <div class="inspiration-title"><h3>{{ card.title }}</h3><button type="button" class="icon-button" @click="usePrompt(card.prompt)">使用</button></div>
            <p>{{ card.prompt }}</p>
            <div class="tag-list"><span v-for="tag in card.tags" :key="tag">{{ tag }}</span></div>
          </div>
        </article>
      </div>
    </section>

    <section id="history" class="studio-section history-section">
      <div class="section-heading"><span class="eyebrow">History</span><h2>最近生成</h2><p>仅在当前浏览器本地保留最近结果，默认 1 小时后自动删除，避免长期保存客户隐私内容。</p></div>
      <div class="history-panel">
        <div v-if="!recentResults.length" class="history-empty">暂无生成记录。</div>
        <div v-else class="history-grid">
          <button v-for="item in recentResults" :key="item.id" type="button" class="history-card" @click="restoreHistory(item)">
            <img v-if="item.images[0]?.url" :src="item.images[0].url" alt="history-preview" class="history-thumb" />
            <div class="history-copy"><strong>{{ item.model }}</strong><span>{{ item.mode === 'image' ? '图生图' : '文生图' }} · {{ item.ratio }} · {{ item.count }}张</span><p>{{ item.prompt }}</p></div>
          </button>
        </div>
      </div>
    </section>

    <footer class="studio-footer"><span>石头生图 · 专业提示词生图控制台</span><span>{{ selectedModel }} · {{ selectedRatio }}</span></footer>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { imageAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'

type GenerationMode = 'text' | 'image'
type ImageQuality = 'auto' | 'low' | 'medium' | 'high'

interface ModelOption {
  value: string
  label: string
}
interface RatioOption {
  value: string
  label: string
  size: string
  hint: string
}
interface ResultImage {
  url: string
  sourceUrl?: string
  objectUrl?: boolean
}
interface HistoryItem {
  id: string
  prompt: string
  model: string
  ratio: string
  count: number
  quality: string
  keyLabel: string
  keyValue: string
  images: ResultImage[]
  createdAt: number
  mode: GenerationMode
}
interface PersistedHistoryPayload {
  version: number
  items: HistoryItem[]
}

const HISTORY_STORAGE_KEY = 'image-studio-recent-results'
const HISTORY_RETENTION_MS = 60 * 60 * 1000
const HISTORY_STORAGE_VERSION = 3

const generationMode = ref<GenerationMode>('text')
const prompt = ref('')
const manualApiKey = ref('')
const generating = ref(false)
const errorMessage = ref('')
const selectedModel = ref('gpt-image-2')
const selectedRatio = ref('16:9')
const selectedCount = ref(1)
const selectedQuality = ref<ImageQuality>('auto')
const resultImages = ref<ResultImage[]>([])
const recentResults = ref<HistoryItem[]>([])
const referenceImageFile = ref<File | null>(null)
const referencePreviewUrl = ref('')
const referenceImageName = ref('')

const quickPrompts = ['电影级', '赛博朋克', '产品摄影', '国风幻想', '3D 潮玩']
const countOptions = [1, 2, 3, 4]
const qualityOptions = [
  { value: 'auto', label: 'Auto' },
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
] as const
const modelOptions: ModelOption[] = [
  { value: 'gpt-image-2', label: '1K 标准 · gpt-image-2' },
  { value: 'gpt-image-1.5', label: '高清 · gpt-image-1.5' },
  { value: 'gpt-image-1', label: '兼容 · gpt-image-1' },
]
const ratioOptions: RatioOption[] = [
  { value: '1:1', label: '1:1 方图', size: '1024x1024', hint: '适合头像、社媒配图和商品图。' },
  { value: '16:9', label: '16:9 横版', size: '1536x1024', hint: '适合海报、封面和产品展示。' },
  { value: '9:16', label: '9:16 竖版', size: '1024x1536', hint: '适合短视频封面和手机海报。' },
  { value: '4:5', label: '4:5 竖图', size: '1024x1280', hint: '适合商品详情、人物和社媒内容。' },
]
const inspirationCards = [
  { title: 'Neon Archive', prompt: '赛博朋克夜景档案馆，霓虹灯反射在玻璃和金属表面，电影级光影，高细节，适合横版封面。', tags: ['赛博朋克', '横版', '电影感'] },
  { title: 'Orbital Forge', prompt: '未来轨道空间站里的产品展示台，冷色科技光，干净构图，高清商业视觉，适合产品主图。', tags: ['科技', '产品', '空间'] },
  { title: 'Android Muse', prompt: '未来感仿生人肖像，柔和面部光线，玻璃材质和蓝紫色反射，超清细节，竖版头像海报。', tags: ['肖像', '竖版', '未来感'] },
]

const selectedRatioOption = computed(() => ratioOptions.find((item) => item.value === selectedRatio.value) ?? ratioOptions[0])
const selectedRatioHint = computed(() => selectedRatioOption.value.hint)
const selectedKeyLabel = computed(() => (manualApiKey.value ? `已输入 (${manualApiKey.value.slice(0, 6)}...)` : '未输入 Key'))
const promptPlaceholder = computed(() => generationMode.value === 'text'
  ? '例如：一座漂浮在海上的未来能源塔，玻璃穹顶、金色晨光、电影级构图、霓虹城市、梦幻而真实，超清细节，16:9 比例，高级质感'
  : '例如：保留主体轮廓，把画面改成赛博朋克夜景风格，增加霓虹灯、雨夜倒影、电影级光影和高细节质感')
const generateDisabled = computed(() => {
  if (generating.value || !prompt.value.trim() || !manualApiKey.value.trim()) return true
  return generationMode.value === 'image' && !referenceImageFile.value
})

function revokeImageObjectUrl(image: ResultImage) {
  if (image.objectUrl && image.url.startsWith('blob:')) {
    URL.revokeObjectURL(image.url)
  }
}
function revokeImageObjectUrls(images: ResultImage[]) {
  images.forEach(revokeImageObjectUrl)
}
async function localizeImageResult(image: ResultImage): Promise<ResultImage> {
  if (image.url.startsWith('data:')) return image
  if (image.sourceUrl) {
    const blob = await imageAPI.proxyImage(image.sourceUrl)
    return {
      ...image,
      url: URL.createObjectURL(blob),
      objectUrl: true,
    }
  }
  if (image.url.startsWith('blob:')) return image
  return image
}
async function localizeImageResults(images: ResultImage[]): Promise<ResultImage[]> {
  const localized: ResultImage[] = []
  try {
    for (const image of images) {
      localized.push(await localizeImageResult(image))
    }
    return localized
  } catch (error) {
    revokeImageObjectUrls(localized)
    throw error
  }
}
function pruneExpiredHistory(items: HistoryItem[], now = Date.now()) {
  return items.filter((item) => now - item.createdAt < HISTORY_RETENTION_MS)
}
function persistHistory(items: HistoryItem[]) {
  if (typeof window === 'undefined') return
  const payload: PersistedHistoryPayload = {
    version: HISTORY_STORAGE_VERSION,
    items,
  }
  window.localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(payload))
}
function loadPersistedHistory() {
  if (typeof window === 'undefined') return
  const raw = window.localStorage.getItem(HISTORY_STORAGE_KEY)
  if (!raw) return

  try {
    const parsed = JSON.parse(raw) as PersistedHistoryPayload | HistoryItem[]
    const items = Array.isArray(parsed) ? parsed : parsed.items
    if (!Array.isArray(items)) {
      window.localStorage.removeItem(HISTORY_STORAGE_KEY)
      return
    }
    const validItems = items.filter((item): item is HistoryItem => {
      return Boolean(
        item &&
          typeof item.id === 'string' &&
          typeof item.prompt === 'string' &&
          typeof item.model === 'string' &&
          typeof item.ratio === 'string' &&
          typeof item.count === 'number' &&
          typeof item.quality === 'string' &&
          typeof item.keyLabel === 'string' &&
          typeof item.keyValue === 'string' &&
          typeof item.createdAt === 'number' &&
          (item.mode === 'text' || item.mode === 'image') &&
          Array.isArray(item.images),
      )
    })
    const recoverableItems = validItems
      .map((item) => ({
        ...item,
        images: item.images.filter((image) => image.url.startsWith('data:') || Boolean(image.sourceUrl)),
      }))
      .filter((item) => item.images.length > 0)
    const prunedItems = pruneExpiredHistory(recoverableItems)
    recentResults.value = prunedItems.slice(0, 10)
    persistHistory(recentResults.value)
  } catch {
    window.localStorage.removeItem(HISTORY_STORAGE_KEY)
  }
}

watch(
  recentResults,
  (items) => {
    persistHistory(pruneExpiredHistory(items).slice(0, 10))
  },
  { deep: true },
)

watch(generationMode, () => {
  revokeImageObjectUrls(resultImages.value)
  resultImages.value = []
  errorMessage.value = ''
})

onMounted(() => {
  loadPersistedHistory()
})

onBeforeUnmount(() => {
  revokeImageObjectUrls(resultImages.value)
  recentResults.value.forEach((item) => revokeImageObjectUrls(item.images))
  if (referencePreviewUrl.value) {
    URL.revokeObjectURL(referencePreviewUrl.value)
  }
})

function appendPrompt(tag: string) {
  prompt.value = prompt.value ? `${prompt.value}，${tag}` : tag
}
function usePrompt(value: string) {
  prompt.value = value
  window.location.hash = '#create'
}
async function copyPrompt() {
  if (!prompt.value) return
  await navigator.clipboard.writeText(prompt.value)
}
function triggerReferencePicker() {
  document.getElementById('reference-image')?.click()
}
function clearReferenceImage() {
  if (referencePreviewUrl.value) {
    URL.revokeObjectURL(referencePreviewUrl.value)
  }
  referenceImageFile.value = null
  referencePreviewUrl.value = ''
  referenceImageName.value = ''
}
function handleReferenceImageChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (referencePreviewUrl.value) {
    URL.revokeObjectURL(referencePreviewUrl.value)
  }
  referenceImageFile.value = file
  referenceImageName.value = file.name
  referencePreviewUrl.value = URL.createObjectURL(file)
  errorMessage.value = ''
}
function isLocalImageUrl(url: string) {
  return url.startsWith('blob:') || url.startsWith('data:')
}
async function getFreshLocalImageUrl(image: ResultImage): Promise<{ url: string; revoke: boolean }> {
  if (image.sourceUrl) {
    const blob = await imageAPI.proxyImage(image.sourceUrl)
    return { url: URL.createObjectURL(blob), revoke: true }
  }
  if (isLocalImageUrl(image.url)) return { url: image.url, revoke: false }
  throw new Error('Image URL is unavailable. Please generate it again.')
}
async function openImage(image: ResultImage) {
  try {
    const localImage = await getFreshLocalImageUrl(image)
    window.open(localImage.url, '_blank', 'noopener,noreferrer')
    if (localImage.revoke) {
      window.setTimeout(() => URL.revokeObjectURL(localImage.url), 60 * 1000)
    }
  } catch (error: any) {
    errorMessage.value = error?.message || 'Open failed. Please generate the image again.'
  }
}
function triggerImageDownload(url: string, index: number) {
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `image-${Date.now()}-${index + 1}.png`
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}
async function downloadImage(image: ResultImage, index: number) {
  try {
    const localImage = await getFreshLocalImageUrl(image)
    triggerImageDownload(localImage.url, index)
    if (localImage.revoke) {
      window.setTimeout(() => URL.revokeObjectURL(localImage.url), 60 * 1000)
    }
  } catch (error: any) {
    errorMessage.value = error?.message || 'Download failed. Please try again.'
  }
}
function convertImageResult(item: { url?: string; b64_json?: string }): ResultImage | null {
  if (item.url) return { url: item.url, sourceUrl: item.url }
  if (item.b64_json) return { url: `data:image/png;base64,${item.b64_json}` }
  return null
}
function resolveImageErrorMessage(error: any, mode: GenerationMode): string {
  const responseBody = error?.responseBody
  const nestedMessage = responseBody?.error?.message || responseBody?.message
  const status = error?.status ?? error?.response?.status
  const rawMessage = nestedMessage || error?.message || ''

  if (status === 404) {
    if (rawMessage.includes('Images API is not supported for this platform')) {
      return mode === 'image'
        ? '当前这个 Key 所在分组不是 OpenAI 生图分组，或该分组不支持图生图接口 /images/edits。请更换支持 OpenAI Images 的 Key 再试。'
        : '当前这个 Key 所在分组不支持 OpenAI 生图接口 /images/generations。请更换支持 OpenAI Images 的 Key 再试。'
    }
    return mode === 'image' ? '图生图接口返回 404：当前 Key、分组或上游路由未提供 /images/edits。' : '生图接口返回 404：当前 Key、分组或上游路由未提供 /images/generations。'
  }

  return rawMessage || (mode === 'text' ? '生成失败，请稍后重试。' : '图生图失败，请稍后重试。')
}
function pushHistory(images: ResultImage[]) {
  recentResults.value = pruneExpiredHistory([
    {
      id: `${Date.now()}`,
      prompt: prompt.value,
      model: selectedModel.value,
      ratio: selectedRatio.value,
      count: selectedCount.value,
      quality: selectedQuality.value,
      keyLabel: selectedKeyLabel.value,
      keyValue: manualApiKey.value,
      images: images.map((image) => image.sourceUrl
        ? { url: image.sourceUrl, sourceUrl: image.sourceUrl }
        : { url: image.url }),
      createdAt: Date.now(),
      mode: generationMode.value,
    },
    ...recentResults.value,
  ]).slice(0, 10)
}
async function restoreHistory(item: HistoryItem) {
  prompt.value = item.prompt
  selectedModel.value = item.model
  selectedRatio.value = item.ratio
  selectedCount.value = item.count
  selectedQuality.value = item.quality as ImageQuality
  manualApiKey.value = item.keyValue
  revokeImageObjectUrls(resultImages.value)
  resultImages.value = await localizeImageResults(item.images)
  generationMode.value = item.mode ?? 'text'
  window.location.hash = '#create'
}
async function handleGenerate() {
  if (!manualApiKey.value.trim() || !prompt.value.trim()) return
  if (generationMode.value === 'image' && !referenceImageFile.value) {
    errorMessage.value = '请先上传参考图。'
    return
  }

  generating.value = true
  errorMessage.value = ''
  try {
    const response = generationMode.value === 'text'
      ? await imageAPI.generateImage(manualApiKey.value.trim(), {
          prompt: prompt.value.trim(),
          model: selectedModel.value,
          size: selectedRatioOption.value.size,
          quality: selectedQuality.value,
          n: selectedCount.value,
        })
      : await imageAPI.editImage(manualApiKey.value.trim(), {
          prompt: prompt.value.trim(),
          model: selectedModel.value,
          image: referenceImageFile.value as File,
          size: selectedRatioOption.value.size,
          quality: selectedQuality.value,
          n: selectedCount.value,
        })

    const sourceImages = response.data.map(convertImageResult).filter((item): item is ResultImage => Boolean(item))
    const images = await localizeImageResults(sourceImages)
    revokeImageObjectUrls(resultImages.value)
    resultImages.value = images
    pushHistory(images)
    if (!images.length) errorMessage.value = '接口已返回，但没有拿到图片结果。'
  } catch (error: any) {
    errorMessage.value = resolveImageErrorMessage(error, generationMode.value)
  } finally {
    generating.value = false
  }
}
</script>

<style scoped>
.image-studio-page {
  color: #0f172a;
}
.studio-hero,
.studio-section,
.studio-footer {
  max-width: 1400px;
  margin: 0 auto;
}
.landing-primary,
.ghost-action {
  text-decoration: none;
}
.landing-primary,
.generate-button {
  border: 1px solid rgba(20, 184, 166, 0.28);
  border-radius: 9999px;
  padding: .85rem 1.4rem;
  font-weight: 800;
  color: #ffffff;
  background: linear-gradient(135deg, #22d3ee, #14b8a6 54%, #0f9f8f);
  box-shadow: 0 16px 34px rgba(20, 184, 166, 0.22);
  transition: transform .2s ease, box-shadow .2s ease, filter .2s ease;
}
.landing-primary:hover,
.generate-button:hover:not([disabled]) {
  transform: translateY(-1px);
  filter: saturate(1.04);
  box-shadow: 0 22px 44px rgba(20, 184, 166, 0.26);
}
.ghost-action {
  border: 1px solid rgba(20, 184, 166, 0.34);
  border-radius: 9999px;
  padding: .8rem 1.2rem;
  color: #0f766e;
  background: rgba(255, 255, 255, 0.86);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.06);
  transition: transform .2s ease, border-color .2s ease, background .2s ease;
}
.ghost-action:hover:not([disabled]) {
  transform: translateY(-1px);
  border-color: rgba(20, 184, 166, 0.56);
  background: #ffffff;
}
.small-action {
  padding: .5rem .9rem;
  font-size: .875rem;
}
.studio-hero {
  display: grid;
  grid-template-columns: 1.15fr .85fr;
  gap: 1.5rem;
}
.studio-hero-copy,
.creative-console,
.studio-section,
.generator-panel,
.result-panel,
.workflow-card,
.studio-feature-card,
.inspiration-card,
.history-panel {
  border: 1px solid rgba(45, 212, 191, 0.48);
  background: rgba(255, 255, 255, 0.78);
  backdrop-filter: blur(18px);
  box-shadow: 0 18px 44px rgba(14, 165, 233, 0.08), 0 10px 30px rgba(15, 23, 42, 0.05);
}
.studio-hero-copy,
.creative-console,
.studio-section,
.generator-panel,
.result-panel,
.inspiration-card,
.history-panel,
.workflow-card,
.studio-feature-card {
  border-radius: 1.5rem;
}
.studio-hero-copy,
.creative-console {
  padding: 1.75rem;
}
.eyebrow,
.studio-alert,
.console-tag,
.studio-metric span {
  font-size: .8rem;
  letter-spacing: .08em;
  text-transform: uppercase;
}
.eyebrow,
.studio-alert,
.console-tag {
  color: #0f766e;
  font-weight: 800;
}
.studio-alert {
  margin: 0 0 .35rem;
}
.studio-hero h1 {
  margin: .75rem 0 1rem;
  color: #0f172a;
  font-size: clamp(2.2rem, 4vw, 4rem);
  line-height: 1.06;
  letter-spacing: -.045em;
}
.studio-hero h1 span {
  color: #1f2937;
}
.studio-hero p,
.workflow-card p,
.studio-feature-card p,
.inspiration-card p,
.section-heading p,
.input-hint,
.prompt-guide,
.model-resolution-note,
.empty-workspace p,
.history-empty {
  color: #475569;
  line-height: 1.75;
}
.studio-metrics,
.hero-actions,
.generation-toolbar,
.quick-prompts,
.result-actions,
.tag-list,
.result-card-actions,
.reference-actions {
  display: flex;
  gap: .75rem;
  flex-wrap: wrap;
}
.studio-metrics {
  margin: 1.25rem 0;
}
.studio-metric,
.studio-chip {
  border: 1px solid rgba(45, 212, 191, 0.32);
  border-radius: 1rem;
  padding: .75rem 1rem;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.04);
}
.studio-metric strong {
  display: block;
  color: #0f172a;
}
.studio-metric span {
  color: #64748b;
}
.console-top {
  display: flex;
  align-items: center;
  gap: .5rem;
  color: #334155;
}
.console-top span {
  width: .75rem;
  height: .75rem;
  border-radius: 9999px;
  background: rgba(20, 184, 166, 0.32);
}
.console-screen {
  margin-top: 1rem;
  min-height: 320px;
  border-radius: 1.25rem;
  background: linear-gradient(135deg, rgba(236, 254, 255, 0.92), rgba(255, 255, 255, 0.82));
  padding: 1rem;
}
.console-placeholder,
.result-workspace {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 280px;
  border-radius: 1.15rem;
  border: 1px dashed rgba(20, 184, 166, 0.36);
  background: rgba(255, 255, 255, 0.72);
}
.console-tag {
  position: absolute;
  top: 1rem;
  padding: .35rem .7rem;
  border-radius: 9999px;
  background: rgba(240, 253, 250, 0.92);
}
.console-tag.right { right: 1rem; }
.console-tag.left { left: 1rem; }
.console-caption {
  text-align: center;
  color: #334155;
}
.console-caption strong {
  color: #0f172a;
}
.studio-section {
  margin-top: 1.5rem;
  padding: 1.75rem;
}
.section-heading.split {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
}
.section-heading h2,
.result-header h2,
.inspiration-title h3,
.workflow-card h3,
.studio-feature-card h3,
.empty-workspace h3 {
  color: #0f172a;
}
.studio-feature-grid,
.workflow-grid,
.inspiration-grid,
.history-grid {
  display: grid;
  gap: 1rem;
}
.studio-feature-grid,
.workflow-grid {
  grid-template-columns: repeat(4,minmax(0,1fr));
}
.inspiration-grid,
.history-grid {
  grid-template-columns: repeat(3,minmax(0,1fr));
}
.studio-feature-card,
.workflow-card,
.inspiration-card {
  padding: 1.25rem;
}
.studio-feature-card div:first-child,
.workflow-card span,
.tag-list span {
  display: inline-flex;
  width: fit-content;
  border: 1px solid rgba(20, 184, 166, 0.24);
  border-radius: 9999px;
  padding: .32rem .68rem;
  color: #0f766e;
  background: rgba(240, 253, 250, 0.86);
  font-size: .82rem;
  font-weight: 800;
}
.studio-feature-card div:first-child {
  margin-bottom: .8rem;
}
.image-generator-shell {
  display: grid;
  grid-template-columns: minmax(0,1.1fr) minmax(320px,.9fr);
  gap: 1rem;
}
.generator-panel,
.result-panel {
  padding: 1.5rem;
}
.mode-switch {
  display: grid;
  grid-template-columns: repeat(2,minmax(0,1fr));
  gap: .75rem;
}
.mode-switch button,
.quick-prompts button,
.result-actions button,
.icon-button {
  border: 1px solid rgba(20, 184, 166, 0.26);
  border-radius: 1rem;
  color: #334155;
  background: rgba(255, 255, 255, 0.82);
}
.mode-switch button {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: .25rem;
  padding: 1rem;
}
.mode-switch .active {
  color: #ffffff;
  background: linear-gradient(135deg, #22d3ee, #14b8a6 56%, #0f9f8f);
  border-color: rgba(20, 184, 166, 0.6);
  box-shadow: 0 12px 26px rgba(20, 184, 166, 0.22);
}
.mode-switch .active small {
  color: rgba(255, 255, 255, 0.82);
}
.prompt-head,
.result-header,
.inspiration-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
.field-label {
  display: flex;
  flex-direction: column;
  gap: .25rem;
  font-weight: 800;
  color: #0f172a;
}
.field-block { margin-top: 1rem; }
.field-stack {
  display: flex;
  flex-direction: column;
  gap: .45rem;
  min-width: 0;
}
.field-stack span {
  color: #0f172a;
  font-size: .9rem;
  font-weight: 800;
}
.field-span-2 { grid-column: span 2; }
.form-grid {
  display: grid;
  grid-template-columns: repeat(2,minmax(0,1fr));
  gap: .85rem;
}
.input {
  width: 100%;
  margin-top: .75rem;
  border: 1px solid rgba(20, 184, 166, 0.26);
  border-radius: 1.15rem;
  padding: 1rem;
  color: #0f172a;
  background: rgba(255, 255, 255, 0.9);
  outline: none;
  transition: border-color .2s ease, box-shadow .2s ease, background .2s ease;
}
.input:focus {
  border-color: rgba(20, 184, 166, 0.65);
  box-shadow: 0 0 0 4px rgba(20, 184, 166, 0.12);
  background: #ffffff;
}
.input::placeholder {
  color: #94a3b8;
}
.select-input { margin-top: 0; }
.quick-prompts { margin-top: .75rem; }
.quick-prompts button,
.result-actions button,
.icon-button { padding: .65rem .9rem; }
.generation-console,
.history-panel,
.reference-panel { margin-top: 1rem; }
.generation-control-note {
  display: flex;
  flex-wrap: wrap;
  gap: .75rem;
  margin-bottom: .75rem;
  color: #475569;
}
.generation-control-note strong {
  color: #0f172a;
}
.reference-shell {
  display: grid;
  gap: .75rem;
}
.upload-dropzone {
  display: flex;
  min-height: 240px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px dashed rgba(20, 184, 166, 0.4);
  border-radius: 1.15rem;
  background: rgba(255, 255, 255, 0.76);
  cursor: pointer;
}
.upload-placeholder {
  display: flex;
  flex-direction: column;
  gap: .4rem;
  text-align: center;
  color: #64748b;
}
.upload-placeholder strong {
  color: #0f172a;
}
.reference-preview {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.create-actions { margin-top: 1rem; }
.generate-button[disabled],
.ghost-action[disabled] {
  opacity: .7;
  cursor: not-allowed;
}
.generate-button { width: 100%; }
.warning-hint { color: #d97706; }
.error-hint { color: #dc2626; }
.history-panel,
.result-panel { margin-top: 1rem; }
.empty-workspace,
.history-empty { text-align: center; }
.empty-icon {
  width: 5rem;
  height: 5rem;
  margin: 0 auto 1rem;
  border-radius: 1.35rem;
  display: grid;
  place-items: center;
  color: #0f766e;
  font-weight: 900;
  background: rgba(240, 253, 250, 0.92);
  border: 1px solid rgba(20, 184, 166, 0.26);
}
.live-dot {
  width: .75rem;
  height: .75rem;
  border-radius: 999px;
  background: #14b8a6;
  box-shadow: 0 0 0 8px rgba(20, 184, 166, 0.14);
}
.result-grid {
  display: grid;
  grid-template-columns: repeat(2,minmax(0,1fr));
  gap: .85rem;
  width: 100%;
}
.result-card {
  border-radius: 1rem;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.84);
  border: 1px solid rgba(20, 184, 166, 0.22);
}
.result-image,
.history-thumb {
  width: 100%;
  display: block;
  object-fit: cover;
}
.result-image { aspect-ratio: 1 / 1; }
.result-card-actions { padding: .75rem; }
.history-card {
  display: flex;
  flex-direction: column;
  border-radius: 1rem;
  overflow: hidden;
  border: 1px solid rgba(20, 184, 166, 0.22);
  background: rgba(255, 255, 255, 0.78);
  text-align: left;
}
.history-thumb { aspect-ratio: 4 / 3; }
.history-copy {
  display: flex;
  flex-direction: column;
  gap: .35rem;
  padding: .85rem;
}
.history-copy strong { color: #0f172a; }
.history-copy span { color: #0f766e; }
.history-copy p {
  margin: 0;
  color: #475569;
  font-size: .85rem;
  line-height: 1.45;
}
.studio-footer {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 1.5rem;
  padding: 1rem 0 2rem;
  color: #64748b;
}
@media (max-width: 1200px) {
  .studio-feature-grid,
  .workflow-grid { grid-template-columns: repeat(2,minmax(0,1fr)); }
  .inspiration-grid,
  .history-grid,
  .image-generator-shell,
  .studio-hero { grid-template-columns: 1fr; }
}
@media (max-width: 720px) {
  .section-heading.split,
  .prompt-head,
  .result-header,
  .studio-footer { flex-direction: column; align-items: flex-start; }
  .studio-feature-grid,
  .workflow-grid,
  .inspiration-grid,
  .history-grid,
  .mode-switch,
  .form-grid,
  .result-grid { grid-template-columns: 1fr; }
  .field-span-2 { grid-column: span 1; }
}
</style>
