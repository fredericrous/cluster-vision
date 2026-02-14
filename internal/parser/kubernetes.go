package parser

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesParser queries the Kubernetes API for cluster state.
type KubernetesParser struct {
	typed   kubernetes.Interface
	dynamic dynamic.Interface
}

// NewKubernetesParser creates a parser from a kubeconfig path.
// Pass "" for in-cluster config.
func NewKubernetesParser(kubeconfig string) (*KubernetesParser, error) {
	var cfg *rest.Config
	var err error

	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("building k8s config: %w", err)
	}

	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating typed client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &KubernetesParser{typed: typed, dynamic: dyn}, nil
}

// ParseAll queries all supported resources and returns cluster data.
func (p *KubernetesParser) ParseAll(ctx context.Context) *model.ClusterData {
	data := &model.ClusterData{}
	data.Nodes = p.parseNodes(ctx)
	data.Flux = p.parseFluxKustomizations(ctx)
	data.Gateways = p.parseGateways(ctx)
	data.HTTPRoutes = p.parseHTTPRoutes(ctx)
	data.Namespaces = p.parseNamespaces(ctx)
	data.SecurityPolicies = p.parseSecurityPolicies(ctx)
	return data
}

func (p *KubernetesParser) parseNodes(ctx context.Context) []model.NodeInfo {
	list, err := p.typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list nodes", "error", err)
		return nil
	}

	var nodes []model.NodeInfo
	for _, n := range list.Items {
		ip := ""
		for _, addr := range n.Status.Addresses {
			if addr.Type == "InternalIP" {
				ip = addr.Address
				break
			}
		}

		var roles []string
		for label := range n.Labels {
			if strings.HasPrefix(label, "node-role.kubernetes.io/") {
				roles = append(roles, strings.TrimPrefix(label, "node-role.kubernetes.io/"))
			}
		}

		cpu := n.Status.Capacity.Cpu().String()
		mem := n.Status.Capacity.Memory().String()

		nodes = append(nodes, model.NodeInfo{
			Name:   n.Name,
			IP:     ip,
			Roles:  roles,
			CPU:    cpu,
			Memory: mem,
			Labels: n.Labels,
		})
	}
	return nodes
}

func (p *KubernetesParser) parseFluxKustomizations(ctx context.Context) []model.FluxKustomization {
	gvr := schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}

	list, err := p.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list flux kustomizations (CRD may not exist)", "error", err)
		return nil
	}

	var result []model.FluxKustomization
	for _, item := range list.Items {
		name := item.GetName()
		ns := item.GetNamespace()

		spec, _ := item.Object["spec"].(map[string]interface{})
		path, _ := spec["path"].(string)

		var deps []string
		if dependsOn, ok := spec["dependsOn"].([]interface{}); ok {
			for _, d := range dependsOn {
				if dm, ok := d.(map[string]interface{}); ok {
					if dn, ok := dm["name"].(string); ok {
						deps = append(deps, dn)
					}
				}
			}
		}

		result = append(result, model.FluxKustomization{
			Name:      name,
			Namespace: ns,
			Path:      path,
			DependsOn: deps,
		})
	}
	return result
}

func (p *KubernetesParser) parseGateways(ctx context.Context) []model.GatewayInfo {
	gvr := schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}

	list, err := p.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list gateways (CRD may not exist)", "error", err)
		return nil
	}

	var result []model.GatewayInfo
	for _, item := range list.Items {
		gw := model.GatewayInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
		}

		spec, _ := item.Object["spec"].(map[string]interface{})
		listeners, _ := spec["listeners"].([]interface{})
		for _, l := range listeners {
			lm, ok := l.(map[string]interface{})
			if !ok {
				continue
			}
			li := model.ListenerInfo{
				Name:     strVal(lm, "name"),
				Hostname: strVal(lm, "hostname"),
				Protocol: strVal(lm, "protocol"),
			}
			if port, ok := lm["port"].(int64); ok {
				li.Port = int(port)
			} else if port, ok := lm["port"].(float64); ok {
				li.Port = int(port)
			}
			gw.Listeners = append(gw.Listeners, li)
		}

		result = append(result, gw)
	}
	return result
}

func (p *KubernetesParser) parseHTTPRoutes(ctx context.Context) []model.HTTPRouteInfo {
	gvr := schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "httproutes",
	}

	list, err := p.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list httproutes (CRD may not exist)", "error", err)
		return nil
	}

	var result []model.HTTPRouteInfo
	for _, item := range list.Items {
		route := model.HTTPRouteInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
		}

		spec, _ := item.Object["spec"].(map[string]interface{})

		// Hostnames
		if hostnames, ok := spec["hostnames"].([]interface{}); ok {
			for _, h := range hostnames {
				if s, ok := h.(string); ok {
					route.Hostnames = append(route.Hostnames, s)
				}
			}
		}

		// SectionName from first parentRef
		if parentRefs, ok := spec["parentRefs"].([]interface{}); ok && len(parentRefs) > 0 {
			if pr, ok := parentRefs[0].(map[string]interface{}); ok {
				route.SectionName = strVal(pr, "sectionName")
			}
		}

		// Backend refs from rules
		if rules, ok := spec["rules"].([]interface{}); ok {
			for _, r := range rules {
				rm, ok := r.(map[string]interface{})
				if !ok {
					continue
				}
				if backends, ok := rm["backendRefs"].([]interface{}); ok {
					for _, b := range backends {
						bm, ok := b.(map[string]interface{})
						if !ok {
							continue
						}
						ref := model.BackendRef{Name: strVal(bm, "name")}
						if port, ok := bm["port"].(int64); ok {
							ref.Port = int(port)
						} else if port, ok := bm["port"].(float64); ok {
							ref.Port = int(port)
						}
						route.Backends = append(route.Backends, ref)
					}
				}
			}
		}

		result = append(result, route)
	}
	return result
}

func (p *KubernetesParser) parseNamespaces(ctx context.Context) []model.NamespaceInfo {
	list, err := p.typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list namespaces", "error", err)
		return nil
	}

	// Filter to namespaces that look like app namespaces (not system ones)
	systemPrefixes := []string{"kube-", "flux-", "cert-manager", "envoy-gateway", "istio-", "cnpg-", "rook-", "ot-operators"}
	systemExact := map[string]bool{
		"default": true, "kube-system": true, "kube-public": true,
		"kube-node-lease": true, "flux-system": true, "local-path-storage": true,
	}

	var result []model.NamespaceInfo
	for _, ns := range list.Items {
		name := ns.Name
		if systemExact[name] {
			continue
		}
		skip := false
		for _, prefix := range systemPrefixes {
			if strings.HasPrefix(name, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		labels := ns.Labels
		if labels == nil {
			labels = map[string]string{}
		}

		result = append(result, model.NamespaceInfo{
			Name:        name,
			Ambient:     labels["istio.io/dataplane-mode"] == "ambient",
			Waypoint:    labels["istio.io/use-waypoint"] != "",
			Backup:      labels["backup"] == "velero",
			MTLS:        labels["mtls.enabled"] == "true",
			PodSecurity: labels["pod-security.kubernetes.io/enforce"],
		})
	}
	return result
}

func (p *KubernetesParser) parseSecurityPolicies(ctx context.Context) []model.SecurityPolicyInfo {
	// Try Envoy Gateway SecurityPolicy
	gvr := schema.GroupVersionResource{
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "securitypolicies",
	}

	list, err := p.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Debug("no envoy gateway security policies found", "error", err)
		return nil
	}

	var result []model.SecurityPolicyInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})
		if _, hasExtAuth := spec["extAuth"]; hasExtAuth {
			result = append(result, model.SecurityPolicyInfo{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			})
		}
	}
	return result
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
