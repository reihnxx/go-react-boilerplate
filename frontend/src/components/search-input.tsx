import { Search } from 'lucide-react'
import { useEffect, useState } from 'react'

import { Input } from '@/components/ui/input'
import { useDebouncedValue } from '@/hooks/use-debounce'

/** Debounced search box; calls onChange after the user stops typing. */
export function SearchInput({
  value,
  onChange,
  placeholder = 'Cari…',
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  const [text, setText] = useState(value)
  const [prevValue, setPrevValue] = useState(value)
  const debounced = useDebouncedValue(text)

  // Follow external changes (e.g. back/forward navigation) without an effect.
  if (value !== prevValue) {
    setPrevValue(value)
    setText(value)
  }
  useEffect(() => {
    if (debounced !== value) onChange(debounced)
    // only react to the debounced text
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debounced])

  return (
    <div className="relative w-full sm:max-w-xs">
      <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        type="search"
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={placeholder}
        className="pl-8"
        aria-label={placeholder}
      />
    </div>
  )
}
