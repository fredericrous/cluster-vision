import { Suspense, lazy, type ReactNode } from "react";
import { ScrollArea } from "@base-ui/react/scroll-area";
import type { DiagramResult } from "../api.server";
import styles from "./diagram-page.module.css";

const MermaidDiagram = lazy(() =>
  import("./mermaid-diagram").then((m) => ({ default: m.MermaidDiagram }))
);
const MarkdownTable = lazy(() =>
  import("./markdown-table").then((m) => ({ default: m.MarkdownTable }))
);
const FlowDiagram = lazy(() =>
  import("./flow-diagram").then((m) => ({ default: m.FlowDiagram }))
);

interface DiagramPageProps {
  diagram: DiagramResult;
  generatedAt: string;
  children?: ReactNode;
}

export function DiagramPage({
  diagram,
  generatedAt,
  children,
}: DiagramPageProps) {
  const formattedTime = new Date(generatedAt).toLocaleString();
  const isFlow = diagram.type === "flow";

  return (
    <div className={isFlow ? styles.flowPage : styles.page}>
      <div className={styles.header}>
        <h1 className={styles.heading}>{diagram.title}</h1>
        <span className={styles.generatedAt}>Updated: {formattedTime}</span>
      </div>

      {isFlow ? (
        <div className={styles.flowContent}>
          <Suspense
            fallback={<div className={styles.loading}>Rendering...</div>}
          >
            <FlowDiagram content={diagram.content} />
          </Suspense>
        </div>
      ) : (
        <div className={styles.content}>
          <ScrollArea.Root>
            <ScrollArea.Viewport className={styles.viewport}>
              <Suspense
                fallback={<div className={styles.loading}>Rendering...</div>}
              >
                {children
                  ? children
                  : diagram.type === "mermaid"
                    ? (
                      <MermaidDiagram content={diagram.content} id={diagram.id} />
                    )
                    : (
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
      )}
    </div>
  );
}
