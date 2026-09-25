package parser

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func secretsListFails(err error) *KubernetesParser {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	return &KubernetesParser{typed: cs, clusterName: "test"}
}

// Secret reading is opt-in in the chart: a Forbidden list means the feature
// is off, so the parse must not be marked partial.
func TestParseConfigsSecretsForbiddenIsNotPartial(t *testing.T) {
	p := secretsListFails(apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "", errors.New("rbac")))
	p.parseConfigs(context.Background())
	if got := p.failed.Load(); got != 0 {
		t.Fatalf("failed = %d, want 0", got)
	}
}

func TestParseConfigsSecretsOtherErrorIsPartial(t *testing.T) {
	p := secretsListFails(errors.New("connection refused"))
	p.parseConfigs(context.Background())
	if got := p.failed.Load(); got != 1 {
		t.Fatalf("failed = %d, want 1", got)
	}
}
