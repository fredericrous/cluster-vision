import { Outlet, useLocation, useNavigate } from "react-router";
import { Tabs } from "@base-ui/react/tabs";
import styles from "./layout.module.css";

const navItems = [
  { value: "/", label: "Overview" },
  { value: "/topology", label: "Topology" },
  { value: "/dependencies", label: "Dependencies" },
  { value: "/network", label: "Network" },
  { value: "/security", label: "Security" },
  { value: "/nodes", label: "Nodes" },
  { value: "/charts", label: "Helm Charts" },
  { value: "/images", label: "Images" },
];

export default function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();

  // Match current path to tab value (exact match for "/", prefix for others)
  const activeTab =
    navItems.find(
      (item) =>
        item.value === location.pathname ||
        (item.value !== "/" && location.pathname.startsWith(item.value))
    )?.value ?? "/";

  return (
    <div className={styles.layout}>
      <nav className={styles.sidebar}>
        <div className={styles.header}>
          <h2 className={styles.title}>Cluster Vision</h2>
        </div>
        <Tabs.Root
          value={activeTab}
          onValueChange={(value) => navigate(value as string)}
        >
          <Tabs.List className={styles.tabList}>
            {navItems.map((item) => (
              <Tabs.Tab
                key={item.value}
                value={item.value}
                className={styles.tab}
              >
                {item.label}
              </Tabs.Tab>
            ))}
            <Tabs.Indicator className={styles.indicator} />
          </Tabs.List>
        </Tabs.Root>
      </nav>
      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  );
}
