import { useEffect, useRef, useState } from "react";
import { Callout, Stack, Text } from "@duro-app/ui";
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
      <Callout variant="error" align="start">
        <Stack gap="sm">
          <Text>Failed to render diagram</Text>
          <pre>{error}</pre>
          {/* <details>/<summary> is a native disclosure with no Duro
              equivalent — allow-listed for this file in eslint.config.js. */}
          <details>
            <summary>Raw Mermaid source</summary>
            <pre>{content}</pre>
          </details>
        </Stack>
      </Callout>
    );
  }

  // Host element for the mermaid-rendered SVG, injected via innerHTML — it
  // needs a real ref'd DOM node and a :global(svg) CSS rule, so it stays a raw
  // div (allow-listed for this file in eslint.config.js).
  return <div ref={containerRef} className={styles.container} />;
}
