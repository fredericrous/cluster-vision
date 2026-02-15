package model

// ClusterData holds all parsed cluster state.
type ClusterData struct {
	Nodes            []NodeInfo
	Flux             []FluxKustomization
	Gateways         []GatewayInfo
	HTTPRoutes       []HTTPRouteInfo
	Namespaces       []NamespaceInfo
	SecurityPolicies []SecurityPolicyInfo
	InfraSources     []InfraSource
}

// DataSource defines where to fetch infrastructure data from.
type DataSource struct {
	Name   string        `json:"name"`
	Type   string        `json:"type"`   // "tfstate" | "docker-compose"
	Path   string        `json:"path"`   // local file path (optional)
	GitHub *GitHubSource `json:"github"` // fetch from GitHub (optional)
}

// GitHubSource defines how to fetch a file from GitHub.
type GitHubSource struct {
	Repo      string `json:"repo"`      // "owner/repo"
	FilePath  string `json:"filePath"`  // path in repo
	Ref       string `json:"ref"`       // branch/tag, default "main"
	TokenFile string `json:"tokenFile"` // path to file with PAT
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
	Type    string `json:"type"` // "mermaid" or "markdown"
	Content string `json:"content"`
}
