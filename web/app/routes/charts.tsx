import { useMemo } from "react";
import type { Route } from "./+types/charts";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable, OutdatedBadge } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface VersionRow {
  cluster: string;
  release: string;
  namespace: string;
  chart: string;
  version: string;
  latest: string;
  outdated: boolean;
  repoType: string;
  repoUrl: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Helm Charts — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("charts");
}

const columns: ColumnDef<VersionRow, string>[] = [
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "release", header: "Release" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "chart", header: "Chart" },
  { accessorKey: "version", header: "Version" },
  {
    accessorKey: "latest",
    header: "Latest",
    cell: ({ row }) => (
      <OutdatedBadge
        value={row.original.latest}
        outdated={row.original.outdated}
      />
    ),
  },
  { accessorKey: "repoType", header: "Repo Type" },
  { accessorKey: "repoUrl", header: "Repository" },
];

export default function Charts({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: VersionRow[] = useMemo(() => {
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
