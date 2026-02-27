import { useMemo, useState } from "react";
import { Link } from "react-router";
import type { Route } from "./+types/applications";
import { fetchApplications } from "../../api.server";
import type { Application } from "../../api.server";
import { DataTable } from "../../components/data-table";
import { Badge } from "@fredericrous/duro-design-system";
import type { ColumnDef } from "@tanstack/react-table";
import styles from "./eam.module.css";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Applications — Cluster Vision EAM" }];
}

export async function loader() {
  const data = await fetchApplications();
  return data;
}

function StatusBadge({ status }: { status: string }) {
  const variant =
    status === "active"
      ? "success"
      : status === "maintenance"
        ? "warning"
        : status === "retired"
          ? "error"
          : "default";
  return (
    <Badge variant={variant} size="sm">
      {status}
    </Badge>
  );
}

function RiskBadge({ risk }: { risk: string }) {
  const variant =
    risk === "high"
      ? "error"
      : risk === "medium"
        ? "warning"
        : "success";
  return (
    <Badge variant={variant} size="sm">
      {risk}
    </Badge>
  );
}

const columns: ColumnDef<Application, string>[] = [
  {
    accessorKey: "name",
    header: "Name",
    cell: ({ row }) => (
      <Link
        to={`/eam/applications/${row.original.id}`}
        style={{ color: "var(--accent)", textDecoration: "none" }}
      >
        {row.original.display_name || row.original.name}
      </Link>
    ),
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ getValue }) => <StatusBadge status={getValue()} />,
  },
  {
    accessorKey: "business_criticality",
    header: "Criticality",
    cell: ({ getValue }) => <RiskBadge risk={getValue()} />,
  },
  {
    accessorKey: "technical_risk",
    header: "Risk",
    cell: ({ getValue }) => <RiskBadge risk={getValue()} />,
  },
  { accessorKey: "lifecycle_phase", header: "Lifecycle" },
  {
    accessorKey: "time_category",
    header: "TIME",
    cell: ({ getValue }) => getValue() || "—",
  },
  {
    accessorKey: "ai_confidence",
    header: "AI",
    cell: ({ getValue }) => {
      const v = parseFloat(getValue());
      if (!v) return "—";
      return `${Math.round(v * 100)}%`;
    },
  },
];

export default function Applications({ loaderData }: Route.ComponentProps) {
  const { items, total } = loaderData;

  return (
    <div className={styles.page}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1rem" }}>
        <div>
          <h1 className={styles.heading}>Applications</h1>
          <p className={styles.subtitle}>{total} applications discovered</p>
        </div>
        <Link to="/eam/applications/new">
          <button className={styles.btnPrimary}>New Application</button>
        </Link>
      </div>
      <DataTable
        data={items ?? []}
        columns={columns}
        filterColumns={["status", "business_criticality", "technical_risk", "lifecycle_phase"]}
      />
    </div>
  );
}
