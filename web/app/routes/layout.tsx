import { Outlet, useLocation, useNavigate } from "react-router";
import { Heading, SideNav, Stack, Text } from "@duro-app/ui";
import type { Route } from "./+types/layout";
import {
  ApiError,
  compareParams,
  fetchConfig,
  fetchDiff,
  fetchSnapshots,
} from "../api.server";
import { CompareContext, type CompareState } from "../lib/compare";
import { CompareBar } from "../components/compare-bar";
import styles from "./layout.module.css";

interface NavItem {
  value: string;
  label: string;
}

interface NavGroup {
  group: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    group: "Overview",
    items: [{ value: "/", label: "Overview" }],
  },
  {
    group: "Infrastructure",
    items: [
      { value: "/topology", label: "Topology" },
      { value: "/nodes", label: "Nodes" },
      { value: "/storage", label: "Storage" },
    ],
  },
  {
    group: "Networking",
    items: [
      { value: "/network", label: "Network" },
      { value: "/network-policies", label: "Network Policies" },
    ],
  },
  {
    group: "GitOps",
    items: [
      { value: "/dependencies", label: "Dependencies" },
      { value: "/circle-map", label: "Circle Map" },
      { value: "/charts", label: "Helm Charts" },
    ],
  },
  {
    group: "Workloads",
    items: [
      { value: "/workloads", label: "Workloads" },
      { value: "/images", label: "Images" },
      { value: "/configs", label: "ConfigMaps/Secrets" },
    ],
  },
  {
    group: "Security & Access",
    items: [
      { value: "/security", label: "Security" },
      { value: "/rbac", label: "RBAC" },
      { value: "/certificates", label: "Certificates" },
    ],
  },
  {
    group: "Cluster Inventory",
    items: [
      { value: "/crds", label: "CRDs" },
      { value: "/labels", label: "Labels/Annotations" },
      { value: "/quotas", label: "Resource Quotas" },
      { value: "/velero", label: "Backup Schedules" },
    ],
  },
  {
    group: "Cross-References",
    items: [
      { value: "/helm-workloads", label: "Helm to Workloads" },
      { value: "/service-map", label: "Service Mapping" },
      { value: "/namespace-summary", label: "Namespace Summary" },
    ],
  },
];

/** Compare state for every page. Snapshots are a DB-backed feature, so
 *  with no database this is a cheap config probe and nothing else. */
export async function loader({ request }: Route.LoaderArgs): Promise<{ compare: CompareState }> {
  const config = await fetchConfig();
  const { before, after } = compareParams(request);
  const state: CompareState = {
    enabled: config.snapshots,
    active: before !== null,
    before,
    after,
    snapshots: [],
    diff: null,
    error: null,
  };
  if (!config.snapshots) return { compare: state };

  try {
    state.snapshots = await fetchSnapshots(50);
  } catch (e) {
    state.error = e instanceof Error ? e.message : String(e);
  }
  if (state.active) {
    try {
      state.diff = await fetchDiff(before ?? "prev", after);
    } catch (e) {
      state.error =
        e instanceof ApiError && e.status === 404
          ? (e.detail ?? e.message)
          : `Could not load changes: ${e instanceof Error ? e.message : String(e)}`;
    }
  }
  return { compare: state };
}

function findActiveTab(pathname: string, items: NavItem[]): string {
  return (
    items.find(
      (item) =>
        item.value === pathname ||
        (item.value !== "/" && pathname.startsWith(item.value))
    )?.value ?? "/"
  );
}

export default function AppLayout({ loaderData }: Route.ComponentProps) {
  const location = useLocation();
  const navigate = useNavigate();

  const allItems = navGroups.flatMap((g) => g.items);
  const activeTab = findActiveTab(location.pathname, allItems);

  // Keep compare selectors when moving between views so the user can
  // walk the changed diagrams without re-picking the window.
  const go = (v: string) => navigate(`${v}${location.search}`);

  return (
    <CompareContext value={loaderData.compare}>
      <div className={styles.layout}>
        <div className={styles.sidebar}>
          <div className={styles.header}>
            <Heading level={2} variant="headingSm">
              Cluster Vision
            </Heading>
          </div>
          <div className={styles.navScroll}>
            <SideNav.Root value={activeTab} onValueChange={go}>
              {navGroups.map((group) => (
                <SideNav.Group key={group.group} label={group.group}>
                  {group.items.map((item) => (
                    <SideNav.Item key={item.value} value={item.value}>
                      {item.label}
                    </SideNav.Item>
                  ))}
                </SideNav.Group>
              ))}
            </SideNav.Root>
          </div>
          <div className={styles.licence}>
            <Stack gap="xs">
              <Text variant="caption" color="muted">
                Free for personal, non-profit and educational use. Running it in
                or for a business requires a commercial licence.
              </Text>
              <a
                className={styles.licenceLink}
                href="mailto:licensing@daddyshome.fr?subject=Cluster%20Vision%20commercial%20licence"
              >
                <Text variant="caption" color="accent" weight="medium">
                  Get a licence
                </Text>
              </a>
            </Stack>
          </div>
        </div>
        <main className={styles.content}>
          <Stack gap="md">
            <CompareBar />
            <Outlet />
          </Stack>
        </main>
      </div>
    </CompareContext>
  );
}
