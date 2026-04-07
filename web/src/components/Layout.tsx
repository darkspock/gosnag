import { Link, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/toast'
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from '@/components/ui/tooltip'
import { Bug, FolderOpen, Users, LogOut, Settings } from 'lucide-react'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/', label: 'Projects', icon: FolderOpen },
  { to: '/users', label: 'Users', icon: Users },
  { to: '/admin', label: 'Tokens', icon: Settings },
]

export default function Layout() {
  const { user, logout } = useAuth()
  const location = useLocation()

  return (
    <TooltipProvider delayDuration={300}>
    <div className="min-h-screen bg-background">
      <header className="border-b border-border/60 bg-card/80 backdrop-blur-md sticky top-0 z-40">
        <div className="max-w-6xl mx-auto px-4 h-14 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <Link to="/" className="flex items-center gap-2.5 font-bold text-primary tracking-tight">
              <div className="relative">
                <Bug className="h-5 w-5 relative z-10" />
                <div className="absolute inset-0 blur-md bg-primary/30" />
              </div>
              <span className="text-lg">GoSnag</span>
            </Link>
            <nav className="flex items-center gap-1">
              {navItems.map(({ to, label, icon: Icon }) => {
                const isActive = location.pathname === to
                return (
                  <Link key={to} to={to}>
                    <Button
                      variant="ghost"
                      size="sm"
                      className={cn(
                        'gap-1.5 font-medium transition-all duration-200',
                        isActive
                          ? 'bg-primary/10 text-primary hover:bg-primary/15'
                          : 'text-muted-foreground hover:text-foreground'
                      )}
                    >
                      <Icon className="h-4 w-4" />
                      {label}
                    </Button>
                  </Link>
                )
              })}
            </nav>
          </div>
          <div className="flex items-center gap-3">
            {user && (
              <>
                <div className="flex items-center gap-2">
                  {user.avatar_url ? (
                    <img src={user.avatar_url} alt="" className="h-6 w-6 rounded-full ring-1 ring-border" />
                  ) : (
                    <div className="h-6 w-6 rounded-full bg-primary/20 flex items-center justify-center text-xs font-semibold text-primary">
                      {(user.name || user.email)[0].toUpperCase()}
                    </div>
                  )}
                  <span className="text-sm text-muted-foreground hidden sm:inline">{user.email}</span>
                </div>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="ghost" size="icon" onClick={logout} className="text-muted-foreground hover:text-foreground h-8 w-8">
                      <LogOut className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Log out</TooltipContent>
                </Tooltip>
              </>
            )}
          </div>
        </div>
      </header>
      <main className="max-w-6xl mx-auto px-4 py-6 animate-fade-in">
        <Outlet />
      </main>
      <Toaster />
    </div>
    </TooltipProvider>
  )
}
