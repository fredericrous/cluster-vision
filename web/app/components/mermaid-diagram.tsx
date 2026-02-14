import { useEffect, useRef, useState } from "react";
import styles from "./mermaid-diagram.module.css";

interface MermaidDiagramProps {
  content: string;
  id: string;
}

export function MermaidDiagram({ content, id }: MermaidDiagramProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function render() {
      if (!containerRef.current) return;

      try {
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
            mainBkg: "#1e293b",
            nodeBorder: "#4f46e5",
            clusterBkg: "#0f172a",
            clusterBorder: "#334155",
            titleColor: "#e2e8f0",
            edgeLabelBackground: "#1e293b",
          },
          flowchart: {
            htmlLabels: true,
            curve: "basis",
          },
          securityLevel: "loose",
        });

        const uniqueId = `mermaid-${id}-${Date.now()}`;
        const { svg } = await mermaid.render(uniqueId, content);

        if (!cancelled && containerRef.current) {
          containerRef.current.innerHTML = svg;
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Failed to render diagram"
          );
        }
      }
    }

    render();
    return () => {
      cancelled = true;
    };
  }, [content, id]);

  if (error) {
    return (
      <div className={styles.error}>
        <p>Failed to render diagram</p>
        <pre>{error}</pre>
        <details>
          <summary>Raw Mermaid source</summary>
          <pre>{content}</pre>
        </details>
      </div>
    );
  }

  return <div ref={containerRef} className={styles.container} />;
}
