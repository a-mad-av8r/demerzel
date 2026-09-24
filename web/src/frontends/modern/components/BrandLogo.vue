<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref, useId, watch } from 'vue'

const props = defineProps<{
  compact?: boolean
  resolvedTheme: 'light' | 'dark'
}>()
type Reaction = 'idle' | 'sniff' | 'nudge' | 'fold' | 'unfold' | 'theme'

const root = ref<HTMLElement>()
const body = ref<SVGGElement>()
const reaction = ref<Reaction>('idle')
const charging = ref(false)
const sequence = ref(0)
const look = ref({ x: 0, y: 0 })
const identity = useId()
const silhouette = 'mascot-shape-' + identity
const bodyClip = 'mascot-body-' + identity
const tailClip = 'mascot-tail-' + identity
const pose = computed(() => ({
  '--modern-mascot-look-x': look.value.x + 'px',
  '--modern-mascot-look-y': look.value.y + 'px',
}))
let reducedMotion: MediaQueryList | undefined
let hoverTimer: ReturnType<typeof setTimeout> | undefined
let holdTimer: ReturnType<typeof setTimeout> | undefined
let pointer: { id: number; x: number; y: number; consumed: boolean } | undefined
let lastSniff = -Infinity
let suppressClickUntil = 0
let disposed = false

function canAnimate(): boolean {
  return Boolean(
    !disposed &&
    reducedMotion &&
    !reducedMotion.matches &&
    !document.hidden &&
    root.value?.getClientRects().length,
  )
}
function timing(name: 'hover-delay' | 'charge-duration' | 'cooldown'): number {
  const value = getComputedStyle(root.value!)
    .getPropertyValue('--modern-mascot-' + name)
    .trim()
  return Number.parseFloat(value) * (value.endsWith('ms') ? 1 : 1000)
}
function clearHover(): void {
  clearTimeout(hoverTimer)
  hoverTimer = undefined
}
function cancelPress(): void {
  clearTimeout(holdTimer)
  holdTimer = undefined
  charging.value = false
  if (pointer?.consumed) suppressClickUntil = performance.now() + 800
  const id = pointer?.id
  pointer = undefined
  if (id !== undefined && root.value?.hasPointerCapture(id)) root.value.releasePointerCapture(id)
}
function reset(): void {
  clearHover()
  cancelPress()
  reaction.value = 'idle'
}
function play(next: Reaction): void {
  if (!canAnimate()) return
  clearHover()
  sequence.value++
  reaction.value = next
}
function nudge(): void {
  if (reaction.value === 'nudge' || !canAnimate()) return
  reset()
  play('nudge')
}
function nearMascot(event: PointerEvent): boolean {
  const rect = root.value?.getBoundingClientRect()
  if (!rect || !rect.width) return false
  const width = props.compact ? rect.width : rect.width * 0.32
  const x = event.clientX - rect.left
  const y = event.clientY - rect.top
  if (x < 0 || x > width || y < 0 || y > rect.height) return false
  look.value = {
    x: Math.round((x / width - 0.35) * 28),
    y: Math.round((y / rect.height - 0.5) * 20),
  }
  return true
}
function hover(event: PointerEvent): void {
  if (pointer) {
    if (Math.hypot(event.clientX - pointer.x, event.clientY - pointer.y) > 10) {
      cancelPress()
    }
    return
  }
  if (event.pointerType !== 'mouse' || !canAnimate()) return
  if (!nearMascot(event)) {
    clearHover()
    return
  }
  if (
    reaction.value !== 'idle' ||
    hoverTimer !== undefined ||
    performance.now() - lastSniff < timing('cooldown')
  )
    return
  hoverTimer = setTimeout(() => {
    hoverTimer = undefined
    if (reaction.value !== 'idle' || !canAnimate()) return
    lastSniff = performance.now()
    play('sniff')
  }, timing('hover-delay'))
}
function leave(): void {
  clearHover()
  if (pointer && !pointer.consumed) cancelPress()
}
function press(event: PointerEvent): void {
  if (event.isPrimary) suppressClickUntil = 0
  if (
    event.button !== 0 ||
    !event.isPrimary ||
    event.ctrlKey ||
    event.metaKey ||
    event.altKey ||
    event.shiftKey ||
    !canAnimate() ||
    !nearMascot(event)
  )
    return
  reset()
  suppressClickUntil = 0
  pointer = { id: event.pointerId, x: event.clientX, y: event.clientY, consumed: false }
  root.value?.setPointerCapture(event.pointerId)
  charging.value = true
  holdTimer = setTimeout(() => {
    holdTimer = undefined
    if (!pointer || !canAnimate()) return
    pointer.consumed = true
    charging.value = false
    play('nudge')
  }, timing('charge-duration'))
}
function release(): void {
  cancelPress()
}
function click(event: MouseEvent): void {
  // 仅吞掉长按后的那次指针点击；普通点击与 Enter 仍交给首页链接。
  const keyboardClick = event.detail === 0 && !('pointerType' in event && event.pointerType)
  if (keyboardClick || performance.now() >= suppressClickUntil) return
  suppressClickUntil = 0
  event.preventDefault()
  event.stopPropagation()
}
function contextMenu(event: MouseEvent): void {
  if (pointer) event.preventDefault()
}
function finish(event: AnimationEvent): void {
  if (event.target !== body.value || event.animationName.startsWith('modern-mascot-charge')) return
  reaction.value = 'idle'
}
function cancelWhenHidden(): void {
  if (document.hidden) reset()
}
function motionPreferenceChanged(): void {
  if (reducedMotion?.matches) reset()
}
watch(
  () => props.compact,
  (compact) => {
    reset()
    play(compact ? 'fold' : 'unfold')
  },
)
watch(
  () => props.resolvedTheme,
  () => {
    reset()
    // 仅响应最终生效模式的变化；首次渲染及同色的偏好切换不播放。
    play('theme')
  },
)
onMounted(() => {
  reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
  reducedMotion.addEventListener('change', motionPreferenceChanged)
  document.addEventListener('visibilitychange', cancelWhenHidden)
})
onScopeDispose(() => {
  disposed = true
  reset()
  reducedMotion?.removeEventListener('change', motionPreferenceChanged)
  document.removeEventListener('visibilitychange', cancelWhenHidden)
})
defineExpose({ nudge })
</script>

<template>
  <span
    ref="root"
    class="modern-brand-mascot"
    :class="[
      'is-' + reaction,
      { 'is-compact': compact, 'is-charging': charging, 'is-dark': resolvedTheme === 'dark' },
    ]"
    :style="pose"
    @pointerenter="hover"
    @pointermove="hover"
    @pointerleave="leave"
    @pointerdown="press"
    @pointerup="release"
    @pointercancel="release"
    @lostpointercapture="release"
    @click.capture="click"
    @contextmenu="contextMenu"
    @animationend="finish"
  >
    <svg
      :key="sequence"
      class="modern-brand-mascot-scene"
      viewBox="0 0 512 512"
      :width="compact ? 32 : 40"
      :height="compact ? 32 : 40"
      role="img"
      aria-label="Demerzel"
      focusable="false"
    >
      <defs>
        <!-- Reuse the original mascot silhouette; clipping only separates its tail. -->
        <path
          :id="silhouette"
          fill-rule="evenodd"
          d="M 257.47 0.47 C 280.84 -0.99 304.38 0.1 327.65 2.35 C 346.64 4.19 366.32 6.86 382.88 17.12 C 407.97 32.65 425.34 58.28 451.47 72.53 C 469.41 82.31 489.13 86.89 508.24 93.76 C 517.44 97.08 527.63 100.44 535.41 106.59 C 545.92 114.88 548 128.45 548 141 C 548 155.44 546.7 171.18 535.94 181.94 C 526.87 191.01 510.4 193.38 498.47 196.47 C 485.06 199.95 471.76 203.88 458.35 207.35 C 450.09 209.5 439.94 211.04 432.59 215.59 C 423.59 221.16 420.23 234.99 425.88 244.12 C 429.61 250.13 436.95 250.74 443.06 252.94 C 455.36 257.37 465.69 270.34 462.12 284.12 C 458.79 296.97 443.26 295 433 295 C 409.67 295 386.33 295 363 295 C 353.26 295 340.17 297.21 331.12 292.88 C 324.26 289.6 321.96 282.55 322.47 275.35 C 323.23 264.7 329.4 254.24 326.59 243.41 C 317.96 210.15 289.31 193.37 256.94 189.06 C 220.4 184.19 177.47 189.7 156.53 223.53 C 152.9 229.4 148.6 237.81 148.29 244.82 C 148.03 250.84 154.62 253.15 158.24 256.76 C 165.74 264.26 171.43 278.72 164.06 288.06 C 157.78 296.01 141.76 293 133 293 C 107 293 81 293 55 293 C 45.74 293 32.14 295.38 24.88 288.12 C 16.35 279.59 21.19 267.1 22.94 256.94 C 25.76 240.6 30.44 224.6 34.59 208.59 C 36.14 202.62 41.47 193.01 39.24 186.94 C 37.61 182.53 30.3 180.81 26.71 178.29 C 9.06 165.94 0 146.4 0 125 C 0 117.31 0.74 107.31 6.53 101.53 C 11.47 96.59 18.08 99.67 21.24 104.76 C 28.41 116.36 33.31 126.71 46.71 132.29 C 50.98 134.08 56.91 136.36 61.29 133.82 C 66.56 130.78 69.15 120.37 72.29 115.29 C 81.45 100.5 90.79 86.26 101.59 72.59 C 125.83 41.89 160.6 17.44 198.59 7.59 C 217.75 2.62 237.8 1.7 257.47 0.47 Z M 458.88 136.12 C 461.21 145.98 471.28 152.37 481.24 150.24 C 491.44 148.05 497.45 138.08 495.06 127.94 C 492.68 117.9 483.1 111.68 472.94 113.94 C 462.9 116.17 456.5 126.06 458.88 136.12 Z"
        />
        <clipPath :id="bodyClip">
          <path d="M72 0V115L38 187L0 211V310H560V0Z" />
        </clipPath>
        <clipPath :id="tailClip">
          <path d="M0 0H74V116L40 188L0 212Z" />
        </clipPath>
      </defs>
      <rect v-if="compact" class="modern-brand-mascot-tile" width="512" height="512" rx="128" />
      <g
        class="modern-brand-mascot-figure"
        :transform="'translate(51.2 145.7518) scale(0.7474)'"
      >
        <g ref="body" class="modern-brand-mascot-body">
          <g class="modern-brand-mascot-tail">
            <use :href="'#' + silhouette" :clip-path="'url(#' + tailClip + ')'" />
          </g>
          <use :href="'#' + silhouette" :clip-path="'url(#' + bodyClip + ')'" />
          <ellipse class="modern-brand-mascot-lid" cx="477" cy="132" rx="23" ry="23" />
          <path class="modern-brand-mascot-closed-eye" d="M462 135Q477 146 492 133" />
        </g>
        <g class="modern-brand-mascot-scent" aria-hidden="true">
          <path d="M569 116Q581 128 569 140M590 107Q607 128 590 149" />
        </g>
        <g class="modern-brand-mascot-impact" aria-hidden="true">
          <path d="M635 89L654 68M646 116L675 112M641 143L660 158" />
        </g>
        <g class="modern-brand-mascot-day" aria-hidden="true">
          <circle cx="389" cy="-25" r="13" />
          <path
            d="M389 -56V-49M389 -1V6M358 -25H365M413 -25H420M367 -47L372 -42M406 -8L411 -3M367 -3L372 -8M406 -42L411 -47"
          />
        </g>
        <g class="modern-brand-mascot-night" aria-hidden="true">
          <path d="M412 -51A30 30 0 1 0 437 -4A27 27 0 0 1 412 -51Z" />
          <path d="M460 -48V-32M452 -40H468" />
        </g>
      </g>
    </svg>
    <span v-if="!compact" class="modern-brand-mascot-name" aria-hidden="true">Demerzel</span>
  </span>
</template>

<style scoped>
.modern-brand-mascot {
  --modern-mascot-look-x: 0px;
  --modern-mascot-look-y: 0px;
  --modern-mascot-eye-color: var(--modern-mascot-backdrop);
  display: flex;
  min-width: 0;
  max-width: 100%;
  flex: none;
  align-items: center;
  gap: var(--modern-space-2);
  color: var(--modern-action);
  user-select: none;
  -webkit-touch-callout: none;
}
.modern-brand-mascot.is-compact {
  --modern-mascot-eye-color: var(--modern-action);
  display: block;
  width: var(--modern-control-sm);
  aspect-ratio: 1;
}
.modern-brand-mascot-scene {
  display: block;
  width: 40px;
  height: 40px;
  flex: none;
  overflow: visible;
}
.is-compact .modern-brand-mascot-scene {
  width: var(--modern-control-sm);
  height: var(--modern-control-sm);
}
.modern-brand-mascot-name {
  min-width: 0;
  color: var(--modern-text);
  font-size: var(--modern-font-size-section);
  font-weight: var(--modern-weight-semibold);
  line-height: var(--modern-leading-compact);
  white-space: nowrap;
}
.modern-brand-mascot-figure {
  fill: currentColor;
}
.is-compact .modern-brand-mascot-figure {
  color: var(--modern-mascot-inverse);
}
.modern-brand-mascot-tile {
  fill: var(--modern-action);
  transform-origin: 256px 256px;
}
.modern-brand-mascot-body {
  transform-origin: 270px 275px;
}
.modern-brand-mascot-tail {
  transform-origin: 57px 162px;
}
.modern-brand-mascot-lid {
  transform-origin: 477px 132px;
  transform: scaleY(0);
}
.modern-brand-mascot-closed-eye {
  fill: none;
  stroke: var(--modern-mascot-eye-color);
  stroke-width: var(--modern-mascot-stroke);
  opacity: 0;
}
.modern-brand-mascot-scent,
.modern-brand-mascot-impact,
.modern-brand-mascot-day,
.modern-brand-mascot-night {
  fill: none;
  stroke: currentColor;
  stroke-width: var(--modern-mascot-stroke);
  stroke-linecap: round;
  stroke-linejoin: round;
  opacity: 0;
  pointer-events: none;
}
.modern-brand-mascot-night {
  display: none;
}
.is-charging .modern-brand-mascot-body {
  animation: modern-mascot-charge var(--modern-mascot-charge-duration) var(--modern-mascot-ease)
    both;
}
.is-sniff .modern-brand-mascot-body {
  animation: modern-mascot-sniff var(--modern-mascot-sniff-duration) var(--modern-mascot-ease) both;
}
.is-sniff .modern-brand-mascot-tail,
.is-unfold .modern-brand-mascot-tail {
  animation: modern-mascot-wag var(--modern-mascot-sniff-duration) var(--modern-mascot-ease) both;
}
.is-sniff .modern-brand-mascot-lid,
.is-unfold .modern-brand-mascot-lid {
  animation: modern-mascot-blink var(--modern-mascot-sniff-duration) linear both;
}
.is-sniff .modern-brand-mascot-scent {
  animation: modern-mascot-scent var(--modern-mascot-sniff-duration) var(--modern-mascot-ease) both;
}
.is-nudge .modern-brand-mascot-body {
  animation: modern-mascot-nudge var(--modern-mascot-nudge-duration) var(--modern-mascot-ease) both;
}
.is-nudge .modern-brand-mascot-impact {
  animation: modern-mascot-impact var(--modern-mascot-nudge-duration) linear both;
}
.is-compact.is-nudge .modern-brand-mascot-impact {
  display: none;
}
.is-compact.is-nudge .modern-brand-mascot-tile {
  animation: modern-mascot-pocket var(--modern-mascot-nudge-duration) var(--modern-mascot-ease) both;
}
.is-fold .modern-brand-mascot-body {
  animation: modern-mascot-fold var(--modern-mascot-fold-duration) var(--modern-mascot-ease) both;
}
.is-fold .modern-brand-mascot-tile {
  animation: modern-mascot-pocket var(--modern-mascot-fold-duration) var(--modern-mascot-ease) both;
}
.is-unfold .modern-brand-mascot-body {
  animation: modern-mascot-unfold var(--modern-mascot-sniff-duration) var(--modern-mascot-ease) both;
}
.is-theme .modern-brand-mascot-body {
  animation: modern-mascot-wake var(--modern-mascot-theme-duration) var(--modern-mascot-ease) both;
}
.is-theme .modern-brand-mascot-lid {
  animation: modern-mascot-day-eyes var(--modern-mascot-theme-duration) linear both;
}
.is-theme .modern-brand-mascot-day,
.is-theme .modern-brand-mascot-night {
  animation: modern-mascot-mood var(--modern-mascot-theme-duration) var(--modern-mascot-ease) both;
}
.modern-brand-mascot.is-dark.is-theme .modern-brand-mascot-body {
  animation-name: modern-mascot-doze;
}
.modern-brand-mascot.is-dark.is-theme .modern-brand-mascot-lid {
  animation-name: modern-mascot-night-eyes;
}
.modern-brand-mascot.is-dark.is-theme .modern-brand-mascot-closed-eye {
  animation: modern-mascot-sleep-eye var(--modern-mascot-theme-duration) linear both;
}
.modern-brand-mascot.is-dark .modern-brand-mascot-day {
  display: none;
}
.modern-brand-mascot.is-dark .modern-brand-mascot-night {
  display: block;
}
@keyframes modern-mascot-charge {
  from {
    transform: none;
  }
  to {
    transform: translateX(-12px) scale(1.05, 0.84);
  }
}
@keyframes modern-mascot-sniff {
  0%,
  100% {
    transform: none;
  }
  25%,
  52% {
    transform: translate(var(--modern-mascot-look-x), var(--modern-mascot-look-y)) rotate(-2deg)
      scale(1.04, 0.98);
  }
  38%,
  64% {
    transform: translate(calc(var(--modern-mascot-look-x) + 12px), var(--modern-mascot-look-y))
      rotate(-3deg) scale(1.055, 0.97);
  }
  82% {
    transform: translateY(-5px);
  }
}
@keyframes modern-mascot-wag {
  0%,
  18%,
  100% {
    transform: none;
  }
  30%,
  52% {
    transform: rotate(-17deg);
  }
  41%,
  63% {
    transform: rotate(12deg);
  }
  78% {
    transform: rotate(-5deg);
  }
}
@keyframes modern-mascot-blink {
  0%,
  65%,
  76%,
  100% {
    transform: scaleY(0);
  }
  70% {
    transform: scaleY(1);
  }
}
@keyframes modern-mascot-scent {
  0%,
  24%,
  70%,
  100% {
    opacity: 0;
    transform: translateX(6px);
  }
  32%,
  52% {
    opacity: var(--modern-opacity-quiet);
    transform: translateX(0);
  }
}
@keyframes modern-mascot-nudge {
  0% {
    transform: translateX(-12px) scale(1.05, 0.84);
  }
  18% {
    transform: translateX(-24px) rotate(3deg) scale(1.07, 0.88);
  }
  36% {
    transform: translate(90px, -8px) rotate(-4deg) scale(1.05, 1.02);
  }
  45% {
    transform: translate(76px, -6px) scale(0.98, 1.05);
  }
  65% {
    transform: translate(-7px, -10px) rotate(2deg);
  }
  83% {
    transform: translateY(4px) scale(1.015, 0.98);
  }
  100% {
    transform: none;
  }
}
@keyframes modern-mascot-impact {
  0%,
  33%,
  52%,
  100% {
    opacity: 0;
  }
  37%,
  43% {
    opacity: var(--modern-opacity-quiet);
  }
}
@keyframes modern-mascot-fold {
  0% {
    transform: translate(16px, -25px) scale(1.32, 1.12);
  }
  30% {
    transform: translateY(25px) rotate(5deg) scale(0.75, 0.62);
  }
  60% {
    transform: translateY(-12px) rotate(-3deg) scale(1.07, 1.08);
  }
  80% {
    transform: translateY(4px) scale(1.015, 0.97);
  }
  100% {
    transform: none;
  }
}
@keyframes modern-mascot-pocket {
  0% {
    transform: scale(0.82);
  }
  40% {
    transform: scale(1.045);
  }
  70% {
    transform: scale(0.99);
  }
  100% {
    transform: none;
  }
}
@keyframes modern-mascot-unfold {
  0% {
    transform: translateY(15px) scale(0.65, 0.74);
  }
  28% {
    transform: translateY(-16px) scale(1.08, 1.14);
  }
  46% {
    transform: translateY(4px) scale(1.025, 0.96);
  }
  68% {
    transform: translateY(-4px) scale(0.99, 1.025);
  }
  100% {
    transform: none;
  }
}
@keyframes modern-mascot-wake {
  0%,
  100% {
    transform: none;
  }
  18% {
    transform: translateY(9px) scale(1.05, 0.85);
  }
  45% {
    transform: translateY(-19px) rotate(-3deg) scale(0.96, 1.13);
  }
  63% {
    transform: translateY(4px) rotate(1deg) scale(1.025, 0.97);
  }
  80% {
    transform: translateY(-3px);
  }
}
@keyframes modern-mascot-doze {
  0%,
  100% {
    transform: none;
  }
  27%,
  66% {
    transform: translateY(12px) rotate(2deg) scale(1.04, 0.87);
  }
  78% {
    transform: translateY(-4px) scale(0.99, 1.035);
  }
}
@keyframes modern-mascot-day-eyes {
  0%,
  29% {
    transform: scaleY(1);
  }
  42%,
  58%,
  74%,
  100% {
    transform: scaleY(0);
  }
  66% {
    transform: scaleY(1);
  }
}
@keyframes modern-mascot-night-eyes {
  0%,
  12%,
  84%,
  100% {
    transform: scaleY(0);
  }
  32%,
  67% {
    transform: scaleY(1);
  }
}
@keyframes modern-mascot-sleep-eye {
  0%,
  24%,
  78%,
  100% {
    opacity: 0;
  }
  34%,
  64% {
    opacity: 1;
  }
}
@keyframes modern-mascot-mood {
  0%,
  14%,
  92%,
  100% {
    opacity: 0;
    transform: translateY(12px);
  }
  32%,
  67% {
    opacity: var(--modern-opacity-quiet);
    transform: translateY(0);
  }
}
</style>
