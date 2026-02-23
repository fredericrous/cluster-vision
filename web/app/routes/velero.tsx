import { useMemo } from "react";
import type { Route } from "./+types/velero";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface VeleroRow {
  name: string;
  namespace: string;
  cluster: string;
  schedule: string;
  includedNS: string;
  excludedNS: string;
  ttl: string;
  phase: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Backup Schedules — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("velero");
}

const columns: ColumnDef<VeleroRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "schedule", header: "Schedule" },
  { accessorKey: "includedNS", header: "Included NS" },
  { accessorKey: "excludedNS", header: "Excluded NS" },
  { accessorKey: "ttl", header: "TTL" },
  { accessorKey: "phase", header: "Phase" },
];

export default function Velero({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: VeleroRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "phase"]}
      />
    </DiagramPage>
  );
}
