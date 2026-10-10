import { TriangleAlert } from 'lucide-react'
import { useRouteError } from 'react-router'

import { EmptyState } from '@/components/states'
import { Button } from '@/components/ui/button'

/** Shown when a route crashes while rendering. */
export function RouteErrorPage() {
  const error = useRouteError()
  if (import.meta.env.DEV) console.error(error)
  return (
    <EmptyState
      className="min-h-svh"
      icon={<TriangleAlert />}
      title="Terjadi kesalahan"
      description="Halaman ini gagal ditampilkan. Muat ulang halaman atau kembali ke beranda."
      action={
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => window.location.reload()}>
            Muat ulang
          </Button>
          <Button onClick={() => window.location.assign('/')}>Beranda</Button>
        </div>
      }
    />
  )
}
