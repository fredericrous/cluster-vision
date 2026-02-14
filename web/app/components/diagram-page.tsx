import { Suspense, lazy } from "react";
import { ScrollArea } from "@base-ui/react/scroll-area";
import type { DiagramResult } from "../api.server";
import styles from "./diagram-page.module.css";

const MermaidDiagram = lazy(() =>
  import("./mermaid-diagram").then((m) => ({ default: m.MermaidDiagram }))
);
const MarkdownTable = lazy(() =>
  import("./markdown-table").then((m) => ({ default: m.MarkdownTable }))
);

interface DiagramPageProps {
  diagram: DiagramResult;
  generatedAt: string;
}

export function DiagramPage({ diagram, generatedAt }: DiagramPageProps) {
  const formattedTime = new Date(generatedAt).toLocaleString();

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.heading}>{diagram.title}</h1>
        <span className={styles.generatedAt}>Updated: {formattedTime}</span>
      </div>

      <div className={styles.content}>
        <ScrollArea.Root>
          <ScrollArea.Viewport className={styles.viewport}>
            <Suspense
              fallback={<div className={styles.loading}>Rendering...</div>}
            >
              {diagram.type === "mermaid" ? (
                <MermaidDiagram content={diagram.content} id={diagram.id} />
              ) : (
                <MarkdownTable content={diagram.content} />
              )}
            </Suspense>
          </ScrollArea.Viewport>
          <ScrollArea.Scrollbar
            orientation="horizontal"
            className={styles.scrollbar}
          >
            <ScrollArea.Thumb className={styles.thumb} />
          </ScrollArea.Scrollbar>
          <ScrollArea.Scrollbar
            orientation="vertical"
            className={styles.scrollbar}
          >
            <ScrollArea.Thumb className={styles.thumb} />
          </ScrollArea.Scrollbar>
        </ScrollArea.Root>
      </div>
    </div>
  );
}
