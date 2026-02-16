import { useMemo, useState } from "react";
import {
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
} from "@tanstack/react-table";
import styles from "./data-table.module.css";

interface DataTableProps<T> {
  data: T[];
  columns: ColumnDef<T, string>[];
  filterColumns?: string[];
}

export function DataTable<T>({
  data,
  columns,
  filterColumns = [],
}: DataTableProps<T>) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnFilters },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  // Compute unique values for filter columns from unfiltered data
  const filterOptions = useMemo(() => {
    const opts: Record<string, string[]> = {};
    for (const colId of filterColumns) {
      const unique = new Set<string>();
      for (const row of data) {
        const val = (row as Record<string, unknown>)[colId];
        if (typeof val === "string" && val !== "") {
          unique.add(val);
        }
      }
      opts[colId] = Array.from(unique).sort();
    }
    return opts;
  }, [data, filterColumns]);

  return (
    <div className={styles.wrapper}>
      {filterColumns.length > 0 && (
        <div className={styles.filters}>
          {filterColumns.map((colId) => {
            const column = table.getColumn(colId);
            if (!column) return null;
            const currentValue = (column.getFilterValue() as string) ?? "";
            return (
              <div key={colId} className={styles.filterGroup}>
                <label className={styles.filterLabel}>
                  {column.columnDef.header as string}
                </label>
                <select
                  className={styles.filterSelect}
                  value={currentValue}
                  onChange={(e) =>
                    column.setFilterValue(e.target.value || undefined)
                  }
                >
                  <option value="">All</option>
                  {filterOptions[colId]?.map((val) => (
                    <option key={val} value={val}>
                      {val}
                    </option>
                  ))}
                </select>
              </div>
            );
          })}
          <span className={styles.rowCount}>
            {table.getFilteredRowModel().rows.length} of {data.length} rows
          </span>
        </div>
      )}

      <table className={styles.table}>
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  onClick={header.column.getToggleSortingHandler()}
                >
                  {flexRender(
                    header.column.columnDef.header,
                    header.getContext()
                  )}
                  <span className={styles.sortIndicator}>
                    {{ asc: " \u25B2", desc: " \u25BC" }[
                      header.column.getIsSorted() as string
                    ] ?? ""}
                  </span>
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function BooleanBadge({ value }: { value: string }) {
  if (value === "yes") {
    return <span className={styles.badgeYes}>yes</span>;
  }
  if (value === "no") {
    return <span className={styles.badgeNo}>no</span>;
  }
  return <>{value}</>;
}

export function OutdatedBadge({
  value,
  outdated,
}: {
  value: string;
  outdated: boolean;
}) {
  if (outdated) {
    return <span className={styles.badgeOutdated}>{value}</span>;
  }
  return <>{value}</>;
}
