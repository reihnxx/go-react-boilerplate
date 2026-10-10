import { ShieldOff } from 'lucide-react'
import { Link } from 'react-router'

import { EmptyState } from '@/components/states'
import { Button } from '@/components/ui/button'

export function ForbiddenPage() {
  return (
    <EmptyState
      className="min-h-[60vh]"
      icon={<ShieldOff />}
      title="Akses ditolak"
      description="Otoritas Anda saat ini tidak memiliki izin untuk membuka halaman ini."
      action={
        <Button asChild variant="outline">
          <Link to="/">Kembali ke beranda</Link>
        </Button>
      }
    />
  )
}
