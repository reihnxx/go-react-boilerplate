import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router'

export type SortOrder = 'asc' | 'desc'

export interface ListParams {
  page: number
  per_page: number
  q: string
  sort: string
  order: SortOrder
  filters: Record<string, string>
}

/**
 * List state (page, search, sort, filters) kept in the URL so lists are
 * shareable and survive reloads. Changing anything but the page resets to page 1.
 */
export function useListParams(defaults: { sort: string; order: SortOrder; filterKeys?: string[] }) {
  const [search, setSearch] = useSearchParams()
  const filterKeys = defaults.filterKeys

  const params = useMemo<ListParams>(() => {
    const page = Math.max(1, Number(search.get('page')) || 1)
    const filters: Record<string, string> = {}
    for (const k of filterKeys ?? []) {
      const v = search.get(k)
      if (v) filters[k] = v
    }
    const order = search.get('order')
    return {
      page,
      per_page: 20,
      q: search.get('q') ?? '',
      sort: search.get('sort') ?? defaults.sort,
      order: order === 'asc' || order === 'desc' ? order : defaults.order,
      filters,
    }
  }, [search, defaults.sort, defaults.order, filterKeys])

  const setParam = useCallback(
    (key: string, value: string | number | null) => {
      setSearch(
        (prev) => {
          const next = new URLSearchParams(prev)
          if (value === null || value === '') next.delete(key)
          else next.set(key, String(value))
          if (key !== 'page') next.delete('page')
          return next
        },
        { replace: true },
      )
    },
    [setSearch],
  )

  const toggleSort = useCallback(
    (key: string) => {
      setSearch(
        (prev) => {
          const next = new URLSearchParams(prev)
          const currentSort = prev.get('sort') ?? defaults.sort
          const currentOrder = prev.get('order') ?? defaults.order
          next.set('sort', key)
          next.set('order', currentSort === key && currentOrder === 'asc' ? 'desc' : 'asc')
          next.delete('page')
          return next
        },
        { replace: true },
      )
    },
    [setSearch, defaults.sort, defaults.order],
  )

  /** Query object for the API (filters flattened). */
  const query = useMemo(
    () => ({ page: params.page, per_page: params.per_page, q: params.q, sort: params.sort, order: params.order, ...params.filters }),
    [params],
  )

  return { params, query, setParam, toggleSort }
}
