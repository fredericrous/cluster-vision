package model

// ClusterData holds all parsed cluster state.
type ClusterData struct {
	PrimaryCluster        string
	Nodes                 []NodeInfo
	Flux                  []FluxKustomization
	Gateways              []GatewayInfo
	HTTPRoutes            []HTTPRouteInfo
	Namespaces            []NamespaceInfo
	SecurityPolicies      []SecurityPolicyInfo
	ClientTrafficPolicies []ClientTrafficPolicyInfo
	InfraSources          []InfraSource
	ServiceEntries        []ServiceEntryInfo
	EastWestGateways      []EastWestGateway
	LoadBalancers         []LoadBalancerService
	HelmReleases          []HelmReleaseInfo
	HelmRepositories      []HelmRepositoryInfo
	Pods                  []PodImageInfo
	Workloads             []WorkloadInfo
	Storage               []StorageInfo
	CRDs                  []CRDInfo
	Quotas                []QuotaInfo
	Certificates          []CertificateInfo
	NetworkPolicies       []NetworkPolicyInfo
	Configs               []ConfigInfo
	Services              []ServiceInfo
	RBACBindings          []RBACBindingInfo
	VeleroSchedules       []VeleroScheduleInfo
	ImageVulns            []ImageVuln
}

// ImageVuln represents vulnerability counts for a container image from trivy-operator.
type ImageVuln struct {
	Image    string // "registry/repo:tag"
	Cluster  string
	Critical int
	High     int
	Medium   int
	Low      int

	// CVEs holds the deduplicated CVE IDs from Trivy's
	// VulnerabilityReport.report.vulnerabilities[].vulnerabilityID. Captured
	// at parse time so we can later cross-reference with KEV/EPSS without
	// re-reading the Trivy CRs. May be nil for older reports without per-CVE
	// detail; consumers should handle len()==0 gracefully.
	CVEs []string

	// KEV/EPSS enrichment — populated by the discovery enrichWithVulns pass
	// after CVEs are looked up against the cve_enrichment cache. Image-level,
	// not namespace-level (this struct is collapsed across namespaces by the
	// existing parser). The metric layer recovers namespace at emit time by
	// joining with PodImageInfo.
	KEVCount   int      // count of CVEs in this image listed on CISA KEV
	KEVCVEs    []string // KEV-flagged CVE IDs (for tooltip / table display)
	MaxEPSS    float64  // highest EPSS score across the image's CVEs
	MaxEPSSCVE string   // CVE that drove MaxEPSS (for tooltip)
}

// PodImageInfo represents a container image running in a pod.
type PodImageInfo struct {
	Cluster       string // stamped at parse time so the merged slice from
	                    // multiple parsers stays attributable per cluster
	Namespace     string
	PodName       string
	Container     string
	Image         string // full image ref (registry/repo:tag)
	ImageID       string // resolved digest from pod status
	InitContainer bool
}

// HelmReleaseInfo represents a Flux HelmRelease resource.
type HelmReleaseInfo struct {
	Name       string
	Namespace  string
	Cluster    string
	ChartName  string
	Version    string // deployed chart version
	RepoName   string // sourceRef name
	RepoNS     string // sourceRef namespace
	AppVersion string // from status, if available
}

// HelmRepositoryInfo represents a Flux HelmRepository source.
type HelmRepositoryInfo struct {
	Name      string
	Namespace string
	Cluster   string
	Type      string // "oci" or "default" (HTTP)
	URL       string
}

// ServiceEntryInfo represents an Istio ServiceEntry resource.
type ServiceEntryInfo struct {
	Name            string
	Namespace       string
	Cluster         string
	Hosts           []string
	Location        string // "MESH_EXTERNAL" etc
	EndpointAddress string // remote gateway IP
	Network         string // e.g. "nas-network" from endpoint label
}

// EastWestGateway represents an Istio east-west gateway Service.
type EastWestGateway struct {
	Name    string
	Cluster string
	IP      string
	Port    int
	Network string // from service label topology.istio.io/network
}

// LoadBalancerService represents a Kubernetes Service of type LoadBalancer.
type LoadBalancerService struct {
	Name      string
	Namespace string
	Cluster   string
	IP        string
	Ports     []int
}

// DataSource defines where to read infrastructure data from.
// The file is expected to be mounted from a Kubernetes Secret (e.g. via ExternalSecrets from Vault).
type DataSource struct {
	Name     string `json:"name"`
	Type     string `json:"type"`     // "tfstate" | "docker-compose" | "kubernetes"
	Path     string `json:"path"`     // path to the mounted file
	Platform string `json:"platform"` // optional: platform name for K8s nodes (e.g. "QNAP")
}

// InfraSource holds parsed infrastructure data from one source.
type InfraSource struct {
	Name           string
	Type           string // "tfstate" | "docker-compose"
	TerraformNodes []TerraformNode
	DockerCompose  *DockerCompose
}

// DockerCompose represents a parsed docker-compose file.
type DockerCompose struct {
	Services []DockerService
}

// DockerService represents a single service in docker-compose.
type DockerService struct {
	Name       string
	Image      string
	Hostname   string
	IP         string
	Ports      []string
	Volumes    []string
	Networks   []string
	Command    string
	Privileged bool
}

// NodeInfo represents a Kubernetes node.
type NodeInfo struct {
	Name             string
	Cluster          string
	IP               string
	Roles            []string
	CPU              string
	Memory           string
	Labels           map[string]string
	OSImage          string // e.g. "Talos (v1.9.0)"
	KubeletVersion   string // e.g. "v1.32.0"
	ContainerRuntime string // e.g. "containerd://2.0.0"
	KernelVersion    string // e.g. "6.6.64-talos"
	Architecture     string // e.g. "amd64"
	ProviderID       string // node.Spec.ProviderID (e.g. "proxmox://region/zone/uuid")
	Platform         string // platform name from DataSource config (e.g. "QNAP")
}

// FluxKustomization represents a Flux Kustomization resource.
type FluxKustomization struct {
	Name      string
	Namespace string
	Path      string
	DependsOn []string
	Cluster   string
}

// GatewayInfo represents a Gateway API Gateway resource.
type GatewayInfo struct {
	Name             string
	Namespace        string
	Cluster          string
	GatewayClassName string
	Listeners        []ListenerInfo
}

// ListenerInfo represents a single Gateway listener.
type ListenerInfo struct {
	Name     string
	Hostname string
	Protocol string
	Port     int
}

// HTTPRouteInfo represents an HTTPRoute resource.
type HTTPRouteInfo struct {
	Name        string
	Namespace   string
	Cluster     string
	Hostnames   []string
	SectionName string
	Backends    []BackendRef
}

// BackendRef is a reference to a backend service.
type BackendRef struct {
	Name string
	Port int
}

// NamespaceInfo holds security-relevant labels from a namespace.
type NamespaceInfo struct {
	Name        string
	Cluster     string
	Ambient     bool
	Waypoint    bool
	Backup      bool
	MTLS        bool
	PodSecurity string
}

// SecurityPolicyInfo tracks external auth policies per namespace.
type SecurityPolicyInfo struct {
	Name      string
	Namespace string
	Cluster   string
}

// ClientTrafficPolicyInfo tracks client mTLS policies at the ingress.
type ClientTrafficPolicyInfo struct {
	Name        string
	Cluster     string
	SectionName string
	Optional    bool
}

// TerraformNode represents a VM parsed from Terraform state.
type TerraformNode struct {
	Name       string
	IP         string
	Cores      int
	MemoryMB   int
	OSDiskGB   int
	DataDiskGB int
	GPU        string
	Role       string
	Provider   string
}

// WorkloadInfo represents a Kubernetes workload (Deployment, StatefulSet, DaemonSet, CronJob).
type WorkloadInfo struct {
	Name           string
	Namespace      string
	Cluster        string
	Kind           string // "Deployment", "StatefulSet", "DaemonSet", "CronJob"
	Replicas       int32
	ReadyReplicas  int32
	UpdateStrategy string
	Images         []string
	Labels         map[string]string
	CreatedAt      string
}

// StorageInfo represents a PV, PVC, or StorageClass.
type StorageInfo struct {
	Name          string
	Namespace     string
	Cluster       string
	Kind          string // "PersistentVolume", "PersistentVolumeClaim", "StorageClass"
	Capacity      string
	AccessModes   string
	Status        string
	StorageClass  string
	ReclaimPolicy string
	BoundTo       string
}

// CRDInfo represents a CustomResourceDefinition.
type CRDInfo struct {
	Name     string
	Group    string
	Versions []string
	Scope    string
	Cluster  string
}

// QuotaInfo represents a ResourceQuota or LimitRange.
type QuotaInfo struct {
	Name      string
	Namespace string
	Cluster   string
	Kind      string // "ResourceQuota" or "LimitRange"
	Resources map[string]string
}

// CertificateInfo represents a cert-manager Certificate.
type CertificateInfo struct {
	Name        string
	Namespace   string
	Cluster     string
	DNSNames    []string
	IssuerName  string
	IssuerKind  string
	NotBefore   string
	NotAfter    string
	RenewalTime string
	Ready       bool
}

// NetworkPolicyInfo represents a Kubernetes NetworkPolicy.
type NetworkPolicyInfo struct {
	Name           string
	Namespace      string
	Cluster        string
	PodSelector    string
	PolicyTypes    []string
	IngressSummary string
	EgressSummary  string
}

// ConfigInfo represents a ConfigMap or Secret (metadata only, never secret data).
type ConfigInfo struct {
	Name         string
	Namespace    string
	Cluster      string
	Kind         string // "ConfigMap" or "Secret"
	KeyCount     int
	ReferencedBy []string
}

// ServiceInfo represents a Kubernetes Service.
type ServiceInfo struct {
	Name      string
	Namespace string
	Cluster   string
	Type      string // "ClusterIP", "NodePort", "LoadBalancer", "ExternalName"
	ClusterIP string
	Ports     string
	Selector  map[string]string
}

// RBACBindingInfo represents a role binding subject-to-role mapping.
type RBACBindingInfo struct {
	SubjectName string
	SubjectKind string // "User", "Group", "ServiceAccount"
	RoleName    string
	RoleKind    string // "Role", "ClusterRole"
	Namespace   string
	Cluster     string
}

// VeleroScheduleInfo represents a Velero backup schedule.
type VeleroScheduleInfo struct {
	Name       string
	Namespace  string
	Cluster    string
	Schedule   string
	IncludedNS []string
	ExcludedNS []string
	TTL        string
	Phase      string
}

// DiagramResult holds a generated diagram.
type DiagramResult struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"` // "mermaid", "markdown", "table", or "flow"
	Content string `json:"content"`
}
