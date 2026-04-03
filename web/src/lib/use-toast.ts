import { useState, useCallback } from 'react'

export type ToastVariant = 'success' | 'error' | 'info'

export interface Toast {
  id: string
  message: string
  variant: ToastVariant
}

let listeners: Array<(toast: Toast) => void> = []
let counter = 0

export function toast(message: string, variant: ToastVariant = 'info') {
  const t: Toast = { id: String(++counter), message, variant }
  listeners.forEach(fn => fn(t))
}

toast.success = (message: string) => toast(message, 'success')
toast.error = (message: string) => toast(message, 'error')
toast.info = (message: string) => toast(message, 'info')

export function useToastStore() {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((t: Toast) => {
    setToasts(prev => [...prev, t])
    setTimeout(() => {
      setToasts(prev => prev.filter(x => x.id !== t.id))
    }, 3500)
  }, [])

  const dismiss = useCallback((id: string) => {
    setToasts(prev => prev.filter(x => x.id !== id))
  }, [])

  const subscribe = useCallback(() => {
    listeners.push(addToast)
    return () => {
      listeners = listeners.filter(fn => fn !== addToast)
    }
  }, [addToast])

  return { toasts, dismiss, subscribe }
}
