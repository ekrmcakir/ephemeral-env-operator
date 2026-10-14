package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

// reconcileResourceQuota enforces strict resource budgets to prevent cluster cost overrun
func (r *PreviewEnvironmentReconciler) reconcileResourceQuota(ctx context.Context, previewEnv *idpv1alpha1.PreviewEnvironment, targetNamespace string) error {
	quotaName := fmt.Sprintf("%s-quota", targetNamespace)

	cpuLimit := "2000m"
	if previewEnv.Spec.Resources.CPULimit != "" {
		cpuLimit = previewEnv.Spec.Resources.CPULimit
	}

	memLimit := "4Gi"
	if previewEnv.Spec.Resources.MemoryLimit != "" {
		memLimit = previewEnv.Spec.Resources.MemoryLimit
	}

	maxPods := int32(15)
	if previewEnv.Spec.Resources.MaxPods > 0 {
		maxPods = previewEnv.Spec.Resources.MaxPods
	}

	hardLimits := corev1.ResourceList{
		corev1.ResourceLimitsCPU:      resource.MustParse(cpuLimit),
		corev1.ResourceLimitsMemory:   resource.MustParse(memLimit),
		corev1.ResourceRequestsCPU:    resource.MustParse("500m"),
		corev1.ResourceRequestsMemory: resource.MustParse("1Gi"),
		corev1.ResourcePods:           *resource.NewQuantity(int64(maxPods), resource.DecimalSI),
	}

	quota := &corev1.ResourceQuota{}
	err := r.Get(ctx, types.NamespacedName{Namespace: targetNamespace, Name: quotaName}, quota)
	if err != nil {
		if errors.IsNotFound(err) {
			newQuota := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quotaName,
					Namespace: targetNamespace,
					Labels: map[string]string{
						ManagedByLabelKey:   ManagedByLabelVal,
						EnvironmentLabelKey: previewEnv.Name,
					},
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: hardLimits,
				},
			}
			return r.Create(ctx, newQuota)
		}
		return err
	}

	quota.Spec.Hard = hardLimits
	return r.Update(ctx, quota)
}
