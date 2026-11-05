import { useEffect, useRef, useState } from 'react'

const STAGE_W = 880
const STAGE_H = 540
const PHONE_W = 640
const PHONE_SCALE = 0.72

// Plays a looping story on an 880 x 540 stage scaled to fit its parent. It rests
// while off screen, and people who ask for less motion see only the last frame.
// Shrunk to a phone the stage would be unreadable, so there it stays at a readable
// size and a camera pans to `focus(frame)`, the stage x that matters at that moment.
export function useFlow(frames: { d: number }[], rest: number, focus: (k: number) => number) {
  const ref = useRef<HTMLDivElement>(null)
  const [k, setK] = useState(0)
  const [width, setWidth] = useState(STAGE_W)
  const [onScreen, setOnScreen] = useState(false)
  const [still] = useState(() => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const fit = () => { if (el.clientWidth) setWidth(el.clientWidth) }
    fit()
    const size = typeof ResizeObserver === 'function' ? new ResizeObserver(fit) : null
    size?.observe(el)
    const seen = typeof IntersectionObserver === 'function' ? new IntersectionObserver(([e]) => setOnScreen(e.isIntersecting)) : null
    if (seen) seen.observe(el)
    else setOnScreen(true)
    return () => { size?.disconnect(); seen?.disconnect() }
  }, [])

  useEffect(() => {
    if (still || !onScreen) return
    const id = setTimeout(() => setK(i => (i + 1) % frames.length), frames[k].d)
    return () => clearTimeout(id)
  }, [k, onScreen, still, frames])

  const shown = still ? rest : k
  const phone = width < PHONE_W
  const scale = phone ? Math.max(PHONE_SCALE, width / STAGE_W) : width / STAGE_W
  const seen = width / scale
  const panX = phone ? Math.max(0, Math.min(STAGE_W - seen, focus(shown) - seen / 2)) * scale : 0
  return {
    ref, k: shown, jump: setK,
    boxH: Math.round(STAGE_H * scale) + 'px',
    scaleT: `translateX(${-Math.round(panX)}px) scale(${scale})`,
    clip: phone ? 'hidden' as const : 'visible' as const,
    pan: phone && !still ? 'transform .7s cubic-bezier(.45,0,.2,1)' : 'none',
  }
}
