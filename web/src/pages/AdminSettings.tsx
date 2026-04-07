import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { useAuth } from '@/lib/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { Copy, Key, Plus, Trash2 } from 'lucide-react'
import { toast } from '@/lib/use-toast'

interface GlobalToken {
  id: string
  name: string
  permission: string
  scope: string
  token?: string
  last_used_at: string | null
  expires_at: string | null
  created_at: string
}

export default function AdminSettings() {
  const { user } = useAuth()
  const [tokens, setTokens] = useState<GlobalToken[]>([])
  const [showForm, setShowForm] = useState(false)
  const [tokenName, setTokenName] = useState('')
  const [tokenPermission, setTokenPermission] = useState('readwrite')
  const [tokenExpiresIn, setTokenExpiresIn] = useState('')
  const [newToken, setNewToken] = useState<string | null>(null)
  const [showDelete, setShowDelete] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    loadTokens()
  }, [])

  const loadTokens = async () => {
    try {
      const data = await api.listGlobalTokens()
      setTokens(data)
    } catch {
      // Not admin or no tokens
    }
  }

  const handleCreate = async () => {
    try {
      const data: Record<string, unknown> = { name: tokenName, permission: tokenPermission }
      if (tokenExpiresIn) data.expires_in = parseInt(tokenExpiresIn)
      const resp = await api.createGlobalToken(data as { name: string; permission: string; expires_in?: number })
      setNewToken(resp.token || null)
      loadTokens()
      toast.success('Token created')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create token')
    }
  }

  const handleDelete = async (id: string) => {
    await api.deleteGlobalToken(id)
    setShowDelete(null)
    loadTokens()
    toast.success('Token deleted')
  }

  const handleCopy = () => {
    if (newToken) {
      navigator.clipboard.writeText(newToken)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      <Breadcrumb items={[{ label: 'Personal Tokens' }]} />

      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Personal Access Tokens</h1>
        <p className="text-sm text-muted-foreground">Tokens for API access, MCP servers, and automation. Each token inherits your permissions.</p>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base flex items-center gap-2">
                <Key className="h-4 w-4" /> My Tokens
              </CardTitle>
              <CardDescription>
                Personal access tokens for API, MCP servers, and automation. Tokens inherit your role ({user?.role}).
              </CardDescription>
            </div>
            <Button size="sm" onClick={() => { setShowForm(true); setNewToken(null); setTokenName(''); setCopied(false) }}>
              <Plus className="h-4 w-4 mr-1" /> New Token
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {tokens.length === 0 ? (
            <p className="text-sm text-muted-foreground">No personal tokens yet.</p>
          ) : (
            <div className="space-y-3">
              {tokens.map(t => (
                <div key={t.id} className="flex items-center justify-between rounded-lg border p-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium">{t.name}</p>
                      <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-primary/10 text-primary/80">{t.permission}</span>
                    </div>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      Created {new Date(t.created_at).toLocaleDateString()}
                      {t.last_used_at && ` \u00b7 Last used ${new Date(t.last_used_at).toLocaleDateString()}`}
                      {t.expires_at && ` \u00b7 Expires ${new Date(t.expires_at).toLocaleDateString()}`}
                    </p>
                  </div>
                  <Button variant="ghost" size="icon" onClick={() => setShowDelete(t.id)}>
                    <Trash2 className="h-4 w-4 text-muted-foreground hover:text-destructive" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Create Token Dialog */}
      <Dialog open={showForm} onOpenChange={open => { if (!open) { setShowForm(false); setNewToken(null) } }}>
        <DialogContent>
          <DialogTitle>Create Personal Token</DialogTitle>
          <DialogDescription className="sr-only">Create a personal access token</DialogDescription>
          {newToken ? (
            <div className="mt-4 space-y-4">
              <p className="text-sm text-amber-400">Copy this token now. It won't be shown again.</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 bg-muted px-3 py-2 rounded text-sm font-mono break-all">{newToken}</code>
                <Button variant="outline" size="icon" onClick={handleCopy}>
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
              {copied && <p className="text-xs text-emerald-400">Copied!</p>}
              <div className="mt-2">
                <p className="text-xs font-medium text-muted-foreground mb-1.5">Example usage:</p>
                <pre className="bg-muted px-3 py-2 rounded text-xs font-mono break-all whitespace-pre-wrap text-muted-foreground">
{`curl -H "Authorization: Bearer ${newToken}" \\
  "${window.location.origin}/api/v1/projects"`}
                </pre>
              </div>
              <div className="flex justify-end">
                <Button onClick={() => { setShowForm(false); setNewToken(null) }}>Done</Button>
              </div>
            </div>
          ) : (
            <div className="mt-4 space-y-4">
              <div>
                <label className="text-sm font-medium">Name</label>
                <Input
                  value={tokenName}
                  onChange={e => setTokenName(e.target.value)}
                  placeholder="e.g. MCP Server, CI/CD Pipeline"
                  className="mt-1"
                />
              </div>
              <div>
                <label className="text-sm font-medium">Permission</label>
                <Select value={tokenPermission} onChange={e => setTokenPermission(e.target.value)} className="mt-1">
                  <option value="read">Read only</option>
                  <option value="readwrite">Read & Write (full access)</option>
                </Select>
              </div>
              <div>
                <label className="text-sm font-medium">Expires in</label>
                <Select value={tokenExpiresIn} onChange={e => setTokenExpiresIn(e.target.value)} className="mt-1">
                  <option value="">Never</option>
                  <option value="30">30 days</option>
                  <option value="90">90 days</option>
                  <option value="365">1 year</option>
                </Select>
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={() => setShowForm(false)}>Cancel</Button>
                <Button onClick={handleCreate} disabled={!tokenName}>Create</Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!showDelete}
        onOpenChange={() => setShowDelete(null)}
        title="Delete Token"
        description="This token will stop working immediately. Any integrations using it will lose access."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => showDelete && handleDelete(showDelete)}
      />
    </div>
  )
}
