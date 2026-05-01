import { useMemo } from "react";
import type { Route } from "./+types/images";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable, ExploitBadge, OutdatedBadge, SecurityBadge } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";
import { Tooltip } from "@duro-app/ui";
import tableStyles from "../components/data-table.module.css";

interface ImageRow {
  image: string;
  tag: string;
  type: string;
  namespaces: string;
  pods: number;
  registry: string;
  latest: string;
  outdated: boolean;
  securityRisk: string;
  vulnSummary: string;
  exploitRisk: string;     // "kev" | "high-epss" | "low-epss" | "none" | ""
  exploitSummary: string;
  kevCVEs: string;          // comma-separated KEV-listed CVE IDs
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Container Images — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("images");
}

const columns: ColumnDef<ImageRow, string>[] = [
  { accessorKey: "image", header: "Image" },
  {
    accessorKey: "tag",
    header: "Tag",
    cell: ({ getValue }) => {
      const tag = getValue();
      const isSha = /^sha256:/.test(tag) || /^[0-9a-f]{40,}$/.test(tag);
      if (isSha) {
        const short = tag.replace(/^sha256:/, "").slice(0, 7);
        return (
          <Tooltip.Root content={tag}>
            <Tooltip.Trigger>
              <span>{short}</span>
            </Tooltip.Trigger>
          </Tooltip.Root>
        );
      }
      return <>{tag}</>;
    },
  },
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
  {
    accessorKey: "securityRisk",
    header: "Security",
    cell: ({ row }) => (
      <SecurityBadge
        risk={row.original.securityRisk}
        summary={row.original.vulnSummary}
      />
    ),
  },
  {
    accessorKey: "exploitRisk",
    header: "Exploit risk",
    cell: ({ row }) => (
      <ExploitBadge
        risk={row.original.exploitRisk}
        summary={row.original.exploitSummary}
      />
    ),
  },
  { accessorKey: "type", header: "Type" },
  { accessorKey: "registry", header: "Registry" },
  { accessorKey: "namespaces", header: "Namespaces", meta: { className: tableStyles.wideCell } },
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
