import { useMemo } from "react";
import type { Route } from "./+types/components";
import { fetchComponents } from "../../api.server";
import type { ITComponent } from "../../api.server";
import { DataTable } from "../../components/data-table";
import { Badge } from "@duro-app/ui";
import type { ColumnDef } from "@tanstack/react-table";
import styles from "./eam.module.css";

export function meta({}: Route.MetaArgs) {
  return [{ title: "IT Components — Cluster Vision EAM" }];
}

export async function loader() {
  return fetchComponents();
}

const typeColors: Record<string, string> = {
  compute: "default",
  storage: "default",
  database: "default",
  messaging: "default",
  network: "default",
  runtime: "default",
  observability: "default",
  security: "default",
};

const columns: ColumnDef<ITComponent, string>[] = [
  { accessorKey: "name", header: "Name" },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ getValue }) => (
      <Badge variant="default" size="sm">
        {getValue()}
      </Badge>
    ),
  },
  {
    accessorKey: "version",
    header: "Version",
    cell: ({ getValue }) => getValue() || "—",
  },
  {
    accessorKey: "provider",
    header: "Provider",
    cell: ({ getValue }) => getValue() || "—",
  },
  { accessorKey: "status", header: "Status" },
];

export default function Components({ loaderData }: Route.ComponentProps) {
  const components = loaderData ?? [];

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>IT Components</h1>
      <p className={styles.subtitle}>
        Infrastructure components auto-discovered from cluster nodes and storage classes.
      </p>
      <DataTable data={components} columns={columns} filterColumns={["type", "status"]} />
    </div>
  );
}
