import {
  type RouteConfig,
  index,
  route,
  layout,
} from "@react-router/dev/routes";

export default [
  layout("routes/layout.tsx", [
    index("routes/home.tsx"),
    route("topology", "routes/topology.tsx"),
    route("dependencies", "routes/dependencies.tsx"),
    route("network", "routes/network.tsx"),
    route("security", "routes/security.tsx"),
    route("nodes", "routes/nodes.tsx"),
    route("charts", "routes/charts.tsx"),
    route("images", "routes/images.tsx"),
  ]),
] satisfies RouteConfig;
