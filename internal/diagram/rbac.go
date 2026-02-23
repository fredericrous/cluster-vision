package diagram

import (
	"encoding/json"
	"sort"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// RBACRow represents a single subject-to-role binding.
type RBACRow struct {
	Subject     string `json:"subject"`
	SubjectKind string `json:"subjectKind"`
	Role        string `json:"role"`
	RoleKind    string `json:"roleKind"`
	Namespace   string `json:"namespace"`
	Cluster     string `json:"cluster"`
}

// GenerateRBAC produces a table of RBAC bindings.
func GenerateRBAC(data *model.ClusterData) model.DiagramResult {
	if len(data.RBACBindings) == 0 {
		return model.DiagramResult{
			ID:      "rbac",
			Title:   "RBAC Inventory",
			Type:    "markdown",
			Content: "*No RBAC binding data available.*",
		}
	}

	var rows []RBACRow
	for _, b := range data.RBACBindings {
		rows = append(rows, RBACRow{
			Subject:     b.SubjectName,
			SubjectKind: b.SubjectKind,
			Role:        b.RoleName,
			RoleKind:    b.RoleKind,
			Namespace:   b.Namespace,
			Cluster:     b.Cluster,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].SubjectKind != rows[j].SubjectKind {
			return rows[i].SubjectKind < rows[j].SubjectKind
		}
		if rows[i].Subject != rows[j].Subject {
			return rows[i].Subject < rows[j].Subject
		}
		return rows[i].Role < rows[j].Role
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "rbac",
		Title:   "RBAC Inventory",
		Type:    "table",
		Content: string(tableJSON),
	}
}
