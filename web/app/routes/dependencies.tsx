import type { Route } from "./+types/dependencies";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Dependencies — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("dependencies");
}

export default function Dependencies({ loaderData }: Route.ComponentProps) {
  return (
    <DiagramPage
      diagram={loaderData.diagram}
      generatedAt={loaderData.generatedAt}
    />
  );
}
