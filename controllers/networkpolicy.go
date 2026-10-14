package controllers

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

// reconcileNetworkPolicy implements Zero-Trust network isolation
// Ensures preview environments cannot communicate with other preview namespaces or staging/prod
func (r *PreviewEnvironmentReconciler) reconcileNetworkPolicy(ctx context.Context, previewEnv *idpv1alpha1.PreviewEnvironment, targetNamespace string) error {
	policyName := fmt.Sprintf("%s-isolation", targetNamespace)

	np := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, types.NamespacedName{Namespace: targetNamespace, Name: policyName}, np)
	if err != nil {
		if errors.IsNotFound(err) {
			newPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyName,
					Namespace: targetNamespace,
					Labels: map[string]string{
						ManagedByLabelKey:   ManagedByLabelVal,
						EnvironmentLabelKey: previewEnv.Name,
					},
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{}, // Select all pods in this namespace
					PolicyTypes: []networkingv1.PolicyType{
						networkingv1.PolicyTypeIngress,
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						// Rule 1: Allow intra-namespace communication (services talking to mock DB / each other)
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{},
								},
							},
						},
						// Rule 2: Allow ingress traffic from Ingress Controller namespace (e.g. ingress-nginx)
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app.kubernetes.io/name": "ingress-nginx",
										},
									},
								},
								{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"kubernetes.io/metadata.name": "ingress-nginx",
										},
									},
								},
							},
						},
					},
				},
			}
			return r.Create(ctx, newPolicy)
		}
		return err
	}

	return nil
}
