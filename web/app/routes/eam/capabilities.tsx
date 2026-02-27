import { useState } from "react";
import type { Route } from "./+types/capabilities";
import { fetchCapabilityTree } from "../../api.server";
import type { BusinessCapability } from "../../api.server";
import styles from "./eam.module.css";

const API_URL = "/api"; // proxied through React Router

export function meta({}: Route.MetaArgs) {
  return [{ title: "Business Capabilities — Cluster Vision EAM" }];
}

export async function loader() {
  return fetchCapabilityTree();
}

async function clientFetchTree(): Promise<BusinessCapability[]> {
  const res = await fetch(`${API_URL}/eam/capabilities/tree`);
  if (!res.ok) throw new Error("Failed to fetch tree");
  return res.json();
}

async function clientCreateCapability(cap: { name: string; level: number; sort_order: number }) {
  const res = await fetch(`${API_URL}/eam/capabilities`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cap),
  });
  if (!res.ok) throw new Error("Failed to create capability");
  return res.json();
}

function TreeNode({
  node,
  depth,
}: {
  node: BusinessCapability;
  depth: number;
}) {
  const [expanded, setExpanded] = useState(true);
  const hasChildren = node.children && node.children.length > 0;

  return (
    <div>
      <div className={styles.treeNode} style={{ paddingLeft: `${depth * 1.5}rem` }}>
        <span className={styles.treeToggle} onClick={() => setExpanded(!expanded)}>
          {hasChildren ? (expanded ? "v" : ">") : " "}
        </span>
        <span style={{ fontWeight: depth === 0 ? 600 : 400 }}>{node.name}</span>
        <span className={styles.treeAppCount}>{node.app_count > 0 ? `(${node.app_count} apps)` : ""}</span>
      </div>
      {expanded && hasChildren && (
        <div>
          {node.children.map((child) => (
            <TreeNode key={child.id} node={child} depth={depth + 1} />
          ))}
        </div>
      )}
    </div>
  );
}

export default function Capabilities({ loaderData }: Route.ComponentProps) {
  const [tree, setTree] = useState(loaderData ?? []);
  const [newName, setNewName] = useState("");
  const [adding, setAdding] = useState(false);

  async function handleAdd() {
    if (!newName.trim()) return;
    setAdding(true);
    try {
      await clientCreateCapability({ name: newName.trim(), level: 1, sort_order: tree.length });
      const refreshed = await clientFetchTree();
      setTree(refreshed);
      setNewName("");
    } catch (err) {
      alert(`Failed: ${err}`);
    } finally {
      setAdding(false);
    }
  }

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>Business Capabilities</h1>
      <p className={styles.subtitle}>
        Hierarchical capability tree. Map applications to capabilities for the landscape view.
      </p>

      <div className={styles.toolbar}>
        <input
          className={styles.searchInput}
          placeholder="New capability name..."
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && handleAdd()}
        />
        <button className={styles.btnPrimary} onClick={handleAdd} disabled={adding}>
          {adding ? "Adding..." : "Add L1 Capability"}
        </button>
      </div>

      <div className={styles.treeContainer}>
        {tree.length > 0 ? (
          tree.map((node) => (
            <TreeNode key={node.id} node={node} depth={0} />
          ))
        ) : (
          <p style={{ color: "var(--text-muted)" }}>
            No capabilities yet. Add one above, or let AI generate them from the Sync page.
          </p>
        )}
      </div>
    </div>
  );
}
