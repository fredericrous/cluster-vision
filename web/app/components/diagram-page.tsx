import { Suspense, lazy, type ReactNode } from "react";
import { Heading, Inline, Stack, Text, ScrollArea } from "@duro-app/ui";
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

function DiagramLoading() {
  return (
    <Stack gap="md" align="center">
      <Text color="muted">Rendering...</Text>
    </Stack>
  );
}

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
      <Inline gap="md" align="baseline">
        <Heading level={1} variant="headingMd">
          {diagram.title}
        </Heading>
        <Text variant="caption" color="muted">
          Updated: {formattedTime}
        </Text>
      </Inline>

      {isFlow ? (
        <div className={styles.flowContent}>
          <Suspense fallback={<DiagramLoading />}>
            <FlowDiagram content={diagram.content} />
          </Suspense>
        </div>
      ) : (
        <div className={styles.content}>
          <ScrollArea.Root>
            <ScrollArea.Viewport>
              <ScrollArea.Content>
                <Suspense fallback={<DiagramLoading />}>
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
              </ScrollArea.Content>
            </ScrollArea.Viewport>
            <ScrollArea.Scrollbar orientation="horizontal">
              <ScrollArea.Thumb orientation="horizontal" />
            </ScrollArea.Scrollbar>
            <ScrollArea.Scrollbar orientation="vertical">
              <ScrollArea.Thumb orientation="vertical" />
            </ScrollArea.Scrollbar>
          </ScrollArea.Root>
        </div>
      )}
    </div>
  );
}
