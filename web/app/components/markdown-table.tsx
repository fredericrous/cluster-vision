import { useEffect, useRef } from "react";
import styles from "./markdown-table.module.css";

interface MarkdownTableProps {
  content: string;
}

export function MarkdownTable({ content }: MarkdownTableProps) {
  const lines = content.trim().split("\n");

  // Find the table portion (lines starting with |)
  const tableLines = lines.filter((l) => l.trim().startsWith("|"));
  if (tableLines.length < 2) {
    return <pre className={styles.raw}>{content}</pre>;
  }

  const parseRow = (line: string) =>
    line
      .split("|")
      .slice(1, -1)
      .map((cell) => cell.trim());

  const headers = parseRow(tableLines[0]);
  // Skip separator row (index 1)
  const rows = tableLines.slice(2).map(parseRow);

  // Check for mermaid blocks after the table
  const mermaidStart = content.indexOf("```mermaid");
  const mermaidContent =
    mermaidStart >= 0
      ? content
          .slice(mermaidStart + "```mermaid\n".length)
          .replace(/```\s*$/, "")
          .trim()
      : null;

  return (
    <div className={styles.wrapper}>
      <table className={styles.table}>
        <thead>
          <tr>
            {headers.map((h, i) => (
              <th key={i}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, ri) => (
            <tr key={ri}>
              {row.map((cell, ci) => (
                <td key={ci}>
                  {cell === "yes" ? (
                    <span className={styles.badgeYes}>yes</span>
                  ) : cell === "no" ? (
                    <span className={styles.badgeNo}>no</span>
                  ) : (
                    cell
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {mermaidContent && (
        <MermaidInline content={mermaidContent} id="security-pie" />
      )}
    </div>
  );
}

function MermaidInline({ content, id }: { content: string; id: string }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    async function render() {
      if (!containerRef.current) return;
      const mermaid = (await import("mermaid")).default;
      mermaid.initialize({
        startOnLoad: false,
        theme: "dark",
        themeVariables: {
          darkMode: true,
          background: "#1a1a2e",
          primaryColor: "#6366f1",
          primaryTextColor: "#e2e8f0",
          primaryBorderColor: "#4f46e5",
          secondaryColor: "#1e293b",
          tertiaryColor: "#0f172a",
          lineColor: "#94a3b8",
          textColor: "#e2e8f0",
        },
      });
      const uniqueId = `mermaid-inline-${id}-${Date.now()}`;
      const { svg } = await mermaid.render(uniqueId, content);
      if (!cancelled && containerRef.current) {
        containerRef.current.innerHTML = svg;
      }
    }
    render();
    return () => {
      cancelled = true;
    };
  }, [content, id]);

  return <div ref={containerRef} className={styles.pieChart} />;
}
