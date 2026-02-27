package agent

// CapabilityInference is the expected JSON structure from the LLM for capability tree generation.
type CapabilityInference struct {
	Capabilities []InferredCapability `json:"capabilities"`
	Mappings     []CapabilityMapping  `json:"mappings"`
}

type InferredCapability struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Level       int                  `json:"level"`
	Children    []InferredCapability `json:"children"`
}

type CapabilityMapping struct {
	AppName        string `json:"app_name"`
	CapabilityName string `json:"capability_name"`
}

// AppEnrichment is the expected JSON structure from the LLM for per-app enrichment.
type AppEnrichment struct {
	Description         string  `json:"description"`
	BusinessCriticality string  `json:"business_criticality"` // high, medium, low
	CriticalityReason   string  `json:"criticality_reason"`
	TechnicalRisk       string  `json:"technical_risk"` // high, medium, low
	RiskReason          string  `json:"risk_reason"`
	TimeCategory        string  `json:"time_category"` // tolerate, invest, migrate, eliminate
	TimeCategoryReason  string  `json:"time_category_reason"`
	Confidence          float64 `json:"confidence"` // 0.0 – 1.0
}

// DependencyInference is the expected JSON structure from the LLM for dependency discovery.
type DependencyInference struct {
	Dependencies []InferredDependency `json:"dependencies"`
}

type InferredDependency struct {
	Source string `json:"source"` // app name
	Target string `json:"target"` // app name
	Reason string `json:"reason"`
}

// AppContext provides the LLM with metadata about a single application for enrichment.
type AppContext struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Cluster      string   `json:"cluster"`
	ChartName    string   `json:"chart_name,omitempty"`
	ChartVersion string   `json:"chart_version,omitempty"`
	Images       []string `json:"images,omitempty"`
	VulnCritical int      `json:"vuln_critical,omitempty"`
	VulnHigh     int      `json:"vuln_high,omitempty"`
	Services     []string `json:"services,omitempty"`
}
