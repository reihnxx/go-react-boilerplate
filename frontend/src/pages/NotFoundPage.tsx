import { Ghost } from 'lucide-react'
import { Link } from 'react-router'

import { EmptyState } from '@/components/states'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function NotFoundPage({ fullScreen = false }: { fullScreen?: boolean }) {
  return (
    <EmptyState
      className={cn(fullScreen ? 'min-h-svh' : 'min-h-[60vh]')}
      icon={<Ghost />}
      title="Halaman tidak ditemukan"
      description="Alamat yang Anda buka tidak ada atau sudah dipindahkan."
      action={
        <Button asChild variant="outline">
          <Link to="/">Kembali ke beranda</Link>
        </Button>
      }
    />
  )
}
