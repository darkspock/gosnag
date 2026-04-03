import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from '@/lib/auth'
import { useAuth } from '@/lib/use-auth'
import Layout from '@/components/Layout'
import Login from '@/pages/Login'
import Projects from '@/pages/Projects'
import IssueList from '@/pages/IssueList'
import IssueDetail from '@/pages/IssueDetail'
import ProjectSettings from '@/pages/ProjectSettings'
import UserManagement from '@/pages/UserManagement'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) return <div className="min-h-screen flex items-center justify-center text-muted-foreground">Loading...</div>
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route path="/" element={<Projects />} />
        <Route path="/projects/:projectId" element={<IssueList />} />
        <Route path="/projects/:projectId/issues/:issueId" element={<IssueDetail />} />
        <Route path="/projects/:projectId/settings" element={<ProjectSettings />} />
        <Route path="/users" element={<UserManagement />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  )
}
