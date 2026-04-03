import { useEffect, useState } from 'react'
import { api, type User } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { UserPlus, Ban, RotateCcw } from 'lucide-react'
import { toast } from '@/lib/use-toast'

const STATUS_BADGE: Record<string, 'success' | 'warning' | 'secondary' | 'error'> = {
  active: 'success',
  invited: 'warning',
  disabled: 'error',
}

export default function UserManagement() {
  const { user: currentUser } = useAuth()
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('viewer')
  const [inviting, setInviting] = useState(false)
  const [error, setError] = useState('')

  const isAdmin = currentUser?.role === 'admin'

  useEffect(() => {
    api.listUsers().then(setUsers).finally(() => setLoading(false))
  }, [])

  const refresh = async () => {
    setUsers(await api.listUsers())
  }

  const handleRoleChange = async (userId: string, role: string) => {
    await api.updateUserRole(userId, role)
    await refresh()
    toast.success(`Role updated to ${role}`)
  }

  const handleStatusChange = async (userId: string, status: string) => {
    await api.updateUserStatus(userId, status)
    await refresh()
    toast.success(`User ${status === 'disabled' ? 'disabled' : 'enabled'}`)
  }

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!inviteEmail.trim()) return
    setInviting(true)
    setError('')
    try {
      await api.inviteUser(inviteEmail.trim(), inviteRole)
      toast.success(`Invitation sent to ${inviteEmail.trim()}`)
      setInviteEmail('')
      setInviteRole('viewer')
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to invite user')
    } finally {
      setInviting(false)
    }
  }

  if (loading) return (
    <div className="text-center py-12">
      <div className="inline-block h-6 w-6 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
    </div>
  )

  return (
    <div>
      <Breadcrumb items={[
        { label: 'Projects', to: '/' },
        { label: 'Users' },
      ]} />
      <h1 className="text-2xl font-semibold mb-6">Users</h1>

      {isAdmin && (
        <form onSubmit={handleInvite} className="flex items-end gap-3 mb-6">
          <div className="flex-1">
            <label className="text-sm font-medium text-muted-foreground mb-1 block">Invite user</label>
            <Input
              type="email"
              value={inviteEmail}
              onChange={e => setInviteEmail(e.target.value)}
              placeholder="user@example.com"
              className="h-9"
              required
            />
          </div>
          <Select
            value={inviteRole}
            onChange={e => setInviteRole(e.target.value)}
            className="w-28 h-9 text-sm"
          >
            <option value="viewer">Viewer</option>
            <option value="admin">Admin</option>
          </Select>
          <Button type="submit" size="sm" disabled={inviting} className="h-9">
            <UserPlus className="h-4 w-4 mr-1" />
            {inviting ? 'Inviting...' : 'Invite'}
          </Button>
        </form>
      )}

      {error && (
        <div className="mb-4 rounded-md border border-red-500/30 bg-red-500/5 px-4 py-2 text-sm text-red-400">
          {error}
        </div>
      )}

      <div className="border rounded-lg divide-y">
        {users.map(u => (
          <div key={u.id} className="flex items-center justify-between p-4">
            <div className="flex items-center gap-3">
              {u.avatar_url ? (
                <img src={u.avatar_url} alt="" className="h-8 w-8 rounded-full ring-1 ring-border" />
              ) : (
                <div className="h-8 w-8 rounded-full bg-muted flex items-center justify-center text-sm font-medium">
                  {(u.name || u.email)[0].toUpperCase()}
                </div>
              )}
              <div>
                <div className="flex items-center gap-2">
                  <p className="font-medium text-sm">{u.name || u.email}</p>
                  <Badge variant={STATUS_BADGE[u.status] || 'secondary'} className="text-xs">
                    {u.status}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground">{u.email}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {isAdmin && u.id !== currentUser?.id ? (
                <>
                  <Select
                    value={u.role}
                    onChange={e => handleRoleChange(u.id, e.target.value)}
                    className="w-28 h-9 text-sm"
                  >
                    <option value="admin">Admin</option>
                    <option value="viewer">Viewer</option>
                  </Select>
                  {u.status === 'disabled' ? (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button variant="outline" size="sm" onClick={() => handleStatusChange(u.id, 'active')}>
                          <RotateCcw className="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Enable user</TooltipContent>
                    </Tooltip>
                  ) : (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button variant="outline" size="sm" onClick={() => handleStatusChange(u.id, 'disabled')} className="text-red-400 hover:text-red-300 hover:border-red-500/40">
                          <Ban className="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Disable user</TooltipContent>
                    </Tooltip>
                  )}
                </>
              ) : (
                <Badge variant={u.role === 'admin' ? 'default' : 'secondary'}>
                  {u.role}
                </Badge>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
