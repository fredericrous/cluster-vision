import type { Route } from "./+types/home";
import { fetchDiagrams } from "../api.server";
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
          <Link to={card.to} key={card.id} className={styles.card}>
            <h3 className={styles.cardTitle}>{card.title}</h3>
            <p className={styles.cardDesc}>{card.description}</p>
          </Link>
        ))}
      </div>
    </div>
  );
}
