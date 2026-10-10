import { ChevronRight } from 'lucide-react'
import { Link, useLocation } from 'react-router'

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar'
import { useNavigation } from '@/features/auth/hooks'
import { appName } from '@/lib/config'
import { DynamicIcon } from '@/components/dynamic-icon'
import type { NavGroup, NavItem } from '@/types/api'

/** True when pathname is path or below it ("/users" matches "/users/1"). */
export function isActivePath(pathname: string, path: string | null): boolean {
  if (!path) return false
  return pathname === path || pathname.startsWith(path.endsWith('/') ? path : path + '/')
}

export function AppSidebar() {
  const { data, isPending, isError } = useNavigation()
  const { setOpenMobile } = useSidebar()

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link to="/" onClick={() => setOpenMobile(false)}>
                <img src="/favicon.svg" alt="" className="size-8 shrink-0" />
                <span className="truncate font-semibold">{appName}</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        {isPending ? (
          <SidebarGroup>
            <SidebarMenu>
              {Array.from({ length: 5 }, (_, i) => (
                <SidebarMenuItem key={i}>
                  <SidebarMenuSkeleton showIcon />
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroup>
        ) : isError ? (
          <p className="px-4 py-3 text-sm text-muted-foreground">Menu gagal dimuat.</p>
        ) : data.length === 0 ? (
          <p className="px-4 py-3 text-sm text-muted-foreground">Belum ada menu untuk otoritas ini.</p>
        ) : (
          <NavGroups groups={data} onNavigate={() => setOpenMobile(false)} />
        )}
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}

export function NavGroups({ groups, onNavigate }: { groups: NavGroup[]; onNavigate?: () => void }) {
  const { pathname } = useLocation()
  return groups.map((group) => (
    <SidebarGroup key={group.id}>
      <SidebarGroupLabel>{group.name}</SidebarGroupLabel>
      <SidebarMenu>
        {group.items.map((item) =>
          item.children.length > 0 ? (
            <ParentItem key={item.id} item={item} pathname={pathname} onNavigate={onNavigate} />
          ) : (
            <SidebarMenuItem key={item.id}>
              <SidebarMenuButton asChild isActive={isActivePath(pathname, item.path)} tooltip={item.name}>
                <Link to={item.path ?? '#'} onClick={onNavigate}>
                  <DynamicIcon icon={item.icon} />
                  <span>{item.name}</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ),
        )}
      </SidebarMenu>
    </SidebarGroup>
  ))
}

function ParentItem({ item, pathname, onNavigate }: { item: NavItem; pathname: string; onNavigate?: () => void }) {
  const childActive = item.children.some((c) => isActivePath(pathname, c.path))
  return (
    <Collapsible asChild defaultOpen={childActive} className="group/collapsible">
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton tooltip={item.name} isActive={childActive}>
            <DynamicIcon icon={item.icon} />
            <span>{item.name}</span>
            <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            {item.children.map((child) => (
              <SidebarMenuSubItem key={child.id}>
                <SidebarMenuSubButton asChild isActive={isActivePath(pathname, child.path)}>
                  <Link to={child.path ?? '#'} onClick={onNavigate}>
                    <span>{child.name}</span>
                  </Link>
                </SidebarMenuSubButton>
              </SidebarMenuSubItem>
            ))}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  )
}
