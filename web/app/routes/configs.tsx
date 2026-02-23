import { useMemo } from "react";
import type { Route } from "./+types/configs";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface ConfigRow {
  name: string;
  namespace: string;
  cluster: string;
  kind: string;
  keyCount: number;
  referencedBy: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "ConfigMaps & Secrets — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("configs");
}

const columns: ColumnDef<ConfigRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "kind", header: "Kind" },
  { accessorKey: "keyCount", header: "Keys" },
  { accessorKey: "referencedBy", header: "Referenced By" },
];

export default function Configs({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: ConfigRow[] = useMemo(() => {
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
