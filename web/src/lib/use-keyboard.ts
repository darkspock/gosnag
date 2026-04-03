import { useEffect } from 'react'

type KeyHandler = (e: KeyboardEvent) => void

export function useKeyboardShortcut(shortcuts: Record<string, KeyHandler>) {
  useEffect(() => {
    function handler(e: KeyboardEvent) {
      // Skip if user is typing in an input/textarea/select
      const tag = (e.target as HTMLElement)?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return

      const key = e.key.toLowerCase()
      const fn = shortcuts[key]
      if (fn) fn(e)
    }

    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [shortcuts])
}
