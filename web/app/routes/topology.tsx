import type { Route } from "./+types/topology";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Topology — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("topology");
}

export default function Topology({ loaderData }: Route.ComponentProps) {
  return (
    <DiagramPage
      diagram={loaderData.diagram}
      generatedAt={loaderData.generatedAt}
    />
  );
}
