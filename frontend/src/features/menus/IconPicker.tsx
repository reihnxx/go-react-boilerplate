import { useMemo, useState } from 'react'

import { Input } from '@/components/ui/input'
import { DynamicIcon } from '@/components/dynamic-icon'
import { iconNames } from '@/lib/icons'
import { cn } from '@/lib/utils'

/** Grid picker over the curated icon registry (see src/lib/icons.ts). */
export function IconPicker({ value, onChange, invalid }: { value: string; onChange: (name: string) => void; invalid?: boolean }) {
  const [filter, setFilter] = useState('')
  const names = useMemo(() => iconNames.filter((n) => n.includes(filter.trim().toLowerCase())), [filter])

  return (
    <div className={cn('space-y-2 rounded-lg border p-2', invalid && 'border-destructive')}>
      <div className="flex items-center gap-2">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <DynamicIcon icon={value} className="size-4" />
        </div>
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder={value || 'Cari ikon…'}
          aria-label="Cari ikon"
          className="h-8"
        />
      </div>
      <div className="flex max-h-32 flex-wrap gap-1 overflow-y-auto" role="listbox" aria-label="Ikon">
        {names.map((name) => {
          return (
            <button
              key={name}
              type="button"
              role="option"
              aria-selected={value === name}
              title={name}
              onClick={() => onChange(name)}
              className={cn(
                'flex size-8 items-center justify-center rounded-md hover:bg-accent',
                value === name && 'bg-primary text-primary-foreground hover:bg-primary',
              )}
            >
              <DynamicIcon icon={name} className="size-4" />
              <span className="sr-only">{name}</span>
            </button>
          )
        })}
        {names.length === 0 && <p className="w-full py-2 text-center text-xs text-muted-foreground">Ikon tidak ditemukan.</p>}
      </div>
    </div>
  )
}
