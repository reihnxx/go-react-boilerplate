import { Inbox, RefreshCw, TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'

import { Button } from '@/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Spinner } from '@/components/ui/spinner'
import { errorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

export function FullPageLoader() {
  return (
    <div className="flex min-h-svh items-center justify-center" role="status" aria-label="Memuat">
      <Spinner className="size-6 text-muted-foreground" />
    </div>
  )
}

export function LoadingState({ label = 'Memuat data…', className }: { label?: string; className?: string }) {
  return (
    <div className={cn('flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground', className)} role="status">
      <Spinner /> {label}
    </div>
  )
}

export function EmptyState({
  title = 'Belum ada data',
  description,
  icon,
  action,
  className,
}: {
  title?: string
  description?: ReactNode
  icon?: ReactNode
  action?: ReactNode
  className?: string
}) {
  return (
    <Empty className={className}>
      <EmptyHeader>
        <EmptyMedia variant="icon">{icon ?? <Inbox />}</EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        {description && <EmptyDescription>{description}</EmptyDescription>}
      </EmptyHeader>
      {action && <EmptyContent>{action}</EmptyContent>}
    </Empty>
  )
}

export function ErrorState({ error, onRetry, className }: { error: unknown; onRetry?: () => void; className?: string }) {
  return (
    <Empty className={className}>
      <EmptyHeader>
        <EmptyMedia variant="icon" className="text-destructive">
          <TriangleAlert />
        </EmptyMedia>
        <EmptyTitle>Gagal memuat data</EmptyTitle>
        <EmptyDescription>{errorMessage(error)}</EmptyDescription>
      </EmptyHeader>
      {onRetry && (
        <EmptyContent>
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RefreshCw /> Coba lagi
          </Button>
        </EmptyContent>
      )}
    </Empty>
  )
}
