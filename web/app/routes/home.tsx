import type { Route } from "./+types/home";
import { fetchDiagrams } from "../api.server";
import { Card } from "@fredericrous/duro-design-system";
import { Separator } from "@base-ui/react/separator";
import { Link } from "react-router";
import styles from "./home.module.css";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Cluster Vision" },
    {
      name: "description",
      content: "Infrastructure diagrams from live Kubernetes state",
    },
  ];
}

export async function loader() {
  const data = await fetchDiagrams();
  return {
    diagramCount: data.diagrams.length,
    diagrams: data.diagrams.map((d) => ({ id: d.id, title: d.title })),
    generatedAt: data.generated_at,
  };
}

const cards = [
  {
    id: "topology",
    title: "Physical Topology",
    description: "Hardware, VMs, and cluster nodes",
    to: "/topology",
  },
  {
    id: "dependencies",
    title: "Flux Dependencies",
    description: "GitOps Kustomization dependency graph",
    to: "/dependencies",
  },
  {
    id: "network",
    title: "Network & Ingress",
    description: "External URLs, gateways, and routing",
    to: "/network",
  },
  {
    id: "security",
    title: "Security Matrix",
    description: "Istio, mTLS, auth, backup, and pod security",
    to: "/security",
  },
  {
    id: "nodes",
    title: "Cluster Nodes",
    description: "Node hardware, OS versions, and update status",
    to: "/nodes",
  },
  {
    id: "charts",
    title: "Helm Charts",
    description: "Helm chart versions and update status",
    to: "/charts",
  },
  {
    id: "images",
    title: "Container Images",
    description: "Container images running across the cluster with versions",
    to: "/images",
  },
  {
    id: "workloads",
    title: "Workloads",
    description: "Deployments, StatefulSets, DaemonSets, and CronJobs",
    to: "/workloads",
  },
  {
    id: "storage",
    title: "Storage",
    description: "PVs, PVCs, and StorageClasses",
    to: "/storage",
  },
  {
    id: "crds",
    title: "Custom Resource Definitions",
    description: "Installed CRDs across clusters",
    to: "/crds",
  },
  {
    id: "quotas",
    title: "Resource Quotas",
    description: "Quotas and limit ranges per namespace",
    to: "/quotas",
  },
  {
    id: "certificates",
    title: "Certificates",
    description: "TLS certificates with expiry tracking",
    to: "/certificates",
  },
  {
    id: "network-policies",
    title: "Network Policies",
    description: "Pod-level network access controls",
    to: "/network-policies",
  },
  {
    id: "configs",
    title: "ConfigMaps & Secrets",
    description: "Configuration resources with key counts",
    to: "/configs",
  },
  {
    id: "rbac",
    title: "RBAC Inventory",
    description: "Role bindings and access permissions",
    to: "/rbac",
  },
  {
    id: "labels",
    title: "Labels & Annotations",
    description: "Label taxonomy across resources",
    to: "/labels",
  },
  {
    id: "velero",
    title: "Backup Schedules",
    description: "Velero backup schedule configuration",
    to: "/velero",
  },
  {
    id: "helm-workloads",
    title: "Helm to Workloads",
    description: "Helm releases mapped to managed workloads",
    to: "/helm-workloads",
  },
  {
    id: "service-map",
    title: "Service Mapping",
    description: "Services mapped to target workloads",
    to: "/service-map",
  },
  {
    id: "namespace-summary",
    title: "Namespace Summary",
    description: "Resource counts aggregated per namespace",
    to: "/namespace-summary",
  },
];

export default function Home({ loaderData }: Route.ComponentProps) {
  const { generatedAt } = loaderData;
  const formattedTime = new Date(generatedAt).toLocaleString();

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>Cluster Vision</h1>
      <p className={styles.subtitle}>
        Auto-generated infrastructure diagrams from live Kubernetes state
      </p>
      <span className={styles.generatedAt}>Last refresh: {formattedTime}</span>
      <Separator />
      <div className={styles.grid}>
        {cards.map((card) => (
          <Link to={card.to} key={card.id} style={{ textDecoration: "none" }}>
            <Card variant="interactive" header={card.title}>
              <p className={styles.cardDesc}>{card.description}</p>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
