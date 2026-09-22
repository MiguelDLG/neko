import Vue from 'vue'
import { getterTree, mutationTree, actionTree } from 'typed-vuex'
import { get, set } from '~/utils/localstorage'
import { EVENT } from '~/neko/events'
import { DrawStroke, DrawStrokeSendPayload, DrawPoint } from '~/neko/messages'
import { accessor } from '~/store'

export const namespaced = true

export const COLORS = [
  '#ff3b30',
  '#ff9500',
  '#ffcc00',
  '#34c759',
  '#00c7be',
  '#007aff',
  '#af52de',
  '#ffffff',
  '#000000',
]

// how long a fading (laser) stroke stays on screen, in ms
export const FADE_MS = 3000

export const state = () => ({
  // server says the plugin exists
  enabled: false,
  // server says this session may draw
  canDraw: false,
  // local draw mode toggle (canvas captures the pointer)
  active: false,
  color: get<string>('draw_color', COLORS[0]),
  width: get<number>('draw_width', 0.004),
  fade: get<boolean>('draw_fade', false),
  strokes: [] as DrawStroke[],
  // ids of fading strokes with their expiry, so the canvas can drop them
  fading: {} as Record<string, number>,
  // bumped on every board change so the canvas knows to repaint
  revision: 0,
})

export const getters = getterTree(state, {
  available: (state) => state.enabled && state.canDraw,
})

export const mutations = mutationTree(state, {
  setInit(state, { enabled, canDraw, strokes }: { enabled: boolean; canDraw: boolean; strokes: DrawStroke[] }) {
    state.enabled = enabled
    state.canDraw = canDraw
    state.strokes = strokes.map((s) => ({ ...s, points: s.points.slice() }))
    state.fading = {}
    state.revision++
    if (!state.canDraw) {
      state.active = false
    }
  },

  setActive(state, active: boolean) {
    state.active = active
  },

  setColor(state, color: string) {
    state.color = color
    set('draw_color', color)
  },

  setWidth(state, width: number) {
    state.width = width
    set('draw_width', width)
  },

  setFade(state, fade: boolean) {
    state.fade = fade
    set('draw_fade', fade)
  },

  // append points to an existing stroke or start a new one
  upsertStroke(state, stroke: DrawStroke) {
    const existing = state.strokes.find((s) => s.id === stroke.id)
    if (existing) {
      existing.points.push(...stroke.points)
    } else {
      state.strokes.push({ ...stroke, points: stroke.points.slice() })
    }
    if (stroke.fade) {
      Vue.set(state.fading, stroke.id, Date.now() + FADE_MS)
    }
    state.revision++
  },

  removeStroke(state, id: string) {
    state.strokes = state.strokes.filter((s) => s.id !== id)
    if (id in state.fading) {
      Vue.delete(state.fading, id)
    }
    state.revision++
  },

  // drop every fading stroke whose time is up
  expireFading(state) {
    const now = Date.now()
    const expired = Object.keys(state.fading).filter((id) => state.fading[id] <= now)
    if (expired.length === 0) return
    state.strokes = state.strokes.filter((s) => !expired.includes(s.id))
    for (const id of expired) {
      Vue.delete(state.fading, id)
    }
    state.revision++
  },

  clear(state) {
    state.strokes = []
    state.fading = {}
    state.revision++
  },

  reset(state) {
    state.enabled = false
    state.canDraw = false
    state.active = false
    state.strokes = []
    state.fading = {}
    state.revision++
  },
})

export const actions = actionTree(
  { state, getters, mutations },
  {
    toggle({ state, getters }) {
      if (!getters.available) {
        accessor.draw.setActive(false)
        return
      }
      accessor.draw.setActive(!state.active)
    },

    // called by the canvas for our own strokes: render locally + send
    sendStroke({ getters }, payload: DrawStrokeSendPayload) {
      if (!accessor.connected || !getters.available || payload.points.length === 0) {
        return
      }

      accessor.draw.upsertStroke({ ...payload, user_id: accessor.user.id })
      $client.sendMessage(EVENT.DRAW.STROKE, payload)
    },

    // undo our last stroke (server resolves which one) or a specific id
    sendUndo({ getters }, id?: string) {
      if (!accessor.connected || !getters.available) {
        return
      }
      $client.sendMessage(EVENT.DRAW.UNDO, id ? { id } : {})
    },

    sendClear({ getters }) {
      if (!accessor.connected || !getters.available) {
        return
      }
      $client.sendMessage(EVENT.DRAW.CLEAR, {})
    },
  },
)

export type { DrawStroke, DrawPoint }
