package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

const capabilitySystemPrompt = `You are an enterprise architect. Given a list of applications running in a Kubernetes cluster, generate a business capability hierarchy and map each application to its best-fit capability.

Return ONLY valid JSON with this structure:
{
  "capabilities": [
    {
      "name": "Category Name",
      "description": "Brief description",
      "level": 1,
      "children": [
        {
          "name": "Subcategory",
          "description": "Brief description",
          "level": 2,
          "children": []
        }
      ]
    }
  ],
  "mappings": [
    { "app_name": "app-name", "capability_name": "Subcategory" }
  ]
}

Guidelines:
- Create a 2-level hierarchy (L1 broad categories, L2 specific capabilities)
- Common L1 categories: Communication, Security & Identity, Media & Entertainment, Data Management, Infrastructure, Developer Tools, Monitoring & Observability, Productivity
- Map each application to the most specific (L2) capability
- Every application must be mapped to exactly one capability`

const enrichmentSystemPrompt = `You are an enterprise architect analyzing a Kubernetes application. Given the app metadata, assess the following and return ONLY valid JSON:

{
  "description": "One-sentence description of what this application does",
  "business_criticality": "high|medium|low",
  "criticality_reason": "Brief reason",
  "technical_risk": "high|medium|low",
  "risk_reason": "Brief reason",
  "time_category": "tolerate|invest|migrate|eliminate",
  "time_category_reason": "Brief reason",
  "confidence": 0.85
}

Assessment criteria:
- business_criticality: How important is this for daily operations? Infrastructure (DNS, auth, storage) = high. Main user-facing apps = medium. Experimental/optional = low.
- technical_risk: Consider vulnerability counts, outdated versions, single points of failure. Many critical vulns or very outdated = high. Some issues = medium. Clean = low.
- time_category (TIME model):
  - tolerate: Works fine, no action needed
  - invest: Strategic, worth improving
  - migrate: Should be replaced with better alternative
  - eliminate: Deprecated or unnecessary, remove
- confidence: How confident you are in these assessments (0.0-1.0). Higher for well-known software, lower for unknown/custom apps.`

const dependencySystemPrompt = `You are an enterprise architect. Given a list of applications with their images and services, identify which applications likely depend on each other.

Return ONLY valid JSON:
{
  "dependencies": [
    { "source": "app-name", "target": "dependency-name", "reason": "Brief reason" }
  ]
}

Guidelines:
- Only include dependencies you are reasonably confident about
- Common patterns: apps depend on auth providers (authelia, keycloak), databases (postgres, redis, mariadb), reverse proxies (traefik, nginx), DNS (external-dns, coredns), certificate management (cert-manager)
- source = the app that DEPENDS ON the target
- Do not include circular dependencies
- Do not include infrastructure dependencies that every app would have (like the CNI or kubelet)`

// BuildCapabilityPrompt creates the user message for capability inference.
func BuildCapabilityPrompt(apps []AppContext) string {
	var sb strings.Builder
	sb.WriteString("Applications:\n")
	for _, a := range apps {
		fmt.Fprintf(&sb, "- %s (namespace: %s", a.Name, a.Namespace)
		if a.ChartName != "" {
			fmt.Fprintf(&sb, ", chart: %s", a.ChartName)
		}
		if len(a.Images) > 0 {
			fmt.Fprintf(&sb, ", images: [%s]", strings.Join(a.Images, ", "))
		}
		sb.WriteString(")\n")
	}
	return sb.String()
}

// BuildEnrichmentPrompt creates the user message for per-app enrichment.
func BuildEnrichmentPrompt(app AppContext) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("App %q in namespace %q:\n", app.Name, app.Namespace))
	if app.ChartName != "" {
		sb.WriteString(fmt.Sprintf("- Chart: %s", app.ChartName))
		if app.ChartVersion != "" {
			sb.WriteString(fmt.Sprintf(" v%s", app.ChartVersion))
		}
		sb.WriteString("\n")
	}
	if len(app.Images) > 0 {
		sb.WriteString(fmt.Sprintf("- Images: %s\n", strings.Join(app.Images, ", ")))
	}
	if app.VulnCritical > 0 || app.VulnHigh > 0 {
		sb.WriteString(fmt.Sprintf("- Vulnerabilities: %d critical, %d high\n", app.VulnCritical, app.VulnHigh))
	}
	return sb.String()
}

// BuildDependencyPrompt creates the user message for dependency inference.
func BuildDependencyPrompt(apps []AppContext) string {
	data, _ := json.MarshalIndent(apps, "", "  ")
	return fmt.Sprintf("Applications:\n%s", string(data))
}
