import { Callout, Stack, Text } from "@duro-app/ui";
import type { MermaidConfig } from "mermaid";
import { useMermaid } from "../lib/use-mermaid";
import styles from "./mermaid-diagram.module.css";

interface MermaidDiagramProps {
  content: string;
  id: string;
}

const config: MermaidConfig = {
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
};

export function MermaidDiagram({ content, id }: MermaidDiagramProps) {
  const { ref, error } = useMermaid(content, id, config);

  // The host div stays mounted even while the error shows: it is where the
  // next `content` renders, and unmounting it left the error stuck forever.
  // It holds the mermaid SVG set via innerHTML, so it needs a real ref'd DOM
  // node and a :global(svg) CSS rule, and stays a raw div (allow-listed for
  // this file in eslint.config.js).
  return (
    <>
      {error && (
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
      )}
      <div ref={ref} className={styles.container} hidden={error !== null} />
    </>
  );
}
