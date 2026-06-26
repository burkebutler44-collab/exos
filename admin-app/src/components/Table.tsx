import { useMemo, useState } from 'react'
import type { ReactNode } from 'react'

export type Column<T> = {
  key: string
  label: string
  render: (row: T) => ReactNode
  search?: (row: T) => string
}

export function DataTable<T>({
  rows,
  columns,
  placeholder = 'Search...',
}: {
  rows: T[]
  columns: Array<Column<T>>
  placeholder?: string
}) {
  const [query, setQuery] = useState('')
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return rows
    return rows.filter((row) =>
      columns.some((column) => (column.search?.(row) ?? '').toLowerCase().includes(needle)),
    )
  }, [columns, query, rows])

  return (
    <div className="admin-card overflow-hidden">
      <div className="border-b border-line p-3">
        <input className="admin-input max-w-md" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={placeholder} />
      </div>
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-left">
          <thead>
            <tr className="border-b border-line bg-gray-50">
              {columns.map((column) => (
                <th key={column.key} className="px-4 py-2.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-gray-400">
                  {column.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((row, index) => (
              <tr key={index} className="border-b border-line last:border-b-0 hover:bg-gray-50/70">
                {columns.map((column) => (
                  <td key={column.key} className="px-4 py-3 text-[12.5px] text-gray-600">
                    {column.render(row)}
                  </td>
                ))}
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={columns.length} className="px-4 py-12 text-center text-[12.5px] text-muted">
                  No records found.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <div className="border-t border-line px-4 py-2 text-[11.5px] text-muted">
        {filtered.length} of {rows.length} records
      </div>
    </div>
  )
}
