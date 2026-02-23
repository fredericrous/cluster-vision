import { useMemo } from "react";
import type { Route } from "./+types/namespace-summary";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface NamespaceSummaryRow {
  namespace: string;
  cluster: string;
  workloads: number;
  services: number;
  configMaps: number;
  secrets: number;
  pvcs: number;
  certificates: number;
  networkPolicies: number;
  helmReleases: number;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Namespace Summary — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("namespace-summary");
}

const columns: ColumnDef<NamespaceSummaryRow, string>[] = [
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "workloads", header: "Workloads" },
  { accessorKey: "services", header: "Services" },
  { accessorKey: "configMaps", header: "ConfigMaps" },
  { accessorKey: "secrets", header: "Secrets" },
  { accessorKey: "pvcs", header: "PVCs" },
  { accessorKey: "certificates", header: "Certificates" },
  { accessorKey: "networkPolicies", header: "Net Policies" },
  { accessorKey: "helmReleases", header: "Helm Releases" },
];

export default function NamespaceSummary({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: NamespaceSummaryRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster"]}
      />
    </DiagramPage>
  );
}
