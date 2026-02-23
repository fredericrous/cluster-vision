import { useState, useEffect } from "react";
import { Outlet, useLocation, useNavigate } from "react-router";
import { Tabs } from "@base-ui/react/tabs";
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

// Flatten all items for tab value matching
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

function findActiveGroup(pathname: string): string {
  const activeValue = findActiveTab(pathname);
  return (
    navGroups.find((g) => g.items.some((item) => item.value === activeValue))
      ?.group ?? "Overview"
  );
}

export default function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const activeTab = findActiveTab(location.pathname);
  const activeGroup = findActiveGroup(location.pathname);

  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(
    () => new Set([activeGroup])
  );

  // Auto-expand the group containing the active route
  useEffect(() => {
    setExpandedGroups((prev) => {
      if (prev.has(activeGroup)) return prev;
      const next = new Set(prev);
      next.add(activeGroup);
      return next;
    });
  }, [activeGroup]);

  const toggleGroup = (group: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(group)) {
        next.delete(group);
      } else {
        next.add(group);
      }
      return next;
    });
  };

  return (
    <div className={styles.layout}>
      <nav className={styles.sidebar}>
        <div className={styles.header}>
          <h2 className={styles.title}>Cluster Vision</h2>
        </div>
        <div className={styles.navScroll}>
          <Tabs.Root
            value={activeTab}
            onValueChange={(value) => navigate(value as string)}
          >
            <Tabs.List className={styles.tabList}>
              {navGroups.map((group) => {
                const isExpanded = expandedGroups.has(group.group);
                const hasActive = group.items.some(
                  (item) => item.value === activeTab
                );

                return (
                  <div key={group.group} className={styles.group}>
                    <button
                      className={`${styles.groupHeader} ${hasActive ? styles.groupHeaderActive : ""}`}
                      onClick={() => toggleGroup(group.group)}
                      type="button"
                    >
                      <span
                        className={`${styles.chevron} ${isExpanded ? styles.chevronOpen : ""}`}
                      >
                        &#9656;
                      </span>
                      {group.group}
                    </button>
                    {isExpanded &&
                      group.items.map((item) => (
                        <Tabs.Tab
                          key={item.value}
                          value={item.value}
                          className={styles.tab}
                        >
                          {item.label}
                        </Tabs.Tab>
                      ))}
                  </div>
                );
              })}
              <Tabs.Indicator className={styles.indicator} />
            </Tabs.List>
          </Tabs.Root>
        </div>
      </nav>
      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  );
}
