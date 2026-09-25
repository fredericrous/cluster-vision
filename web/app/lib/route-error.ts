import { isRouteErrorResponse } from "react-router";

/** What a route's error boundary shows. */
export interface RouteErrorView {
  variant: "info" | "warning" | "error";
  title: string;
  detail: string;
  status: number | null;
}

/** Body of a thrown route error response (see `toRouteError` in api.server). */
export interface RouteErrorBody {
  error?: string;
  /** What was missing on a 404: a stored snapshot, or one diagram of it. */
  kind?: "snapshot" | "diagram";
}

function detailOf(data: unknown): string | undefined {
  if (typeof data === "string" && data.trim() !== "") return data;
  if (data && typeof data === "object" && "error" in data) {
    const e = (data as RouteErrorBody).error;
    if (typeof e === "string" && e.trim() !== "") return e;
  }
  return undefined;
}

/** Map a route error to a message a user can act on. The API answers 503
 *  until the first cluster sync lands, and 404 for an unknown snapshot or
 *  diagram; everything else is a generic failure. */
export function describeRouteError(error: unknown): RouteErrorView {
  if (isRouteErrorResponse(error)) {
    const detail = detailOf(error.data);
    if (error.status === 503) {
      return {
        variant: "info",
        title: "Data is still loading",
        detail:
          "The first cluster sync hasn't finished yet. This page retries on its own and fills in once it has.",
        status: 503,
      };
    }
    if (error.status === 404) {
      const kind =
        error.data && typeof error.data === "object"
          ? (error.data as RouteErrorBody).kind
          : undefined;
      return {
        variant: "warning",
        title: kind === "diagram" ? "Diagram not found" : "Snapshot not found",
        detail:
          detail ??
          "The snapshot or diagram this link points at does not exist, or it has expired.",
        status: 404,
      };
    }
    return {
      variant: "error",
      title: "Could not load this view",
      detail: detail ?? (error.statusText || `The API answered ${error.status}.`),
      status: error.status,
    };
  }
  return {
    variant: "error",
    title: "Could not load this view",
    detail: "An unexpected error occurred.",
    status: null,
  };
}
