package parser

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"

	"golang.org/x/sync/errgroup"
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

// ParseWorkloads returns workload data for this cluster.
func (p *KubernetesParser) ParseWorkloads(ctx context.Context) []model.WorkloadInfo {
	return p.parseWorkloads(ctx)
}

// ParseStorage returns storage data for this cluster.
func (p *KubernetesParser) ParseStorage(ctx context.Context) []model.StorageInfo {
	return p.parseStorage(ctx)
}

// ParseCRDs returns CRD data for this cluster.
func (p *KubernetesParser) ParseCRDs(ctx context.Context) []model.CRDInfo {
	return p.parseCRDs(ctx)
}

// ParseQuotas returns ResourceQuota and LimitRange data for this cluster.
func (p *KubernetesParser) ParseQuotas(ctx context.Context) []model.QuotaInfo {
	return p.parseQuotas(ctx)
}

// ParseCertificates returns cert-manager Certificate data for this cluster.
func (p *KubernetesParser) ParseCertificates(ctx context.Context) []model.CertificateInfo {
	return p.parseCertificates(ctx)
}

// ParseNetworkPolicies returns NetworkPolicy data for this cluster.
func (p *KubernetesParser) ParseNetworkPolicies(ctx context.Context) []model.NetworkPolicyInfo {
	return p.parseNetworkPolicies(ctx)
}

// ParseConfigs returns ConfigMap and Secret metadata for this cluster.
func (p *KubernetesParser) ParseConfigs(ctx context.Context) []model.ConfigInfo {
	return p.parseConfigs(ctx)
}

// ParseServices returns Service data for this cluster.
func (p *KubernetesParser) ParseServices(ctx context.Context) []model.ServiceInfo {
	return p.parseServices(ctx)
}

// ParseRBAC returns RBAC binding data for this cluster.
func (p *KubernetesParser) ParseRBAC(ctx context.Context) []model.RBACBindingInfo {
	return p.parseRBAC(ctx)
}

// ParseVeleroSchedules returns Velero backup schedule data for this cluster.
func (p *KubernetesParser) ParseVeleroSchedules(ctx context.Context) []model.VeleroScheduleInfo {
	return p.parseVeleroSchedules(ctx)
}

// ParseAll queries all supported resources and returns cluster data.
// All parse methods run concurrently via errgroup for faster collection.
func (p *KubernetesParser) ParseAll(ctx context.Context) *model.ClusterData {
	data := &model.ClusterData{}
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error { data.Nodes = p.parseNodes(gctx); return nil })
	g.Go(func() error { data.Flux = p.parseFluxKustomizations(gctx); return nil })
	g.Go(func() error { data.Gateways = p.parseGateways(gctx); return nil })
	g.Go(func() error { data.HTTPRoutes = p.parseHTTPRoutes(gctx); return nil })
	g.Go(func() error { data.Namespaces = p.parseNamespaces(gctx); return nil })
	g.Go(func() error { data.SecurityPolicies = p.parseSecurityPolicies(gctx); return nil })
	g.Go(func() error { data.ClientTrafficPolicies = p.parseClientTrafficPolicies(gctx); return nil })
	g.Go(func() error { data.ServiceEntries = p.parseServiceEntries(gctx); return nil })
	g.Go(func() error { data.EastWestGateways = p.parseEastWestGateways(gctx); return nil })
	g.Go(func() error { data.LoadBalancers = p.parseLoadBalancers(gctx); return nil })
	g.Go(func() error { data.HelmReleases = p.parseHelmReleases(gctx); return nil })
	g.Go(func() error { data.HelmRepositories = p.parseHelmRepositories(gctx); return nil })
	g.Go(func() error { data.Pods = p.parsePods(gctx); return nil })
	g.Go(func() error { data.Workloads = p.parseWorkloads(gctx); return nil })
	g.Go(func() error { data.Storage = p.parseStorage(gctx); return nil })
	g.Go(func() error { data.CRDs = p.parseCRDs(gctx); return nil })
	g.Go(func() error { data.Quotas = p.parseQuotas(gctx); return nil })
	g.Go(func() error { data.Certificates = p.parseCertificates(gctx); return nil })
	g.Go(func() error { data.NetworkPolicies = p.parseNetworkPolicies(gctx); return nil })
	g.Go(func() error { data.Configs = p.parseConfigs(gctx); return nil })
	g.Go(func() error { data.Services = p.parseServices(gctx); return nil })
	g.Go(func() error { data.RBACBindings = p.parseRBAC(gctx); return nil })
	g.Go(func() error { data.VeleroSchedules = p.parseVeleroSchedules(gctx); return nil })

	g.Wait()
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

func (p *KubernetesParser) parseWorkloads(ctx context.Context) []model.WorkloadInfo {
	var result []model.WorkloadInfo

	// Deployments
	deps, err := p.typed.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list deployments", "error", err)
	} else {
		for _, d := range deps.Items {
			var images []string
			for _, c := range d.Spec.Template.Spec.Containers {
				images = append(images, c.Image)
			}
			strategy := string(d.Spec.Strategy.Type)
			result = append(result, model.WorkloadInfo{
				Name:           d.Name,
				Namespace:      d.Namespace,
				Cluster:        p.clusterName,
				Kind:           "Deployment",
				Replicas:       ptrInt32(d.Spec.Replicas),
				ReadyReplicas:  d.Status.ReadyReplicas,
				UpdateStrategy: strategy,
				Images:         images,
				Labels:         d.Labels,
				CreatedAt:      d.CreationTimestamp.Format("2006-01-02"),
			})
		}
	}

	// StatefulSets
	sts, err := p.typed.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list statefulsets", "error", err)
	} else {
		for _, s := range sts.Items {
			var images []string
			for _, c := range s.Spec.Template.Spec.Containers {
				images = append(images, c.Image)
			}
			strategy := string(s.Spec.UpdateStrategy.Type)
			result = append(result, model.WorkloadInfo{
				Name:           s.Name,
				Namespace:      s.Namespace,
				Cluster:        p.clusterName,
				Kind:           "StatefulSet",
				Replicas:       ptrInt32(s.Spec.Replicas),
				ReadyReplicas:  s.Status.ReadyReplicas,
				UpdateStrategy: strategy,
				Images:         images,
				Labels:         s.Labels,
				CreatedAt:      s.CreationTimestamp.Format("2006-01-02"),
			})
		}
	}

	// DaemonSets
	dss, err := p.typed.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list daemonsets", "error", err)
	} else {
		for _, d := range dss.Items {
			var images []string
			for _, c := range d.Spec.Template.Spec.Containers {
				images = append(images, c.Image)
			}
			strategy := string(d.Spec.UpdateStrategy.Type)
			result = append(result, model.WorkloadInfo{
				Name:           d.Name,
				Namespace:      d.Namespace,
				Cluster:        p.clusterName,
				Kind:           "DaemonSet",
				Replicas:       d.Status.DesiredNumberScheduled,
				ReadyReplicas:  d.Status.NumberReady,
				UpdateStrategy: strategy,
				Images:         images,
				Labels:         d.Labels,
				CreatedAt:      d.CreationTimestamp.Format("2006-01-02"),
			})
		}
	}

	// CronJobs
	cjs, err := p.typed.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list cronjobs", "error", err)
	} else {
		for _, c := range cjs.Items {
			var images []string
			for _, ct := range c.Spec.JobTemplate.Spec.Template.Spec.Containers {
				images = append(images, ct.Image)
			}
			result = append(result, model.WorkloadInfo{
				Name:           c.Name,
				Namespace:      c.Namespace,
				Cluster:        p.clusterName,
				Kind:           "CronJob",
				Replicas:       0,
				ReadyReplicas:  0,
				UpdateStrategy: c.Spec.Schedule,
				Images:         images,
				Labels:         c.Labels,
				CreatedAt:      c.CreationTimestamp.Format("2006-01-02"),
			})
		}
	}

	return result
}

func (p *KubernetesParser) parseStorage(ctx context.Context) []model.StorageInfo {
	var result []model.StorageInfo

	// PersistentVolumes
	pvs, err := p.typed.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list persistentvolumes", "error", err)
	} else {
		for _, pv := range pvs.Items {
			var accessModes []string
			for _, am := range pv.Spec.AccessModes {
				accessModes = append(accessModes, string(am))
			}
			capacity := ""
			if q, ok := pv.Spec.Capacity["storage"]; ok {
				capacity = q.String()
			}
			boundTo := ""
			if pv.Spec.ClaimRef != nil {
				boundTo = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
			}
			result = append(result, model.StorageInfo{
				Name:          pv.Name,
				Cluster:       p.clusterName,
				Kind:          "PersistentVolume",
				Capacity:      capacity,
				AccessModes:   strings.Join(accessModes, ", "),
				Status:        string(pv.Status.Phase),
				StorageClass:  pv.Spec.StorageClassName,
				ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
				BoundTo:       boundTo,
			})
		}
	}

	// PersistentVolumeClaims
	pvcs, err := p.typed.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list persistentvolumeclaims", "error", err)
	} else {
		for _, pvc := range pvcs.Items {
			var accessModes []string
			for _, am := range pvc.Spec.AccessModes {
				accessModes = append(accessModes, string(am))
			}
			capacity := ""
			if pvc.Status.Capacity != nil {
				if q, ok := pvc.Status.Capacity["storage"]; ok {
					capacity = q.String()
				}
			}
			sc := ""
			if pvc.Spec.StorageClassName != nil {
				sc = *pvc.Spec.StorageClassName
			}
			result = append(result, model.StorageInfo{
				Name:         pvc.Name,
				Namespace:    pvc.Namespace,
				Cluster:      p.clusterName,
				Kind:         "PersistentVolumeClaim",
				Capacity:     capacity,
				AccessModes:  strings.Join(accessModes, ", "),
				Status:       string(pvc.Status.Phase),
				StorageClass: sc,
				BoundTo:      pvc.Spec.VolumeName,
			})
		}
	}

	// StorageClasses
	scs, err := p.typed.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list storageclasses", "error", err)
	} else {
		for _, sc := range scs.Items {
			reclaimPolicy := ""
			if sc.ReclaimPolicy != nil {
				reclaimPolicy = string(*sc.ReclaimPolicy)
			}
			result = append(result, model.StorageInfo{
				Name:          sc.Name,
				Cluster:       p.clusterName,
				Kind:          "StorageClass",
				ReclaimPolicy: reclaimPolicy,
			})
		}
	}

	return result
}

func (p *KubernetesParser) parseCRDs(ctx context.Context) []model.CRDInfo {
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	list, err := p.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list CRDs", "error", err)
		return nil
	}

	var result []model.CRDInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})
		group := strVal(spec, "group")
		scope := strVal(spec, "scope")

		var versions []string
		if vs, ok := spec["versions"].([]interface{}); ok {
			for _, v := range vs {
				if vm, ok := v.(map[string]interface{}); ok {
					if name := strVal(vm, "name"); name != "" {
						versions = append(versions, name)
					}
				}
			}
		}

		result = append(result, model.CRDInfo{
			Name:     item.GetName(),
			Group:    group,
			Versions: versions,
			Scope:    scope,
			Cluster:  p.clusterName,
		})
	}
	return result
}

func (p *KubernetesParser) parseQuotas(ctx context.Context) []model.QuotaInfo {
	var result []model.QuotaInfo

	// ResourceQuotas
	rqs, err := p.typed.CoreV1().ResourceQuotas("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list resourcequotas", "error", err)
	} else {
		for _, rq := range rqs.Items {
			resources := make(map[string]string)
			for name, qty := range rq.Spec.Hard {
				resources[string(name)] = qty.String()
			}
			result = append(result, model.QuotaInfo{
				Name:      rq.Name,
				Namespace: rq.Namespace,
				Cluster:   p.clusterName,
				Kind:      "ResourceQuota",
				Resources: resources,
			})
		}
	}

	// LimitRanges
	lrs, err := p.typed.CoreV1().LimitRanges("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list limitranges", "error", err)
	} else {
		for _, lr := range lrs.Items {
			resources := make(map[string]string)
			for _, item := range lr.Spec.Limits {
				prefix := string(item.Type)
				for name, qty := range item.Default {
					resources[prefix+".default."+string(name)] = qty.String()
				}
				for name, qty := range item.DefaultRequest {
					resources[prefix+".defaultRequest."+string(name)] = qty.String()
				}
				for name, qty := range item.Max {
					resources[prefix+".max."+string(name)] = qty.String()
				}
				for name, qty := range item.Min {
					resources[prefix+".min."+string(name)] = qty.String()
				}
			}
			result = append(result, model.QuotaInfo{
				Name:      lr.Name,
				Namespace: lr.Namespace,
				Cluster:   p.clusterName,
				Kind:      "LimitRange",
				Resources: resources,
			})
		}
	}

	return result
}

func (p *KubernetesParser) parseCertificates(ctx context.Context) []model.CertificateInfo {
	gvr := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "certificates",
	}

	list, err := p.dynamic.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Debug("failed to list certificates (cert-manager CRD may not exist)", "error", err)
		return nil
	}

	var result []model.CertificateInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})

		var dnsNames []string
		if dns, ok := spec["dnsNames"].([]interface{}); ok {
			for _, d := range dns {
				if s, ok := d.(string); ok {
					dnsNames = append(dnsNames, s)
				}
			}
		}

		issuerName := ""
		issuerKind := ""
		if issuerRef, ok := spec["issuerRef"].(map[string]interface{}); ok {
			issuerName = strVal(issuerRef, "name")
			issuerKind = strVal(issuerRef, "kind")
			if issuerKind == "" {
				issuerKind = "Issuer"
			}
		}

		// Parse status
		status, _ := item.Object["status"].(map[string]interface{})
		notBefore := strVal(status, "notBefore")
		notAfter := strVal(status, "notAfter")
		renewalTime := strVal(status, "renewalTime")

		ready := false
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				if cm, ok := c.(map[string]interface{}); ok {
					if strVal(cm, "type") == "Ready" && strVal(cm, "status") == "True" {
						ready = true
						break
					}
				}
			}
		}

		result = append(result, model.CertificateInfo{
			Name:        item.GetName(),
			Namespace:   item.GetNamespace(),
			Cluster:     p.clusterName,
			DNSNames:    dnsNames,
			IssuerName:  issuerName,
			IssuerKind:  issuerKind,
			NotBefore:   notBefore,
			NotAfter:    notAfter,
			RenewalTime: renewalTime,
			Ready:       ready,
		})
	}
	return result
}

func (p *KubernetesParser) parseNetworkPolicies(ctx context.Context) []model.NetworkPolicyInfo {
	list, err := p.typed.NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list networkpolicies", "error", err)
		return nil
	}

	var result []model.NetworkPolicyInfo
	for _, np := range list.Items {
		// Build pod selector summary
		var selParts []string
		for k, v := range np.Spec.PodSelector.MatchLabels {
			selParts = append(selParts, k+"="+v)
		}
		sort.Strings(selParts)
		podSelector := strings.Join(selParts, ", ")
		if podSelector == "" {
			podSelector = "(all pods)"
		}

		var policyTypes []string
		for _, pt := range np.Spec.PolicyTypes {
			policyTypes = append(policyTypes, string(pt))
		}

		// Summarize ingress rules
		ingressSummary := ""
		if len(np.Spec.Ingress) > 0 {
			ingressSummary = fmt.Sprintf("%d rule(s)", len(np.Spec.Ingress))
		}

		// Summarize egress rules
		egressSummary := ""
		if len(np.Spec.Egress) > 0 {
			egressSummary = fmt.Sprintf("%d rule(s)", len(np.Spec.Egress))
		}

		result = append(result, model.NetworkPolicyInfo{
			Name:           np.Name,
			Namespace:      np.Namespace,
			Cluster:        p.clusterName,
			PodSelector:    podSelector,
			PolicyTypes:    policyTypes,
			IngressSummary: ingressSummary,
			EgressSummary:  egressSummary,
		})
	}
	return result
}

func (p *KubernetesParser) parseConfigs(ctx context.Context) []model.ConfigInfo {
	var result []model.ConfigInfo

	// ConfigMaps
	cms, err := p.typed.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list configmaps", "error", err)
	} else {
		for _, cm := range cms.Items {
			result = append(result, model.ConfigInfo{
				Name:      cm.Name,
				Namespace: cm.Namespace,
				Cluster:   p.clusterName,
				Kind:      "ConfigMap",
				KeyCount:  len(cm.Data) + len(cm.BinaryData),
			})
		}
	}

	// Secrets — only metadata, never expose data
	secrets, err := p.typed.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list secrets", "error", err)
	} else {
		for _, s := range secrets.Items {
			result = append(result, model.ConfigInfo{
				Name:      s.Name,
				Namespace: s.Namespace,
				Cluster:   p.clusterName,
				Kind:      "Secret",
				KeyCount:  len(s.Data),
			})
		}
	}

	return result
}

func (p *KubernetesParser) parseServices(ctx context.Context) []model.ServiceInfo {
	list, err := p.typed.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list services for service map", "error", err)
		return nil
	}

	var result []model.ServiceInfo
	for _, svc := range list.Items {
		var portStrs []string
		for _, port := range svc.Spec.Ports {
			portStrs = append(portStrs, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
		}

		result = append(result, model.ServiceInfo{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Cluster:   p.clusterName,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Ports:     strings.Join(portStrs, ", "),
			Selector:  svc.Spec.Selector,
		})
	}
	return result
}

func (p *KubernetesParser) parseRBAC(ctx context.Context) []model.RBACBindingInfo {
	var result []model.RBACBindingInfo

	// ClusterRoleBindings
	crbs, err := p.typed.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list clusterrolebindings", "error", err)
	} else {
		for _, crb := range crbs.Items {
			for _, subject := range crb.Subjects {
				result = append(result, model.RBACBindingInfo{
					SubjectName: subject.Name,
					SubjectKind: subject.Kind,
					RoleName:    crb.RoleRef.Name,
					RoleKind:    crb.RoleRef.Kind,
					Namespace:   subject.Namespace,
					Cluster:     p.clusterName,
				})
			}
		}
	}

	// RoleBindings
	rbs, err := p.typed.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Warn("failed to list rolebindings", "error", err)
	} else {
		for _, rb := range rbs.Items {
			for _, subject := range rb.Subjects {
				ns := rb.Namespace
				if subject.Namespace != "" {
					ns = subject.Namespace
				}
				result = append(result, model.RBACBindingInfo{
					SubjectName: subject.Name,
					SubjectKind: subject.Kind,
					RoleName:    rb.RoleRef.Name,
					RoleKind:    rb.RoleRef.Kind,
					Namespace:   ns,
					Cluster:     p.clusterName,
				})
			}
		}
	}

	return result
}

func (p *KubernetesParser) parseVeleroSchedules(ctx context.Context) []model.VeleroScheduleInfo {
	gvr := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "schedules",
	}

	list, err := p.dynamic.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Debug("failed to list velero schedules (CRD may not exist)", "error", err)
		return nil
	}

	var result []model.VeleroScheduleInfo
	for _, item := range list.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})
		schedule := strVal(spec, "schedule")

		var includedNS, excludedNS []string
		if tmpl, ok := spec["template"].(map[string]interface{}); ok {
			if ins, ok := tmpl["includedNamespaces"].([]interface{}); ok {
				for _, n := range ins {
					if s, ok := n.(string); ok {
						includedNS = append(includedNS, s)
					}
				}
			}
			if ens, ok := tmpl["excludedNamespaces"].([]interface{}); ok {
				for _, n := range ens {
					if s, ok := n.(string); ok {
						excludedNS = append(excludedNS, s)
					}
				}
			}
		}

		ttl := strVal(spec, "ttl")
		if ttl == "" {
			if tmpl, ok := spec["template"].(map[string]interface{}); ok {
				ttl = strVal(tmpl, "ttl")
			}
		}

		status, _ := item.Object["status"].(map[string]interface{})
		phase := strVal(status, "phase")

		result = append(result, model.VeleroScheduleInfo{
			Name:       item.GetName(),
			Namespace:  item.GetNamespace(),
			Cluster:    p.clusterName,
			Schedule:   schedule,
			IncludedNS: includedNS,
			ExcludedNS: excludedNS,
			TTL:        ttl,
			Phase:      phase,
		})
	}
	return result
}

// ptrInt32 dereferences an int32 pointer, returning 1 if nil (default replicas).
func ptrInt32(p *int32) int32 {
	if p == nil {
		return 1
	}
	return *p
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
