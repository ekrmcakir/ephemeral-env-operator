package webhooks

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

const (
	MaxAllowedTTLDuration = 72 * time.Hour
)

var branchRegex = regexp.MustCompile(`^[a-zA-Z0-9._\-/]+$`)

// PreviewEnvironmentCustomValidator validates PreviewEnvironment specifications
type PreviewEnvironmentCustomValidator struct{}

// ValidateCreate validates new PreviewEnvironment creation
func (v *PreviewEnvironmentCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	env, ok := obj.(*idpv1alpha1.PreviewEnvironment)
	if !ok {
		return nil, fmt.Errorf("expected a PreviewEnvironment object but got %T", obj)
	}

	return v.validatePreviewEnvironment(env)
}

// ValidateUpdate validates PreviewEnvironment modifications
func (v *PreviewEnvironmentCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newEnv, ok := newObj.(*idpv1alpha1.PreviewEnvironment)
	if !ok {
		return nil, fmt.Errorf("expected a PreviewEnvironment object but got %T", newObj)
	}

	oldEnv, ok := oldObj.(*idpv1alpha1.PreviewEnvironment)
	if !ok {
		return nil, fmt.Errorf("expected old object to be a PreviewEnvironment but got %T", oldObj)
	}

	var allErrs field.ErrorList

	// Immutability: PRNumber cannot be changed once provisioned
	if newEnv.Spec.PRNumber != oldEnv.Spec.PRNumber {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("prNumber"),
			"prNumber is immutable once environment is created",
		))
	}

	warnings, err := v.validatePreviewEnvironment(newEnv)
	if err != nil {
		if fieldErr, ok := err.(*field.Error); ok {
			allErrs = append(allErrs, fieldErr)
		} else {
			return warnings, err
		}
	}

	if len(allErrs) > 0 {
		return warnings, allErrs.ToAggregate()
	}

	return warnings, nil
}

// ValidateDelete validates PreviewEnvironment deletion
func (v *PreviewEnvironmentCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *PreviewEnvironmentCustomValidator) validatePreviewEnvironment(env *idpv1alpha1.PreviewEnvironment) (admission.Warnings, error) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// 1. Validate PR Number
	if env.Spec.PRNumber <= 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("prNumber"),
			env.Spec.PRNumber,
			"prNumber must be greater than 0",
		))
	}

	// 2. Validate Branch
	if env.Spec.Branch == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec").Child("branch"),
			"branch name cannot be empty",
		))
	} else if !branchRegex.MatchString(env.Spec.Branch) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("branch"),
			env.Spec.Branch,
			"branch name contains illegal characters for preview environment DNS generation",
		))
	}

	// 3. Validate TTL Limit Policy
	ttlDuration, err := time.ParseDuration(env.Spec.TTL)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("ttl"),
			env.Spec.TTL,
			"invalid duration format, must be parsable by time.ParseDuration (e.g. '2h', '24h')",
		))
	} else {
		if ttlDuration <= 0 {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("ttl"),
				env.Spec.TTL,
				"ttl must be a positive duration",
			))
		}
		if ttlDuration > MaxAllowedTTLDuration {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec").Child("ttl"),
				fmt.Sprintf("ttl exceeds max company policy limit of %s", MaxAllowedTTLDuration),
			))
		}
		if ttlDuration > 48*time.Hour {
			warnings = append(warnings, fmt.Sprintf("High TTL specified (%s). Ephemeral environments should ideally terminate within 24h to optimize cluster costs.", env.Spec.TTL))
		}
	}

	// 4. Validate Workloads
	if len(env.Spec.Workloads) == 0 {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec").Child("workloads"),
			"at least one workload component must be declared",
		))
	}

	for i, w := range env.Spec.Workloads {
		wPath := field.NewPath("spec").Child("workloads").Index(i)
		if w.Name == "" {
			allErrs = append(allErrs, field.Required(wPath.Child("name"), "workload name is required"))
		}
		if w.Image == "" {
			allErrs = append(allErrs, field.Required(wPath.Child("image"), "container image is required"))
		}
		if w.Port <= 0 || w.Port > 65535 {
			allErrs = append(allErrs, field.Invalid(wPath.Child("port"), w.Port, "port must be between 1 and 65535"))
		}
	}

	if len(allErrs) > 0 {
		return warnings, allErrs.ToAggregate()
	}

	return warnings, nil
}
