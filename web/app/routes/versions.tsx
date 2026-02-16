import type { Route } from "./+types/versions";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Versions — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("versions");
}

export default function Versions({ loaderData }: Route.ComponentProps) {
  return (
    <DiagramPage
      diagram={loaderData.diagram}
      generatedAt={loaderData.generatedAt}
    />
  );
}
