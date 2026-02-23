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
    route("workloads", "routes/workloads.tsx"),
    route("storage", "routes/storage.tsx"),
    route("crds", "routes/crds.tsx"),
    route("quotas", "routes/quotas.tsx"),
    route("certificates", "routes/certificates.tsx"),
    route("network-policies", "routes/network-policies.tsx"),
    route("configs", "routes/configs.tsx"),
    route("helm-workloads", "routes/helm-workloads.tsx"),
    route("service-map", "routes/service-map.tsx"),
    route("namespace-summary", "routes/namespace-summary.tsx"),
    route("rbac", "routes/rbac.tsx"),
    route("labels", "routes/labels.tsx"),
    route("velero", "routes/velero.tsx"),
  ]),
] satisfies RouteConfig;
