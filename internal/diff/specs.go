package diff

// tableSpec declares, per table diagram, which fields identify a row and
// which fields are advisory. Advisory fields come from sources other than
// the cluster (registry "latest" lookups, the vulnerability DB, wall-clock
// derived values) and must not make two snapshots look different.
type tableSpec struct {
	Keys     []string // row identity, in order
	Label    []string // fields shown as the row's label; defaults to Keys
	Advisory []string // excluded from the hash and from Changes
}

var vulnAdvisory = []string{"securityRisk", "vulnSummary"}

var tableSpecs = map[string]tableSpec{
	"certificates":      {Keys: []string{"cluster", "namespace", "name"}, Label: []string{"namespace", "name"}},
	"configs":           {Keys: []string{"cluster", "namespace", "kind", "name"}, Label: []string{"kind", "namespace", "name"}},
	"crds":              {Keys: []string{"cluster", "name"}, Label: []string{"name"}},
	"helm-workloads":    {Keys: []string{"cluster", "namespace", "release", "kind", "workload"}, Label: []string{"release", "kind", "workload"}},
	"images":            {Keys: []string{"image", "tag", "type"}, Label: []string{"image", "tag"}, Advisory: append([]string{"latest", "outdated", "exploitRisk", "exploitSummary", "kevCVEs"}, vulnAdvisory...)},
	"labels":            {Keys: []string{"key"}},
	"namespace-summary": {Keys: []string{"cluster", "namespace"}, Label: []string{"namespace"}},
	"network-policies":  {Keys: []string{"cluster", "namespace", "name"}, Label: []string{"namespace", "name"}},
	"nodes":             {Keys: []string{"cluster", "type", "name"}, Label: []string{"name"}, Advisory: append([]string{"latestOS", "osOutdated", "latestKubelet", "kubeletOutdated"}, vulnAdvisory...)},
	"quotas":            {Keys: []string{"cluster", "namespace", "kind", "name"}, Label: []string{"kind", "namespace", "name"}},
	"rbac":              {Keys: []string{"cluster", "namespace", "subjectKind", "subject", "roleKind", "role"}, Label: []string{"subject", "role"}},
	"security":          {Keys: []string{"cluster", "namespace"}, Label: []string{"namespace"}},
	"service-map":       {Keys: []string{"cluster", "namespace", "name"}, Label: []string{"namespace", "name"}},
	"storage":           {Keys: []string{"cluster", "namespace", "kind", "name"}, Label: []string{"kind", "namespace", "name"}},
	"velero":            {Keys: []string{"cluster", "namespace", "name"}, Label: []string{"name"}},
	"charts":            {Keys: []string{"cluster", "namespace", "release"}, Label: []string{"namespace", "release"}, Advisory: append([]string{"latest", "outdated"}, vulnAdvisory...)},
	"workloads":         {Keys: []string{"cluster", "namespace", "kind", "name"}, Label: []string{"kind", "namespace", "name"}},
}

func specFor(diagramID string) tableSpec {
	return tableSpecs[diagramID]
}
