import { Outlet, useLocation, useNavigate } from "react-router";
import { Icon, SideNav, type IconName } from "@duro-app/ui";
import styles from "./layout.module.css";

interface NavItem {
  value: string;
  label: string;
  icon: IconName;
}

interface NavSection {
  section: string;
  items: NavItem[];
}

// Every section is always open. The rail's job is to advertise where you can
// go — behind chevrons, all 21 destinations cost an extra click and the shape
// of the cluster was invisible to anyone scanning. The section labels do the
// chunking on their own. See duro-design-system's SideNav guidance for when a
// collapsible SideNav.Group is the right call instead.
const navSections: NavSection[] = [
  {
    section: "Overview",
    items: [{ value: "/", label: "Overview", icon: "map" }],
  },
  {
    section: "Infrastructure",
    items: [
      { value: "/topology", label: "Topology", icon: "git-branch" },
      { value: "/nodes", label: "Nodes", icon: "server" },
      { value: "/storage", label: "Storage", icon: "hard-drive" },
    ],
  },
  {
    section: "Networking",
    items: [
      { value: "/network", label: "Network", icon: "route" },
      {
        value: "/network-policies",
        label: "Network Policies",
        icon: "shield-check",
      },
    ],
  },
  {
    section: "GitOps",
    items: [
      { value: "/dependencies", label: "Dependencies", icon: "repeat" },
      { value: "/circle-map", label: "Circle Map", icon: "contrast" },
      { value: "/charts", label: "Helm Charts", icon: "layers" },
    ],
  },
  {
    section: "Workloads",
    items: [
      { value: "/workloads", label: "Workloads", icon: "box" },
      { value: "/images", label: "Images", icon: "image" },
      { value: "/configs", label: "ConfigMaps/Secrets", icon: "key" },
    ],
  },
  {
    section: "Security & Access",
    items: [
      { value: "/security", label: "Security", icon: "shield" },
      { value: "/rbac", label: "RBAC", icon: "users" },
      { value: "/certificates", label: "Certificates", icon: "lock" },
    ],
  },
  {
    section: "Cluster Inventory",
    items: [
      { value: "/crds", label: "CRDs", icon: "file-text" },
      { value: "/labels", label: "Labels/Annotations", icon: "tag" },
      { value: "/quotas", label: "Resource Quotas", icon: "pie-chart" },
      { value: "/velero", label: "Backup Schedules", icon: "clock" },
    ],
  },
  {
    section: "Cross-References",
    items: [
      { value: "/helm-workloads", label: "Helm to Workloads", icon: "plug" },
      { value: "/service-map", label: "Service Mapping", icon: "git-branch" },
      {
        value: "/namespace-summary",
        label: "Namespace Summary",
        icon: "monitor",
      },
    ],
  },
];

function findActiveTab(pathname: string, items: NavItem[]): string {
  return (
    items.find(
      (item) =>
        item.value === pathname ||
        (item.value !== "/" && pathname.startsWith(item.value))
    )?.value ?? "/"
  );
}

export default function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();

  const allItems = navSections.flatMap((s) => s.items);
  const activeTab = findActiveTab(location.pathname, allItems);

  return (
    <div className={styles.layout}>
      <div className={styles.sidebar}>
        <div className={styles.header}>
          <h2 className={styles.title}>Cluster Vision</h2>
        </div>
        <div className={styles.navScroll}>
          <SideNav.Root value={activeTab} onValueChange={(v) => navigate(v)}>
            {navSections.map((section) => (
              <SideNav.Section key={section.section} label={section.section}>
                {section.items.map((item) => (
                  <SideNav.Item
                    key={item.value}
                    value={item.value}
                    icon={<Icon name={item.icon} size={18} />}
                  >
                    {item.label}
                  </SideNav.Item>
                ))}
              </SideNav.Section>
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
