<template>
  <div class="fixed inset-0 pointer-events-none bg-slate-50 dark:bg-[#030507] overflow-hidden transition-colors duration-500" style="z-index: -3;">
    <!-- Base Gradient -->
    <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_var(--tw-gradient-stops))] from-blue-50/50 via-slate-50 to-slate-100 dark:from-[#11151b] dark:via-[#080b10] dark:to-[#030507] opacity-90 transition-colors duration-500"></div>

    <!-- Animated Grid -->
    <div ref="gridRef" class="absolute inset-0 opacity-30"
      :class="isDark ? 'bg-grid-dark' : 'bg-grid-light'">
    </div>

    <!-- Canvas for Cyber Particles -->
    <canvas ref="canvasRef" class="absolute inset-0 w-full h-full"></canvas>

    <!-- Mouse Glow -->
    <div ref="glowRef" class="absolute w-[800px] h-[800px] -translate-x-1/2 -translate-y-1/2 rounded-full blur-[140px] pointer-events-none opacity-50 dark:opacity-40 bg-gradient-to-r from-blue-400/40 to-cyan-300/40 dark:from-blue-500/30 dark:to-cyan-400/30" style="top: 50%; left: 50%; will-change: transform;"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, computed } from 'vue'
import gsap from 'gsap'

const canvasRef = ref<HTMLCanvasElement | null>(null)
const glowRef = ref<HTMLElement | null>(null)
const gridRef = ref<HTMLElement | null>(null)

// Use a simple check if the document has 'dark' class, or you can use your store's theme
const isDark = computed(() => {
  return document.documentElement.classList.contains('dark')
})

let ctx: CanvasRenderingContext2D | null = null
let animationFrameId: number
let particles: Particle[] = []
let mouse = { x: -1000, y: -1000 } // initially off-screen
let isHovering = false

const darkColors = ['#60a5fa', '#22d3ee', '#818cf8', '#38bdf8']
const lightColors = ['#3b82f6', '#06b6d4', '#4f46e5', '#0ea5e9']

class Particle {
  x: number
  y: number
  radius: number
  colorIndex: number
  vx: number
  vy: number
  baseX: number
  baseY: number

  constructor(canvasWidth: number, canvasHeight: number) {
    this.x = Math.random() * canvasWidth
    this.y = Math.random() * canvasHeight
    this.baseX = this.x
    this.baseY = this.y
    this.radius = Math.random() * 2.0 + 1.0 // Increased size for visibility
    this.colorIndex = Math.floor(Math.random() * darkColors.length)
    this.vx = (Math.random() - 0.5) * 0.6 // Slightly faster
    this.vy = (Math.random() - 0.5) * 0.6
  }

  update(canvasWidth: number, canvasHeight: number) {
    this.x += this.vx
    this.y += this.vy

    if (this.x < 0 || this.x > canvasWidth) this.vx = -this.vx
    if (this.y < 0 || this.y > canvasHeight) this.vy = -this.vy

    const dx = mouse.x - this.x
    const dy = mouse.y - this.y
    const distance = Math.sqrt(dx * dx + dy * dy)

    if (distance < 220 && isHovering) {
      const force = (220 - distance) / 220
      this.x -= (dx / distance) * force * 2.0
      this.y -= (dy / distance) * force * 2.0
    }
  }

  draw(ctx: CanvasRenderingContext2D, isDarkMode: boolean) {
    ctx.beginPath()
    ctx.arc(this.x, this.y, this.radius, 0, Math.PI * 2)
    ctx.fillStyle = isDarkMode ? darkColors[this.colorIndex] : lightColors[this.colorIndex]
    // Add subtle glow to particles themselves
    ctx.shadowBlur = 8;
    ctx.shadowColor = ctx.fillStyle;
    ctx.fill()
    ctx.shadowBlur = 0; // reset for lines
  }
}

const resizeCanvas = () => {
  if (!canvasRef.value) return
  canvasRef.value.width = window.innerWidth
  canvasRef.value.height = window.innerHeight
  initParticles()
}

const initParticles = () => {
  if (!canvasRef.value) return
  particles = []
  // Slightly denser particles
  const numberOfParticles = Math.min(Math.floor((window.innerWidth * window.innerHeight) / 15000), 150)
  for (let i = 0; i < numberOfParticles; i++) {
    particles.push(new Particle(canvasRef.value.width, canvasRef.value.height))
  }
}

const drawLines = (isDarkMode: boolean) => {
  if (!ctx) return
  for (let i = 0; i < particles.length; i++) {
    for (let j = i + 1; j < particles.length; j++) {
      const dx = particles[i].x - particles[j].x
      const dy = particles[i].y - particles[j].y
      const distance = Math.sqrt(dx * dx + dy * dy)

      // Increased connection distance and opacity
      if (distance < 180) {
        ctx.beginPath()
        const alpha = 0.25 * (1 - distance / 180) // More opaque lines
        ctx.strokeStyle = isDarkMode ? `rgba(100, 180, 255, ${alpha})` : `rgba(59, 130, 246, ${alpha * 1.5})`
        ctx.lineWidth = 1.0 // thicker lines
        ctx.moveTo(particles[i].x, particles[i].y)
        ctx.lineTo(particles[j].x, particles[j].y)
        ctx.stroke()
      }
    }

    if (isHovering) {
      const dx = mouse.x - particles[i].x
      const dy = mouse.y - particles[i].y
      const distance = Math.sqrt(dx * dx + dy * dy)
      // Increased mouse connection distance and opacity
      if (distance < 260) {
        ctx.beginPath()
        const alpha = 0.3 * (1 - distance / 260)
        ctx.strokeStyle = isDarkMode ? `rgba(100, 220, 255, ${alpha})` : `rgba(6, 182, 212, ${alpha * 1.5})`
        ctx.lineWidth = 1.5
        ctx.moveTo(particles[i].x, particles[i].y)
        ctx.lineTo(mouse.x, mouse.y)
        ctx.stroke()
      }
    }
  }
}

const animate = () => {
  if (!ctx || !canvasRef.value) return
  ctx.clearRect(0, 0, canvasRef.value.width, canvasRef.value.height)

  const currentDark = isDark.value
  particles.forEach(p => {
    p.update(canvasRef.value!.width, canvasRef.value!.height)
    p.draw(ctx!, currentDark)
  })

  drawLines(currentDark)
  animationFrameId = requestAnimationFrame(animate)
}

const onMouseMove = (e: MouseEvent) => {
  mouse.x = e.clientX
  mouse.y = e.clientY
  isHovering = true

  if (glowRef.value) {
    gsap.to(glowRef.value, {
      x: mouse.x - window.innerWidth / 2,
      y: mouse.y - window.innerHeight / 2,
      duration: 0.8,
      ease: 'power2.out'
    })
  }
}

const onMouseLeave = () => {
  isHovering = false
  if (glowRef.value) {
    gsap.to(glowRef.value, {
      x: 0,
      y: 0,
      duration: 1.5,
      ease: 'power3.inOut'
    })
  }
}

let resizeTimer: number
const debouncedResize = () => {
  clearTimeout(resizeTimer)
  resizeTimer = window.setTimeout(resizeCanvas, 150)
}

onMounted(async () => {
  await nextTick()
  if (canvasRef.value) {
    ctx = canvasRef.value.getContext('2d')
    window.addEventListener('resize', debouncedResize)
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseleave', onMouseLeave)

    resizeCanvas()
    animate()

    if (gridRef.value) {
      gsap.fromTo(gridRef.value,
        { opacity: 0, scale: 1.05 },
        { opacity: 0.2, scale: 1, duration: 2.5, ease: 'power3.out' }
      )
    }
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', debouncedResize)
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('mouseleave', onMouseLeave)
  cancelAnimationFrame(animationFrameId)
})
</script>

<style scoped>
.bg-grid-dark {
  background-image:
    linear-gradient(to right, rgba(234, 242, 246, 0.05) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(234, 242, 246, 0.05) 1px, transparent 1px);
  background-size: 52px 52px;
}
.bg-grid-light {
  background-image:
    linear-gradient(to right, rgba(0, 0, 0, 0.04) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(0, 0, 0, 0.04) 1px, transparent 1px);
  background-size: 52px 52px;
}
</style>
