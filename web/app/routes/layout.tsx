import { Outlet, useLocation, useNavigate } from "react-router";
import { SideNav } from "@fredericrous/duro-design-system";
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

const allItems = navGroups.flatMap((g) => g.items);

function findActiveTab(pathname: string): string {
  return (
    allItems.find(
      (item) =>
        item.value === pathname ||
        (item.value !== "/" && pathname.startsWith(item.value))
    )?.value ?? "/"
  );
}

export default function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const activeTab = findActiveTab(location.pathname);

  return (
    <div className={styles.layout}>
      <div className={styles.sidebar}>
        <div className={styles.header}>
          <h2 className={styles.title}>Cluster Vision</h2>
        </div>
        <div className={styles.navScroll}>
          <SideNav.Root value={activeTab} onValueChange={(v) => navigate(v)}>
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
      </div>
      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  );
}
