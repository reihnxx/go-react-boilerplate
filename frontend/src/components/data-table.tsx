import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight } from 'lucide-react'
import type { ReactNode } from 'react'

import { EmptyState, ErrorState } from '@/components/states'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { SortOrder } from '@/hooks/use-list-params'
import { cn } from '@/lib/utils'
import type { PageMeta } from '@/types/api'

export interface Column<T> {
  key: string
  header: ReactNode
  /** API sort key; makes the header clickable. */
  sortKey?: string
  cell: (row: T) => ReactNode
  className?: string
}

interface DataTableProps<T> {
  columns: Column<T>[]
  rows: T[] | undefined
  rowKey: (row: T) => string
  isLoading?: boolean
  error?: unknown
  onRetry?: () => void
  sort?: { key: string; order: SortOrder }
  onSort?: (key: string) => void
  empty?: ReactNode
  meta?: PageMeta
  onPageChange?: (page: number) => void
}

/** Server-driven table: sorting and pagination are delegated to the caller. */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  isLoading,
  error,
  onRetry,
  sort,
  onSort,
  empty,
  meta,
  onPageChange,
}: DataTableProps<T>) {
  return (
    <div className="space-y-3">
      <div className="overflow-hidden rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40 hover:bg-muted/40">
              {columns.map((col) => (
                <TableHead key={col.key} className={col.className} aria-sort={ariaSort(col, sort)}>
                  {col.sortKey && onSort ? (
                    <button
                      type="button"
                      className="-ml-1 inline-flex items-center gap-1 rounded px-1 py-0.5 hover:bg-accent hover:text-accent-foreground"
                      onClick={() => onSort(col.sortKey!)}
                    >
                      {col.header}
                      <SortIcon active={sort?.key === col.sortKey} order={sort?.order} />
                    </button>
                  ) : (
                    col.header
                  )}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }, (_, i) => (
                <TableRow key={i}>
                  {columns.map((col) => (
                    <TableCell key={col.key} className={col.className}>
                      <Skeleton className="h-4 w-full max-w-40" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : error ? (
              <TableRow>
                <TableCell colSpan={columns.length}>
                  <ErrorState error={error} onRetry={onRetry} />
                </TableCell>
              </TableRow>
            ) : !rows || rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length}>{empty ?? <EmptyState />}</TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (
                <TableRow key={rowKey(row)}>
                  {columns.map((col) => (
                    <TableCell key={col.key} className={col.className}>
                      {col.cell(row)}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
      {meta && onPageChange && <Pagination meta={meta} onPageChange={onPageChange} />}
    </div>
  )
}

function ariaSort<T>(col: Column<T>, sort?: { key: string; order: SortOrder }) {
  if (!col.sortKey || sort?.key !== col.sortKey) return undefined
  return sort.order === 'asc' ? 'ascending' : 'descending'
}

function SortIcon({ active, order }: { active: boolean; order?: SortOrder }) {
  if (!active) return <ArrowUpDown className="size-3.5 opacity-50" />
  return order === 'asc' ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />
}

export function Pagination({ meta, onPageChange }: { meta: PageMeta; onPageChange: (page: number) => void }) {
  const from = meta.total === 0 ? 0 : (meta.page - 1) * meta.per_page + 1
  const to = Math.min(meta.page * meta.per_page, meta.total)
  const pages = Math.max(meta.total_pages, 1)
  return (
    <div className={cn('flex flex-col items-center justify-between gap-2 text-sm text-muted-foreground sm:flex-row')}>
      <p>
        Menampilkan {from}–{to} dari {meta.total} data
      </p>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={meta.page <= 1}
          onClick={() => onPageChange(meta.page - 1)}
          aria-label="Halaman sebelumnya"
        >
          <ChevronLeft />
        </Button>
        <span className="tabular-nums">
          Hal. {meta.page} / {pages}
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={meta.page >= pages}
          onClick={() => onPageChange(meta.page + 1)}
          aria-label="Halaman berikutnya"
        >
          <ChevronRight />
        </Button>
      </div>
    </div>
  )
}
