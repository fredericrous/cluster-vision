import { useMemo } from "react";
import type { Route } from "./+types/rbac";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";

interface RBACRow {
  subject: string;
  subjectKind: string;
  role: string;
  roleKind: string;
  namespace: string;
  cluster: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "RBAC — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("rbac");
}

const columns: ColumnDef<RBACRow, string>[] = [
  { accessorKey: "subject", header: "Subject" },
  { accessorKey: "subjectKind", header: "Subject Kind" },
  { accessorKey: "role", header: "Role" },
  { accessorKey: "roleKind", header: "Role Kind" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
];

export default function RBAC({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: RBACRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "subjectKind", "roleKind", "namespace"]}
      />
    </DiagramPage>
  );
}
