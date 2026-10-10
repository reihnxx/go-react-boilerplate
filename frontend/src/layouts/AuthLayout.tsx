import { Suspense } from 'react'
import { Outlet } from 'react-router'

import { LoadingState } from '@/components/states'

import { ThemeToggle } from '@/components/theme-toggle'
import { appName } from '@/lib/config'

/** Centered card layout for login, password reset, registration and role choice. */
export function AuthLayout() {
  return (
    <div className="relative flex min-h-svh flex-col items-center justify-center bg-muted/40 px-4 py-10">
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>
      <div className="mb-6 flex items-center gap-2.5">
        <img src="/favicon.svg" alt="" className="size-9" />
        <span className="text-lg font-semibold tracking-tight">{appName}</span>
      </div>
      <div className="w-full max-w-sm">
        <Suspense fallback={<LoadingState />}>
          <Outlet />
        </Suspense>
      </div>
    </div>
  )
}
