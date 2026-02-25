import { useMemo } from "react";
import type { Route } from "./+types/certificates";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable, BooleanBadge } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";
import { Badge } from "@fredericrous/duro-design-system";

interface CertificateRow {
  name: string;
  namespace: string;
  cluster: string;
  dnsNames: string;
  issuer: string;
  notAfter: string;
  renewalTime: string;
  ready: string;
  expiryDays: number;
  expiryLevel: string;
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Certificates — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("certificates");
}

function ExpiryBadge({ days, level }: { days: number; level: string }) {
  if (days < 0) return <>-</>;
  if (level === "critical")
    return <Badge variant="error" size="sm">{days}d</Badge>;
  if (level === "warning")
    return <Badge variant="warning" size="sm">{days}d</Badge>;
  return <span>{days}d</span>;
}

const columns: ColumnDef<CertificateRow, string>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "namespace", header: "Namespace" },
  { accessorKey: "cluster", header: "Cluster" },
  { accessorKey: "dnsNames", header: "DNS Names" },
  { accessorKey: "issuer", header: "Issuer" },
  {
    accessorKey: "notAfter",
    header: "Expires",
    cell: ({ row }) => {
      const { notAfter, expiryDays, expiryLevel } = row.original;
      if (!notAfter) return <>-</>;
      return (
        <>
          {notAfter.slice(0, 10)}{" "}
          <ExpiryBadge days={expiryDays} level={expiryLevel} />
        </>
      );
    },
  },
  { accessorKey: "renewalTime", header: "Renewal" },
  {
    accessorKey: "ready",
    header: "Ready",
    cell: ({ getValue }) => <BooleanBadge value={getValue()} />,
  },
];

export default function Certificates({ loaderData }: Route.ComponentProps) {
  const { diagram, generatedAt } = loaderData;

  const rows: CertificateRow[] = useMemo(() => {
    if (diagram.type !== "table") return [];
    return JSON.parse(diagram.content);
  }, [diagram]);

  return (
    <DiagramPage diagram={diagram} generatedAt={generatedAt}>
      <DataTable
        data={rows}
        columns={columns}
        filterColumns={["cluster", "namespace", "issuer"]}
      />
    </DiagramPage>
  );
}
