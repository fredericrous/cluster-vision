import { useMemo, useState } from "react";
import {
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  flexRender,
  type Column,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
} from "@tanstack/react-table";
import {
  Badge,
  Button,
  Cluster,
  Inline,
  Select,
  Stack,
  Text,
  Tooltip,
} from "@duro-app/ui";
import { Table } from "@duro-app/ui/table";

declare module "@tanstack/react-table" {
  interface ColumnMeta<TData extends unknown, TValue> {
    /** grid-template-columns track for this column, e.g. 'minmax(200px, 400px)'. */
    width?: string;
    /** Clip overflowing cell text to a single line with an ellipsis. */
    truncate?: boolean;
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
    <Stack gap="md">
      {filterColumns.length > 0 && (
        <Cluster gap="ms" align="center">
          {filterColumns.map((colId) => {
            const column = table.getColumn(colId);
            if (!column) return null;
            const currentValue = (column.getFilterValue() as string) ?? "";
            return (
              <Inline key={colId} gap="xs" align="center">
                <Text variant="caption" color="muted" weight="medium">
                  {column.columnDef.header as string}
                </Text>
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
              </Inline>
            );
          })}
          <Text variant="caption" color="muted">
            {table.getFilteredRowModel().rows.length} of {data.length} rows
          </Text>
        </Cluster>
      )}

      <Table.Root size="sm">
        <Table.Header>
          {table.getHeaderGroups().map((headerGroup) => (
            <Table.Row key={headerGroup.id}>
              {headerGroup.headers.map((header) => {
                const label =
                  typeof header.column.columnDef.header === "string"
                    ? header.column.columnDef.header
                    : header.id;
                return (
                  <Table.HeaderCell
                    key={header.id}
                    label={label}
                    width={header.column.columnDef.meta?.width}
                  >
                    <SortableHeader column={header.column}>
                      {flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                    </SortableHeader>
                  </Table.HeaderCell>
                );
              })}
            </Table.Row>
          ))}
        </Table.Header>
        <Table.Body>
          {table.getRowModel().rows.map((row) => (
            <Table.Row key={row.id}>
              {row.getVisibleCells().map((cell) => {
                const content = flexRender(
                  cell.column.columnDef.cell,
                  cell.getContext()
                );
                return (
                  <Table.Cell key={cell.id}>
                    {cell.column.columnDef.meta?.truncate ? (
                      <Text truncate>{content}</Text>
                    ) : (
                      content
                    )}
                  </Table.Cell>
                );
              })}
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </Stack>
  );
}

/** Column header that toggles sorting. Table.HeaderCell has no onClick, so the
 *  affordance is a link-style Button — which also makes it keyboard-operable,
 *  unlike the click-only <span> this replaced. */
function SortableHeader<T>({
  column,
  children,
}: {
  column: Column<T, unknown>;
  children: React.ReactNode;
}) {
  if (!column.getCanSort()) return <>{children}</>;
  return (
    <Button variant="link" size="small" onClick={() => column.toggleSorting()}>
      <Inline gap="xs" align="center">
        {children}
        <Table.SortIndicator column={column} />
      </Inline>
    </Button>
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
