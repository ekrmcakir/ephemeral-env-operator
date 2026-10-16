package webhooks

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

func TestValidatePreviewEnvironment_Valid(t *testing.T) {
	v := &PreviewEnvironmentCustomValidator{}

	env := &idpv1alpha1.PreviewEnvironment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-valid"},
		Spec: idpv1alpha1.PreviewEnvironmentSpec{
			PRNumber: 12,
			Branch:   "feature/order-api",
			TTL:      "6h",
			Workloads: []idpv1alpha1.WorkloadSpec{
				{
					Name:  "web",
					Image: "nginx:alpine",
					Port:  80,
				},
			},
		},
	}

	warnings, err := v.ValidateCreate(context.Background(), env)
	if err != nil {
		t.Fatalf("expected valid spec to pass, got err: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestValidatePreviewEnvironment_InvalidPR(t *testing.T) {
	v := &PreviewEnvironmentCustomValidator{}

	env := &idpv1alpha1.PreviewEnvironment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-invalid-pr"},
		Spec: idpv1alpha1.PreviewEnvironmentSpec{
			PRNumber: 0,
			Branch:   "fix/bug",
			TTL:      "2h",
			Workloads: []idpv1alpha1.WorkloadSpec{
				{Name: "api", Image: "app:v1", Port: 8080},
			},
		},
	}

	_, err := v.ValidateCreate(context.Background(), env)
	if err == nil {
		t.Fatal("expected error for PRNumber=0, got nil")
	}
}

func TestValidatePreviewEnvironment_TTLPolicyViolation(t *testing.T) {
	v := &PreviewEnvironmentCustomValidator{}

	env := &idpv1alpha1.PreviewEnvironment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ttl-violation"},
		Spec: idpv1alpha1.PreviewEnvironmentSpec{
			PRNumber: 15,
			Branch:   "feature/big-test",
			TTL:      "120h", // Exceeds 72h policy limit
			Workloads: []idpv1alpha1.WorkloadSpec{
				{Name: "api", Image: "app:v1", Port: 8080},
			},
		},
	}

	_, err := v.ValidateCreate(context.Background(), env)
	if err == nil {
		t.Fatal("expected error for TTL exceeding 72h limit, got nil")
	}
}

func TestValidatePreviewEnvironment_ImmutablePR(t *testing.T) {
	v := &PreviewEnvironmentCustomValidator{}

	oldEnv := &idpv1alpha1.PreviewEnvironment{
		Spec: idpv1alpha1.PreviewEnvironmentSpec{
			PRNumber: 10,
			Branch:   "main",
			TTL:      "4h",
			Workloads: []idpv1alpha1.WorkloadSpec{
				{Name: "api", Image: "app:v1", Port: 8080},
			},
		},
	}

	newEnv := oldEnv.DeepCopy()
	newEnv.Spec.PRNumber = 11 // Attempt to mutate PRNumber

	_, err := v.ValidateUpdate(context.Background(), oldEnv, newEnv)
	if err == nil {
		t.Fatal("expected error when attempting to mutate PRNumber, got nil")
	}
}
