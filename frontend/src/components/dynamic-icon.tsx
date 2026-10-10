import type { LucideProps } from 'lucide-react'
import { createElement } from 'react'

import { getIcon } from '@/lib/icons'

/** Renders a registry icon by name (menus store icon names in the database). */
export function DynamicIcon({ icon, ...props }: { icon: string | null | undefined } & LucideProps) {
  return createElement(getIcon(icon), { 'aria-hidden': true, ...props })
}
