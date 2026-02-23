import { useMemo } from "react";
import type { Route } from "./+types/workloads";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface WorkloadRow {
  name: string;
  namespace: string;
  cluster: string;
  kind: string;
  replicas: string;
  updateStrategy: string;
  images: string;
  age: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Workloads — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("workloads");
}

const columns: ColumnDef<WorkloadRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "kind", header: "Kind" },
  { accessorKey: "replicas", header: "Replicas" },
  { accessorKey: "updateStrategy", header: "Strategy" },
  { accessorKey: "images", header: "Images" },
  { accessorKey: "age", header: "Created" },
];

export default function Workloads({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: WorkloadRow[] = useMemo(() => {
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
