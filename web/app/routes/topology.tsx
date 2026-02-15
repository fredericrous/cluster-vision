import type { Route } from "./+types/topology";
import { fetchDiagramsByPrefix } from "../api.server";
import { DiagramPage } from "../components/diagram-page";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Topology — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagramsByPrefix("topology");
}

export default function Topology({ loaderData }: Route.ComponentProps) {
  const { diagrams, generatedAt } = loaderData;

  if (diagrams.length === 0) {
    return <p>No topology data available.</p>;
  }

  return (
    <>
      {diagrams.map((diagram) => (
        <DiagramPage
          key={diagram.id}
          diagram={diagram}
          generatedAt={generatedAt}
        />
      ))}
    </>
  );
}
