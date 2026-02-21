package parser

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	typed       kubernetes.Interface
	dynamic     dynamic.Interface
	clusterName string
}

// NewKubernetesParser creates a parser from a kubeconfig path and cluster name.
// Pass "" for kubeconfig to use in-cluster config.
func NewKubernetesParser(kubeconfig, clusterName string) (*KubernetesParser, error) {
	var cfg *rest.Config
	var err error

	if kubeconfig != "" {
		data, readErr := os.ReadFile(kubeconfig)
		if readErr != nil {
			return nil, fmt.Errorf("reading kubeconfig %s: %w", kubeconfig, readErr)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("kubeconfig %s is empty", kubeconfig)
		}
		cfg, err = clientcmd.RESTConfigFromKubeConfig(data)
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

	return &KubernetesParser{typed: typed, dynamic: dyn, clusterName: clusterName}, nil
}

// ParseSecurity returns only namespace and security policy data for this cluster.
func (p *KubernetesParser) ParseSecurity(ctx context.Context) ([]model.NamespaceInfo, []model.SecurityPolicyInfo) {
	return p.parseNamespaces(ctx), p.parseSecurityPolicies(ctx)
}

// ParseHelm returns HelmRelease and HelmRepository data for this cluster.
func (p *KubernetesParser) ParseHelm(ctx context.Context) ([]model.HelmReleaseInfo, []model.HelmRepositoryInfo) {
	return p.parseHelmReleases(ctx), p.parseHelmRepositories(ctx)
}

// ParseFlux returns Flux Kustomization data for this cluster.
func (p *KubernetesParser) ParseFlux(ctx context.Context) []model.FluxKustomization {
	return p.parseFluxKustomizations(ctx)
}

// ParseNodes returns node data for this cluster.
func (p *KubernetesParser) ParseNodes(ctx context.Context) []model.NodeInfo {
	return p.parseNodes(ctx)
}

// ParseServiceEntries returns ServiceEntry data for this cluster.
func (p *KubernetesParser) ParseServiceEntries(ctx context.Context) []model.ServiceEntryInfo {
	return p.parseServiceEntries(ctx)
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
	data.ClientTrafficPolicies = p.parseClientTrafficPolicies(ctx)
	data.ServiceEntries = p.parseServiceEntries(ctx)
	data.EastWestGateways = p.parseEastWestGateways(ctx)
	data.LoadBalancers = p.parseLoadBalancers(ctx)
	data.HelmReleases = p.parseHelmReleases(ctx)
	data.HelmRepositories = p.parseHelmRepositories(ctx)
	data.Pods = p.parsePods(ctx)
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
		memBytes := n.Status.Capacity.Memory().Value()
		mem := fmt.Sprintf("%.1f Gi", float64(memBytes)/(1024*1024*1024))

		nodes = append(nodes, model.NodeInfo{
			Name:             n.Name,
			Cluster:          p.clusterName,
			IP:               ip,
			Roles:            roles,
			CPU:              cpu,
			Memory:           mem,
			Labels:           n.Labels,
			OSImage:          n.Status.NodeInfo.OSImage,
			KubeletVersion:   n.Status.NodeInfo.KubeletVersion,
			ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
			KernelVersion:    n.Status.NodeInfo.KernelVersion,
			Architecture:     n.Status.NodeInfo.Architecture,
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
			Cluster:   p.clusterName,
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
			Cluster:     p.clusterName,
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
				Cluster:   p.clusterName,
			})
		}
	}
	return result
}

func (p *KubernetesParser) parseClientTrafficPolicies(ctx context.Context) []model.ClientTrafficPolicyInfo {
	gvr := schema.GroupVersionResource{
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "clienttrafficpolicies",
	}

	list, err := p.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Debug("no client traffic policies found", "error", err)
		return nil
	}

	var result []model.ClientTrafficPolicyInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})
		targetRef, _ := spec["targetRef"].(map[string]interface{})
		sectionName := strVal(targetRef, "sectionName")
		if sectionName == "" {
			continue
		}

		optional := false
		if tls, ok := spec["tls"].(map[string]interface{}); ok {
			if cv, ok := tls["clientValidation"].(map[string]interface{}); ok {
				if opt, ok := cv["optional"].(bool); ok {
					optional = opt
				}
			}
		}

		result = append(result, model.ClientTrafficPolicyInfo{
			Name:        item.GetName(),
			SectionName: sectionName,
			Optional:    optional,
		})
	}
	return result
}

func (p *KubernetesParser) parseServiceEntries(ctx context.Context) []model.ServiceEntryInfo {
	gvr := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1",
		Resource: "serviceentries",
	}

	list, err := p.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Debug("failed to list serviceentries (CRD may not exist)", "error", err)
		return nil
	}

	var result []model.ServiceEntryInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})

		var hosts []string
		if hs, ok := spec["hosts"].([]interface{}); ok {
			for _, h := range hs {
				if s, ok := h.(string); ok {
					hosts = append(hosts, s)
				}
			}
		}

		location, _ := spec["location"].(string)

		var endpointAddr, network string
		if endpoints, ok := spec["endpoints"].([]interface{}); ok && len(endpoints) > 0 {
			if ep, ok := endpoints[0].(map[string]interface{}); ok {
				endpointAddr, _ = ep["address"].(string)
				if labels, ok := ep["labels"].(map[string]interface{}); ok {
					network, _ = labels["topology.istio.io/network"].(string)
				}
			}
		}

		result = append(result, model.ServiceEntryInfo{
			Name:            item.GetName(),
			Namespace:       item.GetNamespace(),
			Cluster:         p.clusterName,
			Hosts:           hosts,
			Location:        location,
			EndpointAddress: endpointAddr,
			Network:         network,
		})
	}
	return result
}

func (p *KubernetesParser) parseEastWestGateways(ctx context.Context) []model.EastWestGateway {
	list, err := p.typed.CoreV1().Services("istio-system").List(ctx, metav1.ListOptions{
		LabelSelector: "topology.istio.io/network",
	})
	if err != nil {
		slog.Warn("failed to list east-west gateway services", "error", err)
		return nil
	}

	var result []model.EastWestGateway
	for _, svc := range list.Items {
		network := svc.Labels["topology.istio.io/network"]

		ip := ""
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			ip = svc.Status.LoadBalancer.Ingress[0].IP
		}
		if ip == "" {
			ip = svc.Spec.LoadBalancerIP
		}

		port := 15443
		for _, p := range svc.Spec.Ports {
			if p.Port == 15443 {
				port = int(p.Port)
				break
			}
		}

		result = append(result, model.EastWestGateway{
			Name:    svc.Name,
			IP:      ip,
			Port:    port,
			Network: network,
		})
	}
	return result
}

func (p *KubernetesParser) parseLoadBalancers(ctx context.Context) []model.LoadBalancerService {
	list, err := p.typed.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list services", "error", err)
		return nil
	}

	var result []model.LoadBalancerService
	for _, svc := range list.Items {
		if svc.Spec.Type != "LoadBalancer" {
			continue
		}

		ip := ""
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			ip = svc.Status.LoadBalancer.Ingress[0].IP
		}
		if ip == "" {
			ip = svc.Spec.LoadBalancerIP
		}
		if ip == "" {
			continue
		}

		var ports []int
		for _, p := range svc.Spec.Ports {
			ports = append(ports, int(p.Port))
		}

		result = append(result, model.LoadBalancerService{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			IP:        ip,
			Ports:     ports,
		})
	}
	return result
}

func (p *KubernetesParser) parseHelmReleases(ctx context.Context) []model.HelmReleaseInfo {
	gvr := schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}

	list, err := p.dynamic.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list helmreleases (CRD may not exist)", "error", err)
		return nil
	}

	var result []model.HelmReleaseInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})
		chart, _ := spec["chart"].(map[string]interface{})
		chartSpec, _ := chart["spec"].(map[string]interface{})

		chartName := strVal(chartSpec, "chart")
		version := strVal(chartSpec, "version")

		repoName := ""
		repoNS := ""
		if sourceRef, ok := chartSpec["sourceRef"].(map[string]interface{}); ok {
			repoName = strVal(sourceRef, "name")
			repoNS = strVal(sourceRef, "namespace")
		}
		if repoNS == "" {
			repoNS = item.GetNamespace()
		}

		// Try to get appVersion from status
		appVersion := ""
		if status, ok := item.Object["status"].(map[string]interface{}); ok {
			if history, ok := status["history"].([]interface{}); ok && len(history) > 0 {
				if latest, ok := history[0].(map[string]interface{}); ok {
					appVersion = strVal(latest, "appVersion")
				}
			}
		}

		result = append(result, model.HelmReleaseInfo{
			Name:       item.GetName(),
			Namespace:  item.GetNamespace(),
			Cluster:    p.clusterName,
			ChartName:  chartName,
			Version:    version,
			RepoName:   repoName,
			RepoNS:     repoNS,
			AppVersion: appVersion,
		})
	}
	return result
}

func (p *KubernetesParser) parseHelmRepositories(ctx context.Context) []model.HelmRepositoryInfo {
	gvr := schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "helmrepositories",
	}

	list, err := p.dynamic.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list helmrepositories (CRD may not exist)", "error", err)
		return nil
	}

	var result []model.HelmRepositoryInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})

		repoType := strVal(spec, "type")
		if repoType == "" {
			repoType = "default"
		}

		result = append(result, model.HelmRepositoryInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Cluster:   p.clusterName,
			Type:      repoType,
			URL:       strVal(spec, "url"),
		})
	}
	return result
}

func (p *KubernetesParser) parsePods(ctx context.Context) []model.PodImageInfo {
	list, err := p.typed.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list pods", "error", err)
		return nil
	}

	// Build imageID lookup from container statuses
	type statusKey struct {
		podNS, podName, container string
	}

	var result []model.PodImageInfo
	for _, pod := range list.Items {
		// Skip terminal pods
		phase := pod.Status.Phase
		if phase == "Succeeded" || phase == "Failed" {
			continue
		}

		// Build image and imageID maps from status (status has resolved image refs)
		statusImages := make(map[string]string)
		imageIDs := make(map[string]string)
		for _, cs := range pod.Status.ContainerStatuses {
			statusImages[cs.Name] = cs.Image
			imageIDs[cs.Name] = cs.ImageID
		}
		for _, cs := range pod.Status.InitContainerStatuses {
			statusImages[cs.Name] = cs.Image
			imageIDs[cs.Name] = cs.ImageID
		}

		for _, c := range pod.Spec.Containers {
			img := c.Image
			if resolved := statusImages[c.Name]; resolved != "" {
				img = resolved
			}
			result = append(result, model.PodImageInfo{
				Namespace:     pod.Namespace,
				PodName:       pod.Name,
				Container:     c.Name,
				Image:         img,
				ImageID:       imageIDs[c.Name],
				InitContainer: false,
			})
		}
		for _, c := range pod.Spec.InitContainers {
			img := c.Image
			if resolved := statusImages[c.Name]; resolved != "" {
				img = resolved
			}
			result = append(result, model.PodImageInfo{
				Namespace:     pod.Namespace,
				PodName:       pod.Name,
				Container:     c.Name,
				Image:         img,
				ImageID:       imageIDs[c.Name],
				InitContainer: true,
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
