package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

// reconcileNamespace ensures dedicated isolated namespace exists with proper governance labels
func (r *PreviewEnvironmentReconciler) reconcileNamespace(ctx context.Context, previewEnv *idpv1alpha1.PreviewEnvironment, targetNamespace string) error {
	logger := log.FromContext(ctx)

	ns := &corev1.Namespace{}
	err := r.Get(ctx, clientObjectKey("", targetNamespace), ns)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Provisioning new preview namespace", "namespace", targetNamespace)
			newNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: targetNamespace,
					Labels: map[string]string{
						ManagedByLabelKey:   ManagedByLabelVal,
						EnvironmentLabelKey: previewEnv.Name,
						PRLabelKey:          fmt.Sprintf("%d", previewEnv.Spec.PRNumber),
						"pod-security.kubernetes.io/enforce": "baseline",
						"pod-security.kubernetes.io/audit":   "restricted",
						"pod-security.kubernetes.io/warn":    "restricted",
					},
					Annotations: map[string]string{
						"idp.cloudnative.io/branch":     previewEnv.Spec.Branch,
						"idp.cloudnative.io/repository": previewEnv.Spec.Repository,
						"idp.cloudnative.io/author":     previewEnv.Spec.Author,
						"idp.cloudnative.io/ttl":        previewEnv.Spec.TTL,
					},
				},
			}
			return r.Create(ctx, newNs)
		}
		return err
	}

	// Update metadata annotations if changed
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[ManagedByLabelKey] = ManagedByLabelVal
	ns.Labels[EnvironmentLabelKey] = previewEnv.Name
	ns.Labels[PRLabelKey] = fmt.Sprintf("%d", previewEnv.Spec.PRNumber)

	return r.Update(ctx, ns)
}

// cleanupNamespace safely terminates the preview namespace
func (r *PreviewEnvironmentReconciler) cleanupNamespace(ctx context.Context, targetNamespace string) error {
	logger := log.FromContext(ctx)
	ns := &corev1.Namespace{}
	err := r.Get(ctx, clientObjectKey("", targetNamespace), ns)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	logger.Info("Deleting preview namespace", "namespace", targetNamespace)
	return r.Delete(ctx, ns)
}

func clientObjectKey(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}
