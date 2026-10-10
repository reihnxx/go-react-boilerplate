import { ArrowRight } from 'lucide-react'
import { Link } from 'react-router'

import { PageHeader } from '@/components/page-header'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useCurrentUser, useNavigation } from '@/features/auth/hooks'
import { DynamicIcon } from '@/components/dynamic-icon'
import type { NavItem } from '@/types/api'

export function DashboardPage() {
  const me = useCurrentUser()
  const nav = useNavigation()
  const shortcuts: NavItem[] = (nav.data ?? [])
    .flatMap((g) => g.items.flatMap((i) => (i.children.length > 0 ? i.children : [i])))
    .filter((i) => i.path && i.slug !== 'dashboard')

  return (
    <>
      <PageHeader title={`Selamat datang, ${me.user.name}!`} description={`Anda masuk sebagai ${me.active_role?.name ?? '—'}.`} />
      {shortcuts.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-sm font-medium text-muted-foreground">Akses cepat</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {shortcuts.map((item) => {
              return (
                <Link
                  key={item.id}
                  to={item.path!}
                  className="group rounded-xl focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                >
                  <Card className="h-full transition-colors group-hover:border-primary/40 group-hover:bg-accent/40">
                    <CardHeader>
                      <div className="mb-2 flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
                        <DynamicIcon icon={item.icon} className="size-5" />
                      </div>
                      <CardTitle className="flex items-center justify-between text-base">
                        {item.name}
                        <ArrowRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
                      </CardTitle>
                      <CardDescription>{item.path}</CardDescription>
                    </CardHeader>
                  </Card>
                </Link>
              )
            })}
          </div>
        </section>
      )}
      {shortcuts.length === 0 && !nav.isPending && (
        <Card>
          <CardContent className="py-6 text-sm text-muted-foreground">Belum ada menu lain yang dapat Anda akses.</CardContent>
        </Card>
      )}
    </>
  )
}
