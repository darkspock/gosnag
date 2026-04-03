import { useEffect } from 'react'
import { X, CheckCircle2, AlertCircle, Info } from 'lucide-react'
import { useToastStore, type ToastVariant } from '@/lib/use-toast'
import { cn } from '@/lib/utils'

const icons: Record<ToastVariant, typeof CheckCircle2> = {
  success: CheckCircle2,
  error: AlertCircle,
  info: Info,
}

const styles: Record<ToastVariant, string> = {
  success: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300',
  error: 'border-red-500/30 bg-red-500/10 text-red-300',
  info: 'border-blue-500/30 bg-blue-500/10 text-blue-300',
}

export function Toaster() {
  const { toasts, dismiss, subscribe } = useToastStore()

  useEffect(() => subscribe(), [subscribe])

  if (toasts.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 max-w-sm">
      {toasts.map(t => {
        const Icon = icons[t.variant]
        return (
          <div
            key={t.id}
            className={cn(
              'flex items-center gap-2.5 rounded-lg border px-4 py-3 text-sm shadow-lg backdrop-blur-sm animate-slide-up',
              styles[t.variant]
            )}
          >
            <Icon className="h-4 w-4 shrink-0" />
            <span className="flex-1">{t.message}</span>
            <button
              onClick={() => dismiss(t.id)}
              className="shrink-0 rounded p-0.5 hover:bg-white/10 transition-colors"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
