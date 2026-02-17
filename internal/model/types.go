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
	HelmReleases          []HelmReleaseInfo
	HelmRepositories      []HelmRepositoryInfo
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
	IP      string
	Port    int
	Network string // from service label topology.istio.io/network
}

// DataSource defines where to read infrastructure data from.
// The file is expected to be mounted from a Kubernetes Secret (e.g. via ExternalSecrets from Vault).
type DataSource struct {
	Name string `json:"name"`
	Type string `json:"type"` // "tfstate" | "docker-compose"
	Path string `json:"path"` // path to the mounted file
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
	Name   string
	IP     string
	Roles  []string
	CPU    string
	Memory string
	Labels map[string]string
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
	Name      string
	Namespace string
	Listeners []ListenerInfo
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

// DiagramResult holds a generated diagram.
type DiagramResult struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"` // "mermaid", "markdown", "table", or "flow"
	Content string `json:"content"`
}
