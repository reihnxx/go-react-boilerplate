import { ChevronsUpDown, LogOut, Repeat, User } from 'lucide-react'
import { Link, useNavigate } from 'react-router'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useCurrentUser, useLogout } from '@/features/auth/hooks'
import { initials } from '@/lib/format'

export function UserMenu() {
  const me = useCurrentUser()
  const logout = useLogout()
  const navigate = useNavigate()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="h-auto gap-2 px-2 py-1.5" aria-label="Menu pengguna">
          <Avatar className="size-8">
            <AvatarFallback className="bg-primary text-xs text-primary-foreground">{initials(me.user.name)}</AvatarFallback>
          </Avatar>
          <span className="hidden text-left leading-tight md:grid">
            <span className="max-w-40 truncate text-sm font-medium">{me.user.name}</span>
            <span className="max-w-40 truncate text-xs text-muted-foreground">{me.active_role?.name}</span>
          </span>
          <ChevronsUpDown className="hidden size-4 text-muted-foreground md:block" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="font-normal">
          <div className="truncate text-sm font-medium">{me.user.name}</div>
          <div className="truncate text-xs text-muted-foreground">{me.user.email}</div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/profile">
            <User /> Profil saya
          </Link>
        </DropdownMenuItem>
        {me.roles.length > 1 && (
          <DropdownMenuItem asChild>
            <Link to="/choose-role">
              <Repeat /> Ganti otoritas
            </Link>
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          disabled={logout.isPending}
          onSelect={() => logout.mutate(undefined, { onSettled: () => navigate('/login', { replace: true }) })}
        >
          <LogOut /> Keluar
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
