import { useMemo } from "react";
import type { Route } from "./+types/nodes";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable, OutdatedBadge } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface NodeRow {
  name: string;
  cluster: string;
  type: string;
  roles: string;
  ip: string;
  os: string;
  osVersion: string;
  latestOS: string;
  osOutdated: boolean;
  kubelet: string;
  latestKubelet: string;
  kubeletOutdated: boolean;
  containerRuntime: string;
  kernel: string;
  cpu: string;
  memory: string;
  arch: string;
  provider: string;
  gpu: string;
  osDisk: string;
  dataDisk: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Cluster Nodes — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("nodes");
}

const columns: ColumnDef<NodeRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "type", header: "Type" },
  { accessorKey: "roles", header: "Roles" },
  { accessorKey: "ip", header: "IP" },
  { accessorKey: "provider", header: "Provider" },
  {
    id: "osVersion",
    header: "OS",
    accessorFn: (row) =>
      row.osVersion ? `${row.os} ${row.osVersion}` : row.os,
  },
  {
    accessorKey: "latestOS",
    header: "Latest OS",
    cell: ({ row }) => (
      <OutdatedBadge
        value={row.original.latestOS || "-"}
        outdated={row.original.osOutdated}
      />
    ),
  },
  { accessorKey: "kubelet", header: "Kubelet" },
  {
    accessorKey: "latestKubelet",
    header: "Latest Kubelet",
    cell: ({ row }) => (
      <OutdatedBadge
        value={row.original.latestKubelet || "-"}
        outdated={row.original.kubeletOutdated}
      />
    ),
  },
  { accessorKey: "containerRuntime", header: "Runtime" },
  { accessorKey: "kernel", header: "Kernel" },
  { accessorKey: "cpu", header: "CPU" },
  { accessorKey: "memory", header: "Memory" },
  { accessorKey: "arch", header: "Arch" },
  { accessorKey: "gpu", header: "GPU" },
  { accessorKey: "osDisk", header: "OS Disk" },
  { accessorKey: "dataDisk", header: "Data Disk" },
];

export default function Nodes({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: NodeRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "type", "roles", "arch"]}
      />
    </DiagramPage>
  );
}
