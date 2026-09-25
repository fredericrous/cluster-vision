import { afterEach, describe, it, expect, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";

const initialize = vi.fn();
const renderMermaid = vi.fn();

vi.mock("mermaid", () => ({
  default: { initialize, render: renderMermaid },
}));

const { MermaidDiagram } = await import("../app/components/mermaid-diagram");

afterEach(() => {
  cleanup();
  initialize.mockReset();
  renderMermaid.mockReset();
});

describe("MermaidDiagram", () => {
  it("clears the error when new content renders", async () => {
    renderMermaid.mockRejectedValueOnce(new Error("Parse error on line 1"));
    const { rerender, container } = render(<MermaidDiagram id="t" content="graph TD; A--" />);
    expect(await screen.findByText("Failed to render diagram")).toBeInTheDocument();

    renderMermaid.mockResolvedValueOnce({ svg: '<svg data-testid="ok"></svg>' });
    rerender(<MermaidDiagram id="t" content="graph TD; A-->B" />);
    await waitFor(() =>
      expect(screen.queryByText("Failed to render diagram")).not.toBeInTheDocument()
    );
    expect(container.querySelector("svg[data-testid=ok]")).not.toBeNull();
  });

  it("runs mermaid in strict security mode", async () => {
    renderMermaid.mockResolvedValueOnce({ svg: "<svg></svg>" });
    render(<MermaidDiagram id="t" content="graph TD; A-->B" />);
    await waitFor(() => expect(initialize).toHaveBeenCalled());
    expect(initialize.mock.calls[0][0].securityLevel).toBe("strict");
  });
});

describe("MarkdownTable pie chart", () => {
  it("shows an error instead of an empty box when the chart fails", async () => {
    const { MarkdownTable } = await import("../app/components/markdown-table");
    renderMermaid.mockRejectedValueOnce(new Error("bad pie"));
    render(
      <MarkdownTable content={"| a | b |\n|---|---|\n| 1 | 2 |\n\n```mermaid\npie\n  \"x\" : 1\n```"} />
    );
    expect(await screen.findByText(/Failed to render chart: bad pie/)).toBeInTheDocument();
  });
});
