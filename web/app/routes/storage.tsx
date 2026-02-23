import { useMemo } from "react";
import type { Route } from "./+types/storage";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface StorageRow {
  name: string;
  namespace: string;
  cluster: string;
  kind: string;
  capacity: string;
  accessModes: string;
  status: string;
  storageClass: string;
  reclaimPolicy: string;
  boundTo: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Storage — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("storage");
}

const columns: ColumnDef<StorageRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "kind", header: "Kind" },
  { accessorKey: "capacity", header: "Capacity" },
  { accessorKey: "accessModes", header: "Access Modes" },
  { accessorKey: "status", header: "Status" },
  { accessorKey: "storageClass", header: "Storage Class" },
  { accessorKey: "reclaimPolicy", header: "Reclaim Policy" },
  { accessorKey: "boundTo", header: "Bound To" },
];

export default function Storage({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: StorageRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "kind", "status", "storageClass"]}
      />
    </DiagramPage>
  );
}
