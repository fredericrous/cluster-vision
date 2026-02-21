import { useMemo } from "react";
import type { Route } from "./+types/images";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface ImageRow {
  image: string;
  tag: string;
  type: string;
  namespaces: string;
  pods: number;
  registry: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Container Images — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("images");
}

const columns: ColumnDef<ImageRow, string>[] = [
  { accessorKey: "image", header: "Image" },
  { accessorKey: "tag", header: "Tag" },
  { accessorKey: "type", header: "Type" },
  { accessorKey: "registry", header: "Registry" },
  { accessorKey: "namespaces", header: "Namespaces" },
  { accessorKey: "pods", header: "Pods" },
];

export default function Images({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: ImageRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["registry", "namespaces"]}
      />
    </DiagramPage>
  );
}
