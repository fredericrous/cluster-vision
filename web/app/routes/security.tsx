import type { Route } from "./+types/security";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Security — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("security");
}

export default function Security({ loaderData }: Route.ComponentProps) {
  return (
    <DiagramPage
      diagram={loaderData.diagram}
      generatedAt={loaderData.generatedAt}
    />
  );
}
