package store

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildTree(t *testing.T) {
	t.Run("two L1 with children", func(t *testing.T) {
		parentA := uuid.New()
		parentB := uuid.New()
		childA1 := uuid.New()
		childA2 := uuid.New()
		childB1 := uuid.New()

		nodes := []CapabilityTreeNode{
			{BusinessCapability: BusinessCapability{ID: parentA, Name: "Security", Level: 1}, Children: []CapabilityTreeNode{}, AppCount: 0},
			{BusinessCapability: BusinessCapability{ID: parentB, Name: "Monitoring", Level: 1}, Children: []CapabilityTreeNode{}, AppCount: 0},
			{BusinessCapability: BusinessCapability{ID: childA1, Name: "Authentication", ParentID: &parentA, Level: 2}, Children: []CapabilityTreeNode{}, AppCount: 3},
			{BusinessCapability: BusinessCapability{ID: childA2, Name: "Authorization", ParentID: &parentA, Level: 2}, Children: []CapabilityTreeNode{}, AppCount: 1},
			{BusinessCapability: BusinessCapability{ID: childB1, Name: "Logging", ParentID: &parentB, Level: 2}, Children: []CapabilityTreeNode{}, AppCount: 2},
		}

		roots := buildTree(nodes)

		if len(roots) != 2 {
			t.Fatalf("expected 2 roots, got %d", len(roots))
		}

		// Find Security root
		var security, monitoring *CapabilityTreeNode
		for i := range roots {
			if roots[i].Name == "Security" {
				security = &roots[i]
			}
			if roots[i].Name == "Monitoring" {
				monitoring = &roots[i]
			}
		}

		if security == nil {
			t.Fatal("Security root not found")
		}
		if len(security.Children) != 2 {
			t.Errorf("Security children = %d, want 2", len(security.Children))
		}

		if monitoring == nil {
			t.Fatal("Monitoring root not found")
		}
		if len(monitoring.Children) != 1 {
			t.Errorf("Monitoring children = %d, want 1", len(monitoring.Children))
		}
	})

	t.Run("orphaned child becomes root", func(t *testing.T) {
		orphanParent := uuid.New() // does not exist in nodes
		childID := uuid.New()

		nodes := []CapabilityTreeNode{
			{BusinessCapability: BusinessCapability{ID: childID, Name: "Orphan", ParentID: &orphanParent, Level: 2}, Children: []CapabilityTreeNode{}},
		}

		roots := buildTree(nodes)
		if len(roots) != 1 {
			t.Fatalf("expected 1 root, got %d", len(roots))
		}
		if roots[0].Name != "Orphan" {
			t.Errorf("root name = %q, want %q", roots[0].Name, "Orphan")
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		roots := buildTree(nil)
		if roots != nil {
			t.Errorf("expected nil, got %v", roots)
		}
	})

	t.Run("all nil parents are all roots", func(t *testing.T) {
		nodes := []CapabilityTreeNode{
			{BusinessCapability: BusinessCapability{ID: uuid.New(), Name: "A"}, Children: []CapabilityTreeNode{}},
			{BusinessCapability: BusinessCapability{ID: uuid.New(), Name: "B"}, Children: []CapabilityTreeNode{}},
			{BusinessCapability: BusinessCapability{ID: uuid.New(), Name: "C"}, Children: []CapabilityTreeNode{}},
		}

		roots := buildTree(nodes)
		if len(roots) != 3 {
			t.Errorf("expected 3 roots, got %d", len(roots))
		}
	})
}
