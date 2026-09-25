import { useEffect } from "react";
import { useLocation, useNavigate, useRevalidator } from "react-router";
import { Button, Callout, Inline, Stack, Text } from "@duro-app/ui";
import { describeRouteError } from "../lib/route-error";

/** While the API answers 503 (first cluster sync still running), re-run the
 *  loaders on this cadence so the page fills in without a manual reload. */
const LOADING_RETRY_MS = 15_000;

/** Query string without the compare selectors — the live view of this page. */
export function liveSearch(search: string): string {
  const params = new URLSearchParams(search);
  params.delete("before");
  params.delete("after");
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

/** Error UI for a view. Rendered inside the app layout, so the sidebar and
 *  compare bar stay usable when one page's data cannot be loaded. */
export function RouteErrorBoundary({ error }: { error: unknown }) {
  const view = describeRouteError(error);
  const location = useLocation();
  const navigate = useNavigate();
  const revalidator = useRevalidator();
  const inSnapshot = new URLSearchParams(location.search).has("after");

  useEffect(() => {
    if (view.status !== 503) return;
    const t = setInterval(() => {
      if (revalidator.state === "idle") void revalidator.revalidate();
    }, LOADING_RETRY_MS);
    return () => clearInterval(t);
  }, [view.status, revalidator]);

  return (
    <Callout variant={view.variant} align="start">
      <Stack gap="sm">
        <Text weight="semibold">{view.title}</Text>
        <Text>{view.detail}</Text>
        <Inline gap="sm" align="center">
          <Button
            variant="secondary"
            size="small"
            onClick={() => void revalidator.revalidate()}
          >
            {revalidator.state === "loading" ? "Retrying…" : "Retry"}
          </Button>
          {view.status === 404 && inSnapshot && (
            <Button
              variant="link"
              size="small"
              onClick={() => navigate(`${location.pathname}${liveSearch(location.search)}`)}
            >
              Back to the live view
            </Button>
          )}
        </Inline>
      </Stack>
    </Callout>
  );
}
