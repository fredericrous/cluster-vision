import { useMemo } from "react";
import type { Route } from "./+types/quotas";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface QuotaRow {
  name: string;
  namespace: string;
  cluster: string;
  kind: string;
  resources: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Resource Quotas — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("quotas");
}

const columns: ColumnDef<QuotaRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "kind", header: "Kind" },
  { accessorKey: "resources", header: "Resources" },
];

export default function Quotas({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: QuotaRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "namespace", "kind"]}
      />
    </DiagramPage>
  );
}
