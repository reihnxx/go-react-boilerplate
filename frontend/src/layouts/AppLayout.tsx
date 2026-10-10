import { House } from 'lucide-react'
import { Fragment, Suspense } from 'react'
import { Link, Outlet, useMatches, useNavigate } from 'react-router'
import { toast } from 'sonner'

import { LoadingState } from '@/components/states'
import { ThemeToggle } from '@/components/theme-toggle'
import { Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { SIDEBAR_STORAGE_KEY, SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import { useCurrentUser, useStopImpersonation } from '@/features/auth/hooks'
import { toastError } from '@/lib/errors'

import { AppSidebar } from './AppSidebar'
import { UserMenu } from './UserMenu'

export interface RouteHandle {
  title?: string
}

function sidebarDefaultOpen(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_STORAGE_KEY) !== 'false'
  } catch {
    return true
  }
}

export function AppLayout() {
  return (
    <SidebarProvider defaultOpen={sidebarDefaultOpen()}>
      <AppSidebar />
      <SidebarInset>
        <ImpersonationBanner />
        <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center gap-2 border-b bg-background/95 px-4 backdrop-blur supports-backdrop-filter:bg-background/80">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-1 data-[orientation=vertical]:h-4" />
          <Breadcrumbs />
          <div className="ml-auto flex items-center gap-1">
            <ThemeToggle />
            <UserMenu />
          </div>
        </header>
        <main className="flex-1 p-4 md:p-6">
          <div className="mx-auto w-full max-w-6xl space-y-6">
            <Suspense fallback={<LoadingState />}>
              <Outlet />
            </Suspense>
          </div>
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}

function Breadcrumbs() {
  const matches = useMatches()
  const crumbs = matches
    .filter((m) => (m.handle as RouteHandle | undefined)?.title)
    .map((m) => ({ title: (m.handle as RouteHandle).title!, path: m.pathname }))
  return (
    <Breadcrumb className="min-w-0">
      <BreadcrumbList>
        <BreadcrumbItem className="hidden sm:inline-flex">
          <BreadcrumbLink asChild>
            <Link to="/" aria-label="Beranda">
              <House className="size-4" />
            </Link>
          </BreadcrumbLink>
        </BreadcrumbItem>
        {crumbs.map((c, i) => (
          <Fragment key={c.path}>
            <BreadcrumbSeparator className="hidden sm:inline-flex" />
            <BreadcrumbItem className="min-w-0">
              {i === crumbs.length - 1 ? (
                <BreadcrumbPage className="truncate">{c.title}</BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <Link to={c.path}>{c.title}</Link>
                </BreadcrumbLink>
              )}
            </BreadcrumbItem>
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
}

function ImpersonationBanner() {
  const me = useCurrentUser()
  const stop = useStopImpersonation()
  const navigate = useNavigate()
  if (!me.impersonator) return null
  return (
    <div className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 bg-amber-100 px-4 py-2 text-sm text-amber-950 dark:bg-amber-950 dark:text-amber-100">
      <span>
        Anda sedang masuk sebagai <strong>{me.user.name}</strong> (oleh {me.impersonator.name}).
      </span>
      <Button
        size="sm"
        variant="outline"
        className="h-7 border-amber-400 bg-transparent"
        disabled={stop.isPending}
        onClick={() =>
          stop.mutate(undefined, {
            onSuccess: () => {
              toast.success('Kembali ke akun Anda')
              navigate('/', { replace: true })
            },
            onError: (err) => toastError(err),
          })
        }
      >
        Kembali ke akun saya
      </Button>
    </div>
  )
}
