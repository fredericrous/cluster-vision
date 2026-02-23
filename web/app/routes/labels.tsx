import { useMemo } from "react";
import type { Route } from "./+types/labels";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface LabelRow {
  key: string;
  distinctValues: number;
  resourceCount: number;
  resourceKinds: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Labels & Annotations — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("labels");
}

const columns: ColumnDef<LabelRow, string>[] = [
  { accessorKey: "key", header: "Label Key" },
  { accessorKey: "distinctValues", header: "Distinct Values" },
  { accessorKey: "resourceCount", header: "Resource Count" },
  { accessorKey: "resourceKinds", header: "Resource Kinds" },
];

export default function Labels({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: LabelRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["resourceKinds"]}
      />
    </DiagramPage>
  );
}
