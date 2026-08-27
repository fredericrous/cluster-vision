import { useMemo } from "react";
import type { Route } from "./+types/certificates";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";
import { DataTable, BooleanBadge } from "../components/data-table";
import type { ColumnDef } from "@tanstack/react-table";
import { Badge, Text } from "@duro-app/ui";

interface CertificateRow {
  name: string;
  namespace: string;
  cluster: string;
  dnsNames: string;
  issuer: string;
  notAfter: string;
  renewalTime: string;
  ready: string;
}

// Days-until-expiry is derived here, not by the API: generators stay free
// of wall-clock reads so cluster snapshots hash and diff cleanly.
function expiry(notAfter: string): { days: number; level: string } {
  if (!notAfter) return { days: -1, level: "ok" };
  const t = Date.parse(notAfter);
  if (Number.isNaN(t)) return { days: -1, level: "ok" };
  const days = Math.floor((t - Date.now()) / 86_400_000);
  const level = days < 30 ? "critical" : days < 90 ? "warning" : "ok";
  return { days, level };
}

export function meta({}: Route.MetaArgs) {
  return [{ title: "Certificates — Cluster Vision" }];
}

export async function loader({ request }: Route.LoaderArgs) {
  return fetchDiagram("certificates", request);
}

function ExpiryBadge({ days, level }: { days: number; level: string }) {
  if (days < 0) return <>-</>;
  if (level === "critical")
    return <Badge variant="error" size="sm">{days}d</Badge>;
  if (level === "warning")
    return <Badge variant="warning" size="sm">{days}d</Badge>;
  // caption (12px) matches the size="sm" table cell these render in.
  return <Text variant="caption">{days}d</Text>;
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
      const { notAfter } = row.original;
      if (!notAfter) return <>-</>;
      const { days, level } = expiry(notAfter);
      return (
        <>
          {notAfter.slice(0, 10)}{" "}
          <ExpiryBadge days={days} level={level} />
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
