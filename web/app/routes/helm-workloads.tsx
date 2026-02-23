import { useMemo } from "react";
import type { Route } from "./+types/helm-workloads";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface HelmWorkloadRow {
  release: string;
  namespace: string;
  cluster: string;
  workload: string;
  kind: string;
  replicas: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Helm to Workloads — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("helm-workloads");
}

const columns: ColumnDef<HelmWorkloadRow, string>[] = [
  { accessorKey: "release", header: "Helm Release" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "workload", header: "Workload" },
  { accessorKey: "kind", header: "Kind" },
  { accessorKey: "replicas", header: "Replicas" },
];

export default function HelmWorkloads({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: HelmWorkloadRow[] = useMemo(() => {
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
