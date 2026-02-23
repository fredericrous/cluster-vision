import { useMemo } from "react";
import type { Route } from "./+types/service-map";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface ServiceMapRow {
  name: string;
  namespace: string;
  cluster: string;
  type: string;
  ports: string;
  targets: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Service Mapping — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("service-map");
}

const columns: ColumnDef<ServiceMapRow, string>[] = [
  { accessorKey: "name", header: "Service" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "type", header: "Type" },
  { accessorKey: "ports", header: "Ports" },
  { accessorKey: "targets", header: "Target Workloads" },
];

export default function ServiceMap({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: ServiceMapRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "namespace", "type"]}
      />
    </DiagramPage>
  );
}
