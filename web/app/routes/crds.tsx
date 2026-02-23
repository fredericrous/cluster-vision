import { useMemo } from "react";
import type { Route } from "./+types/crds";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface CRDRow {
  name: string;
  group: string;
  versions: string;
  scope: string;
  cluster: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "CRDs — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("crds");
}

const columns: ColumnDef<CRDRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "group", header: "Group" },
  { accessorKey: "versions", header: "Versions" },
  { accessorKey: "scope", header: "Scope" },
  { accessorKey: "cluster", header: "Cluster" },
];

export default function CRDs({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: CRDRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "group", "scope"]}
      />
    </DiagramPage>
  );
}
