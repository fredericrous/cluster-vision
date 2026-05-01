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
import { Badge, Select, Table, Tooltip } from "@duro-app/ui";
import styles from "./data-table.module.css";

declare module "@tanstack/react-table" {
  interface ColumnMeta<TData extends unknown, TValue> {
    className?: string;
  }
}

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
                <Select.Root
                  value={currentValue}
                  onValueChange={(v) =>
                    column.setFilterValue(v || undefined)
                  }
                >
                  <Select.Trigger>
                    <Select.Value placeholder="All" />
                    <Select.Icon />
                  </Select.Trigger>
                  <Select.Popup>
                    <Select.Item value="">
                      <Select.ItemText>All</Select.ItemText>
                    </Select.Item>
                    {filterOptions[colId]?.map((val) => (
                      <Select.Item key={val} value={val}>
                        <Select.ItemText>{val}</Select.ItemText>
                      </Select.Item>
                    ))}
                  </Select.Popup>
                </Select.Root>
              </div>
            );
          })}
          <span className={styles.rowCount}>
            {table.getFilteredRowModel().rows.length} of {data.length} rows
          </span>
        </div>
      )}

      <Table.Root columns={table.getHeaderGroups()[0]?.headers.length ?? 0} size="sm">
        <Table.Header>
          {table.getHeaderGroups().map((headerGroup) => (
            <Table.Row key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <Table.HeaderCell key={header.id}>
                  <span
                    className={styles.sortableHeader}
                    onClick={header.column.getToggleSortingHandler()}
                  >
                    {flexRender(
                      header.column.columnDef.header,
                      header.getContext()
                    )}
                    <span className={styles.sortIndicator}>
                      {{ asc: " ▲", desc: " ▼" }[
                        header.column.getIsSorted() as string
                      ] ?? ""}
                    </span>
                  </span>
                </Table.HeaderCell>
              ))}
            </Table.Row>
          ))}
        </Table.Header>
        <Table.Body>
          {table.getRowModel().rows.map((row) => (
            <Table.Row key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <Table.Cell key={cell.id}>
                  {cell.column.columnDef.meta?.className ? (
                    <span className={cell.column.columnDef.meta.className}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </span>
                  ) : (
                    flexRender(cell.column.columnDef.cell, cell.getContext())
                  )}
                </Table.Cell>
              ))}
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </div>
  );
}

export function BooleanBadge({ value }: { value: string }) {
  if (value === "yes") {
    return <Badge variant="success" size="sm">yes</Badge>;
  }
  if (value === "no") {
    return <Badge variant="default" size="sm">no</Badge>;
  }
  return <>{value}</>;
}

export function SecurityBadge({ risk, summary }: { risk: string; summary: string }) {
  if (risk === "critical")
    return (
      <Tooltip.Root content={summary}>
        <Tooltip.Trigger>
          <Badge variant="error" size="sm">critical</Badge>
        </Tooltip.Trigger>
      </Tooltip.Root>
    );
  if (risk === "warning")
    return (
      <Tooltip.Root content={summary}>
        <Tooltip.Trigger>
          <Badge variant="warning" size="sm">warning</Badge>
        </Tooltip.Trigger>
      </Tooltip.Root>
    );
  if (risk === "none")
    return <Badge variant="success" size="sm">ok</Badge>;
  return <>—</>;
}

// ExploitBadge surfaces the KEV/EPSS-derived risk tier from
// vulnExploitRisk(). "kev" is the loudest tier (CISA-confirmed active
// exploitation); "high-epss" is >50% predicted exploitation in 30d;
// "low-epss" is a watch-list signal.
export function ExploitBadge({ risk, summary }: { risk: string; summary: string }) {
  if (risk === "kev")
    return (
      <Tooltip.Root content={summary || "CISA Known Exploited Vulnerability"}>
        <Tooltip.Trigger>
          <Badge variant="error" size="sm">KEV</Badge>
        </Tooltip.Trigger>
      </Tooltip.Root>
    );
  if (risk === "high-epss")
    return (
      <Tooltip.Root content={summary}>
        <Tooltip.Trigger>
          <Badge variant="warning" size="sm">EPSS↑</Badge>
        </Tooltip.Trigger>
      </Tooltip.Root>
    );
  if (risk === "low-epss")
    return (
      <Tooltip.Root content={summary}>
        <Tooltip.Trigger>
          <Badge variant="default" size="sm">EPSS</Badge>
        </Tooltip.Trigger>
      </Tooltip.Root>
    );
  if (risk === "none")
    return <Badge variant="success" size="sm">ok</Badge>;
  return <>—</>;
}

export function OutdatedBadge({
  value,
  outdated,
}: {
  value: string;
  outdated: boolean;
}) {
  if (outdated) {
    return <Badge variant="error" size="sm">{value}</Badge>;
  }
  return <>{value}</>;
}
