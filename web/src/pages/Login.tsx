import { useEffect, useRef, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Bug } from 'lucide-react'
import { api } from '@/lib/api'

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: {
            client_id: string
            callback: (response: { credential: string }) => void
            auto_select?: boolean
          }) => void
          renderButton: (
            element: HTMLElement,
            config: {
              theme?: string
              size?: string
              width?: number
              text?: string
              shape?: string
            }
          ) => void
        }
      }
    }
  }
}

export default function Login() {
  const buttonRef = useRef<HTMLDivElement>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false

    async function init() {
      try {
        const config = await api.getAuthConfig()

        // Wait for GIS script to load
        const waitForGoogle = () =>
          new Promise<void>((resolve) => {
            if (window.google?.accounts) {
              resolve()
              return
            }
            const interval = setInterval(() => {
              if (window.google?.accounts) {
                clearInterval(interval)
                resolve()
              }
            }, 100)
            setTimeout(() => {
              clearInterval(interval)
              resolve()
            }, 5000)
          })

        await waitForGoogle()

        if (cancelled || !window.google?.accounts || !buttonRef.current) return

        window.google.accounts.id.initialize({
          client_id: config.google_client_id,
          callback: async (response) => {
            try {
              await api.googleLogin(response.credential)
              window.location.href = '/'
            } catch {
              setError('Authentication failed. Please try again.')
            }
          },
        })

        window.google.accounts.id.renderButton(buttonRef.current, {
          theme: 'outline',
          size: 'large',
          width: 320,
          text: 'signin_with',
          shape: 'rectangular',
        })

        setLoading(false)
      } catch {
        setError('Failed to load authentication config.')
        setLoading(false)
      }
    }

    init()
    return () => { cancelled = true }
  }, [])

  return (
    <div className="min-h-screen flex items-center justify-center relative overflow-hidden">
      <div className="absolute inset-0 bg-background" />
      <div
        className="absolute inset-0 opacity-[0.03]"
        style={{
          backgroundImage: 'linear-gradient(rgba(245,158,11,0.5) 1px, transparent 1px), linear-gradient(90deg, rgba(245,158,11,0.5) 1px, transparent 1px)',
          backgroundSize: '60px 60px',
        }}
      />
      <div className="absolute top-1/4 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-primary/5 rounded-full blur-[100px]" />
      <div className="absolute bottom-0 left-0 w-[400px] h-[400px] bg-destructive/5 rounded-full blur-[120px]" />

      <Card className="w-full max-w-sm relative animate-scale-in border-border/60 bg-card/80 backdrop-blur-sm">
        <CardHeader className="text-center pb-2">
          <div className="flex justify-center mb-4">
            <div className="relative">
              <Bug className="h-12 w-12 text-primary relative z-10" />
              <div className="absolute inset-0 blur-lg bg-primary/20" />
            </div>
          </div>
          <CardTitle className="text-2xl font-bold tracking-tight">GoSnag</CardTitle>
          <CardDescription>Self-hosted error tracking</CardDescription>
        </CardHeader>
        <CardContent className="pt-4 flex flex-col items-center">
          {loading && !error && (
            <div className="h-[44px] w-[320px] rounded bg-muted/30 animate-pulse" />
          )}
          <div ref={buttonRef} />
          {error && (
            <p className="text-sm text-destructive mt-3">{error}</p>
          )}
          <p className="text-xs text-center text-muted-foreground mt-4">
            Secure authentication via Google
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
