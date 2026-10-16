package controllers

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

func TestParseTTL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"valid 4h", "4h", 4 * time.Hour},
		{"valid 30m", "30m", 30 * time.Minute},
		{"valid 48h", "48h", 48 * time.Hour},
		{"invalid format fallback", "invalid", 4 * time.Hour},
		{"empty string fallback", "", 4 * time.Hour},
		{"negative duration fallback", "-1h", 4 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTTL(tt.input)
			if got != tt.expected {
				t.Errorf("ParseTTL(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestComputeTargetNamespace(t *testing.T) {
	r := &PreviewEnvironmentReconciler{}

	tests := []struct {
		name     string
		env      *idpv1alpha1.PreviewEnvironment
		expected string
	}{
		{
			name: "default prefix",
			env: &idpv1alpha1.PreviewEnvironment{
				Spec: idpv1alpha1.PreviewEnvironmentSpec{
					PRNumber: 104,
				},
			},
			expected: "preview-pr-104",
		},
		{
			name: "custom prefix",
			env: &idpv1alpha1.PreviewEnvironment{
				Spec: idpv1alpha1.PreviewEnvironmentSpec{
					NamespacePrefix: "dev-sandbox",
					PRNumber:        42,
				},
			},
			expected: "dev-sandbox-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.computeTargetNamespace(tt.env)
			if got != tt.expected {
				t.Errorf("computeTargetNamespace() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRenderHost(t *testing.T) {
	r := &PreviewEnvironmentReconciler{}

	env := &idpv1alpha1.PreviewEnvironment{
		ObjectMeta: metav1.ObjectMeta{Name: "pr-88-env"},
		Spec: idpv1alpha1.PreviewEnvironmentSpec{
			PRNumber: 88,
			Branch:   "feat-checkout",
			Ingress: idpv1alpha1.IngressSpec{
				HostTemplate: "pr-{{.PRNumber}}.test.internal",
			},
		},
	}

	host, err := r.renderHost(env)
	if err != nil {
		t.Fatalf("renderHost failed unexpectedly: %v", err)
	}

	expected := "pr-88.test.internal"
	if host != expected {
		t.Errorf("renderHost() = %q, want %q", host, expected)
	}
}

func TestConditionManagement(t *testing.T) {
	r := &PreviewEnvironmentReconciler{}
	env := &idpv1alpha1.PreviewEnvironment{}

	r.setCondition(env, "NamespaceReady", metav1.ConditionTrue, "Allocated", "Namespace created")
	if len(env.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(env.Status.Conditions))
	}

	// Update existing condition
	r.setCondition(env, "NamespaceReady", metav1.ConditionFalse, "Failed", "Quota exceeded")
	if len(env.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(env.Status.Conditions))
	}
	if env.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Errorf("expected status False, got %v", env.Status.Conditions[0].Status)
	}
}
