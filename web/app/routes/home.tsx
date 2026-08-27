import type { Route } from "./+types/home";
import { fetchConfig, fetchDiagrams, fetchSnapshots, type Snapshot } from "../api.server";
import { Badge, Button, Card, Grid, Heading, Inline, Stack, Text } from "@duro-app/ui";
import { Separator } from "@base-ui/react/separator";
import { Link, useNavigate } from "react-router";
import { formatWhen, shortSha } from "../lib/compare";
import { routeForDiagram } from "../components/compare-bar";

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
  const [data, config] = await Promise.all([fetchDiagrams(), fetchConfig()]);
  let recent: Snapshot[] = [];
  if (config.snapshots) {
    try {
      recent = (await fetchSnapshots(10)).filter(
        (s) => s.summary.previous_id !== null
      );
    } catch {
      recent = [];
    }
  }
  return {
    diagramCount: data.diagrams.length,
    diagrams: data.diagrams.map((d) => ({ id: d.id, title: d.title })),
    generatedAt: data.generated_at,
    snapshotsEnabled: config.snapshots,
    recent,
  };
}

/** The home page's answer to "what changed?": one row per snapshot that
 *  differs from the one before it, deep-linking into compare mode with
 *  both ends pinned so the link stays stable in a postmortem. */
function RecentChanges({ recent }: { recent: Snapshot[] }) {
  const navigate = useNavigate();
  if (recent.length === 0) {
    return (
      <Text variant="caption" color="muted">
        Change history starts now — this list fills in after the next cluster change.
      </Text>
    );
  }
  return (
    <Stack gap="sm">
      {recent.map((s) => {
        const total = s.summary.total.added + s.summary.total.removed + s.summary.total.changed;
        const views = Object.entries(s.summary.diagrams);
        const first = views[0]?.[0];
        const to = `${first ? routeForDiagram(first) : "/dependencies"}?before=${s.summary.previous_id}&after=${s.id}`;
        return (
          <Card key={s.id} variant="interactive" size="compact" onClick={() => navigate(to)}>
            <Stack gap="xs">
              <Inline gap="sm" align="baseline">
                <Text variant="bodySm" weight="semibold">
                  {formatWhen(s.taken_at)} · {shortSha(s)}
                </Text>
                <Text variant="caption" color="muted">
                  {total} change{total === 1 ? "" : "s"} across {views.length} view{views.length === 1 ? "" : "s"}
                </Text>
                {s.new_revision && <Badge variant="info" size="sm">deploy</Badge>}
                {s.summary.drift && (
                  <Badge variant="warning" size="sm">changed with no new commit</Badge>
                )}
              </Inline>
              <Inline gap="xs">
                {views.slice(0, 6).map(([id, sum]) => (
                  <Badge key={id} variant="default" size="sm">
                    {id} {sum.added + sum.removed + sum.changed}
                  </Badge>
                ))}
                {views.length > 6 && (
                  <Text variant="caption" color="muted">+{views.length - 6} more</Text>
                )}
              </Inline>
            </Stack>
          </Card>
        );
      })}
    </Stack>
  );
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
  const { generatedAt, snapshotsEnabled, recent } = loaderData;
  const formattedTime = new Date(generatedAt).toLocaleString();
  const navigate = useNavigate();

  return (
    <Stack gap="lg">
      <Stack gap="xs">
        <Heading level={1} variant="headingLg">
          Cluster Vision
        </Heading>
        <Text color="muted">
          Auto-generated infrastructure diagrams from live Kubernetes state
        </Text>
        <Text variant="caption" color="muted">
          Last refresh: {formattedTime}
        </Text>
      </Stack>
      {snapshotsEnabled && (
        <>
          <Separator />
          <Stack gap="sm">
            <Inline gap="md" align="baseline">
              <Heading level={2} variant="headingSm">
                Recent changes
              </Heading>
              <Button
                variant="link"
                size="small"
                onClick={() => navigate("/dependencies?before=deploy")}
              >
                Since last deploy
              </Button>
            </Inline>
            <RecentChanges recent={recent} />
          </Stack>
        </>
      )}
      <Separator />
      <Grid minColumnWidth="280px" gap="md">
        {cards.map((card) => (
          <Link to={card.to} key={card.id} style={{ textDecoration: "none" }}>
            <Card variant="interactive" header={card.title}>
              <Text variant="bodySm" color="muted">
                {card.description}
              </Text>
            </Card>
          </Link>
        ))}
      </Grid>
    </Stack>
  );
}
