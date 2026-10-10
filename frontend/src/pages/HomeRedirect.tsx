import { LayoutDashboard } from 'lucide-react'
import { Navigate } from 'react-router'

import { EmptyState, ErrorState, LoadingState } from '@/components/states'
import { firstPath, useNavigation, usePermission } from '@/features/auth/hooks'

/** "/" sends users to the dashboard, or to the first menu their role can open. */
export function HomeRedirect() {
  const can = usePermission()
  const nav = useNavigation()
  if (can('dashboard')) return <Navigate to="/dashboard" replace />
  if (nav.isPending) return <LoadingState />
  if (nav.error) return <ErrorState error={nav.error} onRetry={() => nav.refetch()} />
  const path = firstPath(nav.data)
  if (path) return <Navigate to={path} replace />
  return (
    <EmptyState
      className="min-h-[60vh]"
      icon={<LayoutDashboard />}
      title="Belum ada menu"
      description="Otoritas Anda belum diberi akses ke menu mana pun. Hubungi administrator."
    />
  )
}
