import { useMemo } from "react";
import type { Route } from "./+types/security";
import { fetchDiagrams } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import {
  DataTable,
  BooleanBadge,
} from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface SecurityRow {
  cluster: string;
  namespace: string;
  ingress: string;
  ambient: string;
  mtls: string;
  mtlsClient: string;
  extAuth: string;
  backup: string;
  podSecurity: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Security — Cluster Vision" }];
}

export async function loader() {
  const data = await fetchDiagrams();
  const table = data.diagrams.find((d) => d.id === "security");
  const chart = data.diagrams.find((d) => d.id === "security-chart");
  if (!table) {
    throw new Error('Diagram "security" not found');
  }
  return { table, chart, generatedAt: data.generated_at };
}

const columns: ColumnDef<SecurityRow, string>[] = [
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "namespace", header: "Namespace" },
  {
    accessorKey: "ingress",
    header: "Ingress",
    cell: ({ getValue }) => <BooleanBadge value={getValue()} />,
  },
  {
    accessorKey: "ambient",
    header: "Istio Ambient",
    cell: ({ getValue }) => <BooleanBadge value={getValue()} />,
  },
  {
    accessorKey: "mtls",
    header: "mTLS",
    cell: ({ getValue }) => <BooleanBadge value={getValue()} />,
  },
  {
    accessorKey: "mtlsClient",
    header: "mTLS Client",
    cell: ({ getValue }) => <BooleanBadge value={getValue()} />,
  },
  {
    accessorKey: "extAuth",
    header: "Ext Auth",
    cell: ({ getValue }) => <BooleanBadge value={getValue()} />,
  },
  {
    accessorKey: "backup",
    header: "Backup",
    cell: ({ getValue }) => <BooleanBadge value={getValue()} />,
  },
  { accessorKey: "podSecurity", header: "Pod Security" },
];

export default function Security({ loaderData }: Route.ComponentProps) {
  const { table, chart, generatedAt } = loaderData;

  const rows: SecurityRow[] = useMemo(() => {
    if (table.type !== "table") return [];
    return JSON.parse(table.content);
  }, [table]);

  return (
    <>
      <DiagramPage diagram={table} generatedAt={generatedAt}>
        <DataTable
          data={rows}
          columns={columns}
          filterColumns={["cluster", "namespace"]}
        />
      </DiagramPage>
      {chart && (
        <DiagramPage diagram={chart} generatedAt={generatedAt} />
      )}
    </>
  );
}
