import type { Route } from "./+types/network";
import { fetchDiagram } from "../api.server";
import { DiagramPage } from "../components/diagram-page";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Network — Cluster Vision" }];
}

export async function loader() {
  return fetchDiagram("network");
}

export default function Network({ loaderData }: Route.ComponentProps) {
  return (
    <DiagramPage
      diagram={loaderData.diagram}
      generatedAt={loaderData.generatedAt}
    />
  );
}
