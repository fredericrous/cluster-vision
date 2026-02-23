import { useMemo } from "react";
import type { Route } from "./+types/network-policies";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface NetworkPolicyRow {
  name: string;
  namespace: string;
  cluster: string;
  podSelector: string;
  policyTypes: string;
  ingressSummary: string;
  egressSummary: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Network Policies — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("network-policies");
}

const columns: ColumnDef<NetworkPolicyRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "podSelector", header: "Pod Selector" },
  { accessorKey: "policyTypes", header: "Policy Types" },
  { accessorKey: "ingressSummary", header: "Ingress" },
  { accessorKey: "egressSummary", header: "Egress" },
];

export default function NetworkPolicies({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: NetworkPolicyRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "namespace"]}
      />
    </DiagramPage>
  );
}
