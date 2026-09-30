import { Outlet } from "react-router";
import type { Route } from "./+types/view-boundary";
import { RouteErrorBoundary } from "../components/route-error-boundary";

/** Pathless route between the app layout and every view. It exists only to
 *  own the views' ErrorBoundary: an error thrown by a view's loader renders
 *  here, inside the layout, instead of replacing the whole app (sidebar and
 *  compare bar included) with root's generic error page. */
export default function ViewBoundary() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  return <RouteErrorBoundary error={error} />;
}
