<template>
  <div class="draw" :class="{ active }">
    <canvas
      ref="canvas"
      class="draw-canvas"
      :style="{ pointerEvents: active ? 'auto' : 'none' }"
      @pointerdown.stop.prevent="onPointerDown"
      @pointermove.stop.prevent="onPointerMove"
      @pointerup.stop.prevent="onPointerUp"
      @pointercancel.stop.prevent="onPointerUp"
      @contextmenu.stop.prevent
    />
    <div v-if="active && !hideControls" class="draw-toolbar" @pointerdown.stop @click.stop.prevent>
      <ul class="colors">
        <li v-for="c in colors" :key="c">
          <span
            class="swatch"
            :class="{ selected: c === color }"
            :style="{ background: c }"
            @click.stop.prevent="setColor(c)"
          />
        </li>
      </ul>
      <ul class="tools">
        <li v-for="s in sizes" :key="s.value">
          <span
            class="size"
            :class="{ selected: Math.abs(s.value - width) < 0.0001 }"
            v-tooltip="tooltip(s.label)"
            @click.stop.prevent="setWidth(s.value)"
          >
            <span class="dot" :style="{ width: s.px + 'px', height: s.px + 'px' }" />
          </span>
        </li>
        <li>
          <i
            class="fas fa-ghost"
            :class="{ selected: fade }"
            v-tooltip="tooltip(fade ? $t('draw.fade_off') : $t('draw.fade_on'))"
            @click.stop.prevent="toggleFade"
          />
        </li>
        <li>
          <i class="fas fa-rotate-left" v-tooltip="tooltip($t('draw.undo'))" @click.stop.prevent="undo" />
        </li>
        <li>
          <i class="fas fa-trash" v-tooltip="tooltip($t('draw.clear'))" @click.stop.prevent="clear" />
        </li>
        <li>
          <i class="fas fa-xmark" v-tooltip="tooltip($t('draw.close'))" @click.stop.prevent="close" />
        </li>
      </ul>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .draw {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    // above the input overlay (textarea) and emotes
    z-index: 5;
    pointer-events: none;

    .draw-canvas {
      position: absolute;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      touch-action: none;
      cursor: crosshair;
    }

    .draw-toolbar {
      position: absolute;
      top: 15px;
      left: 20px;
      display: flex;
      flex-direction: column;
      gap: 6px;
      padding: 8px;
      border-radius: 5px;
      background: rgba($color: #000, $alpha: 0.6);
      pointer-events: auto;

      ul {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 6px;
        list-style: none;
        margin: 0;
        padding: 0;

        li {
          display: flex;
          align-items: center;
          justify-content: center;
        }
      }

      .swatch {
        display: block;
        width: 20px;
        height: 20px;
        border-radius: 50%;
        border: 2px solid transparent;
        box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.35);
        cursor: pointer;

        &.selected {
          border-color: #fff;
          box-shadow: 0 0 0 2px $style-primary;
        }
      }

      .size {
        display: flex;
        width: 24px;
        height: 24px;
        align-items: center;
        justify-content: center;
        border-radius: 4px;
        cursor: pointer;

        .dot {
          display: block;
          border-radius: 50%;
          background: #fff;
        }

        &.selected {
          background: rgba($style-primary, 0.5);
        }
      }

      i {
        width: 24px;
        height: 24px;
        line-height: 24px;
        text-align: center;
        border-radius: 4px;
        color: #fff;
        cursor: pointer;

        &:hover {
          background: rgba(255, 255, 255, 0.15);
        }

        &.selected {
          background: rgba($style-primary, 0.5);
        }
      }
    }
  }
</style>

<script lang="ts">
  import { Component, Ref, Vue, Watch, Prop } from 'vue-property-decorator'
  import { makeid } from '~/utils'
  import { COLORS, FADE_MS } from '~/store/draw'
  import { DrawPoint, DrawStroke } from '~/neko/messages'

  // minimum normalized distance between two recorded points
  const MIN_DIST = 0.0015
  // how often buffered points are flushed to the store + server
  const FLUSH_MS = 40

  @Component({ name: 'neko-draw' })
  export default class extends Vue {
    @Ref('canvas') readonly _canvas!: HTMLCanvasElement

    @Prop(Boolean) readonly hideControls!: boolean

    private observer = new ResizeObserver(this.onResize.bind(this))
    private ctx: CanvasRenderingContext2D | null = null
    private fadeTimer = 0

    // in-progress local stroke
    private drawing = false
    private pointerId = -1
    private strokeId = ''
    private lastPoint: DrawPoint | null = null
    private pending: DrawPoint[] = []
    private flushTimer = 0

    readonly colors = COLORS
    readonly sizes = [
      { label: 'S', value: 0.002, px: 4 },
      { label: 'M', value: 0.004, px: 7 },
      { label: 'L', value: 0.008, px: 11 },
    ]

    get active() {
      return this.$accessor.draw.active
    }

    get color() {
      return this.$accessor.draw.color
    }

    get width() {
      return this.$accessor.draw.width
    }

    get fade() {
      return this.$accessor.draw.fade
    }

    get strokes(): DrawStroke[] {
      return this.$accessor.draw.strokes
    }

    get fading() {
      return this.$accessor.draw.fading
    }

    get revision() {
      return this.$accessor.draw.revision
    }

    tooltip(content: string) {
      return { content, placement: 'right', offset: 5, boundariesElement: 'body', delay: { show: 300, hide: 100 } }
    }

    setColor(c: string) {
      this.$accessor.draw.setColor(c)
    }

    setWidth(w: number) {
      this.$accessor.draw.setWidth(w)
    }

    toggleFade() {
      this.$accessor.draw.setFade(!this.fade)
    }

    undo() {
      this.$accessor.draw.sendUndo()
    }

    clear() {
      this.$accessor.draw.sendClear()
    }

    close() {
      this.$accessor.draw.setActive(false)
    }

    mounted() {
      this.ctx = this._canvas.getContext('2d')
      this.observer.observe(this._canvas)
      this.onResize()
      window.addEventListener('keydown', this.onKeyDown)
    }

    beforeDestroy() {
      this.observer.disconnect()
      window.removeEventListener('keydown', this.onKeyDown)
      this.stopFadeTimer()
      if (this.flushTimer) window.clearTimeout(this.flushTimer)
    }

    @Watch('revision')
    onRevision() {
      this.repaint()
      this.syncFadeTimer()
    }

    @Watch('active')
    onActiveChanged(active: boolean) {
      if (!active) {
        this.endStroke()
      }
    }

    // ---- sizing / painting ------------------------------------------------

    onResize() {
      const rect = this._canvas.getBoundingClientRect()
      const dpr = window.devicePixelRatio || 1
      const w = Math.max(1, Math.round(rect.width * dpr))
      const h = Math.max(1, Math.round(rect.height * dpr))
      if (this._canvas.width !== w || this._canvas.height !== h) {
        this._canvas.width = w
        this._canvas.height = h
      }
      this.repaint()
    }

    private strokeAlpha(s: DrawStroke) {
      if (!s.fade) return 1
      const expires = this.fading[s.id]
      if (!expires) return 1
      const left = expires - Date.now()
      return Math.max(0, Math.min(1, left / FADE_MS))
    }

    private paintStroke(ctx: CanvasRenderingContext2D, s: DrawStroke, from = 0) {
      const W = this._canvas.width
      const H = this._canvas.height
      const pts = s.points
      if (pts.length === 0) return

      ctx.save()
      ctx.globalAlpha = this.strokeAlpha(s)
      ctx.strokeStyle = s.color
      ctx.fillStyle = s.color
      ctx.lineCap = 'round'
      ctx.lineJoin = 'round'
      ctx.lineWidth = Math.max(1, s.width * W)

      if (pts.length === 1) {
        // a tap: draw a dot
        ctx.beginPath()
        ctx.arc(pts[0][0] * W, pts[0][1] * H, ctx.lineWidth / 2, 0, Math.PI * 2)
        ctx.fill()
      } else {
        const start = Math.max(0, from - 1)
        ctx.beginPath()
        ctx.moveTo(pts[start][0] * W, pts[start][1] * H)
        for (let i = start + 1; i < pts.length; i++) {
          ctx.lineTo(pts[i][0] * W, pts[i][1] * H)
        }
        ctx.stroke()
      }
      ctx.restore()
    }

    repaint() {
      const ctx = this.ctx
      if (!ctx) return
      ctx.clearRect(0, 0, this._canvas.width, this._canvas.height)
      for (const s of this.strokes) {
        this.paintStroke(ctx, s)
      }
    }

    // while fading strokes exist, tick to animate + expire them
    private syncFadeTimer() {
      const hasFading = Object.keys(this.fading).length > 0
      if (hasFading && !this.fadeTimer) {
        this.fadeTimer = window.setInterval(() => {
          this.$accessor.draw.expireFading()
          this.repaint()
          if (Object.keys(this.fading).length === 0) {
            this.stopFadeTimer()
          }
        }, 50)
      } else if (!hasFading) {
        this.stopFadeTimer()
      }
    }

    private stopFadeTimer() {
      if (this.fadeTimer) {
        window.clearInterval(this.fadeTimer)
        this.fadeTimer = 0
      }
    }

    // ---- input ------------------------------------------------------------

    private toPoint(e: PointerEvent): DrawPoint {
      const rect = this._canvas.getBoundingClientRect()
      const x = (e.clientX - rect.left) / rect.width
      const y = (e.clientY - rect.top) / rect.height
      return [Math.min(1, Math.max(0, +x.toFixed(4))), Math.min(1, Math.max(0, +y.toFixed(4)))]
    }

    onPointerDown(e: PointerEvent) {
      if (!this.active || this.drawing) return
      if (e.pointerType === 'mouse' && e.button !== 0) return

      this.drawing = true
      this.pointerId = e.pointerId
      this.strokeId = makeid(16)
      this.lastPoint = this.toPoint(e)
      this.pending = [this.lastPoint]

      try {
        this._canvas.setPointerCapture(e.pointerId)
      } catch (err) {}

      this.scheduleFlush()
    }

    onPointerMove(e: PointerEvent) {
      if (!this.drawing || e.pointerId !== this.pointerId || !this.lastPoint) return

      const pt = this.toPoint(e)
      const dx = pt[0] - this.lastPoint[0]
      const dy = pt[1] - this.lastPoint[1]
      if (dx * dx + dy * dy < MIN_DIST * MIN_DIST) return

      // draw the segment immediately for a smooth local feel; the store
      // repaint on flush will redraw the same pixels
      if (this.ctx) {
        this.paintStroke(
          this.ctx,
          { id: this.strokeId, user_id: '', color: this.color, width: this.width, points: [this.lastPoint, pt] },
          1,
        )
      }

      this.lastPoint = pt
      this.pending.push(pt)
      this.scheduleFlush()
    }

    onPointerUp(e: PointerEvent) {
      if (!this.drawing || e.pointerId !== this.pointerId) return
      this.endStroke()
    }

    private endStroke() {
      if (!this.drawing) return
      this.flush()
      this.drawing = false
      this.pointerId = -1
      this.strokeId = ''
      this.lastPoint = null
      this.pending = []
      if (this.flushTimer) {
        window.clearTimeout(this.flushTimer)
        this.flushTimer = 0
      }
    }

    private scheduleFlush() {
      if (this.flushTimer) return
      this.flushTimer = window.setTimeout(() => {
        this.flushTimer = 0
        this.flush()
      }, FLUSH_MS)
    }

    private flush() {
      if (this.pending.length === 0 || !this.strokeId) return
      const points = this.pending
      this.pending = []
      this.$accessor.draw.sendStroke({
        id: this.strokeId,
        color: this.color,
        width: this.width,
        fade: this.fade || undefined,
        points,
      })
    }

    private onKeyDown = (e: KeyboardEvent) => {
      if (!this.active) return
      if (e.key === 'Escape') {
        e.preventDefault()
        this.close()
      } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z') {
        e.preventDefault()
        this.undo()
      }
    }
  }
</script>
