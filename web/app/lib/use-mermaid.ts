import { useEffect, useRef, useState } from "react";
import type { MermaidConfig } from "mermaid";

/** Labels are built from cluster-derived strings (names, hostnames, IPs), so
 *  mermaid runs in "strict": no click callbacks, and label HTML goes through
 *  DOMPurify. The backend emits no `click` directives, and the `<br/>` /
 *  `<small>` it does put in labels survive the sanitizer. */
export const MERMAID_SECURITY_LEVEL = "strict" as const;

let renderSeq = 0;

/** Render `content` as a mermaid SVG into the returned ref'd host. The host
 *  must stay mounted whatever `error` is: a new `content` re-renders into it
 *  and clears the error, and a failure empties it instead of unmounting it. */
export function useMermaid(content: string, id: string, config: MermaidConfig) {
  const ref = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function render() {
      try {
        const mermaid = (await import("mermaid")).default;
        mermaid.initialize({
          ...config,
          startOnLoad: false,
          securityLevel: MERMAID_SECURITY_LEVEL,
        });
        const { svg } = await mermaid.render(`mermaid-${id}-${++renderSeq}`, content);
        if (cancelled) return;
        if (ref.current) ref.current.innerHTML = svg;
        setError(null);
      } catch (err) {
        if (cancelled) return;
        if (ref.current) ref.current.innerHTML = "";
        setError(err instanceof Error ? err.message : "Failed to render diagram");
      }
    }

    void render();
    return () => {
      cancelled = true;
    };
  }, [content, id, config]);

  return { ref, error };
}
