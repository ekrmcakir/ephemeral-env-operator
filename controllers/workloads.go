package controllers

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

// reconcileWorkloads deploys microservices specified in Spec.Workloads
func (r *PreviewEnvironmentReconciler) reconcileWorkloads(ctx context.Context, previewEnv *idpv1alpha1.PreviewEnvironment, targetNamespace string) (int32, int32, error) {
	var totalReplicas int32 = 0
	var readyReplicas int32 = 0

	for _, w := range previewEnv.Spec.Workloads {
		replicas := int32(1)
		if w.Replicas != nil {
			replicas = *w.Replicas
		}
		totalReplicas += replicas

		// 1. Reconcile Deployment
		deploy := &appsv1.Deployment{}
		deployName := fmt.Sprintf("%s-%s", previewEnv.Name, w.Name)

		labels := map[string]string{
			ManagedByLabelKey:   ManagedByLabelVal,
			EnvironmentLabelKey: previewEnv.Name,
			"app.kubernetes.io/component": w.Name,
			PRLabelKey: fmt.Sprintf("%d", previewEnv.Spec.PRNumber),
		}

		resources := w.Resources
		if resources.Requests == nil {
			resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			}
		}
		if resources.Limits == nil {
			resources.Limits = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			}
		}

		err := r.Get(ctx, types.NamespacedName{Namespace: targetNamespace, Name: deployName}, deploy)
		if err != nil {
			if errors.IsNotFound(err) {
				newDeploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deployName,
						Namespace: targetNamespace,
						Labels:    labels,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/component": w.Name,
								EnvironmentLabelKey:         previewEnv.Name,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: labels,
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:      w.Name,
										Image:     w.Image,
										Ports:     []corev1.ContainerPort{{ContainerPort: w.Port}},
										Env:       w.Env,
										Resources: resources,
									},
								},
							},
						},
					},
				}
				if err := r.Create(ctx, newDeploy); err != nil {
					return totalReplicas, readyReplicas, err
				}
			} else {
				return totalReplicas, readyReplicas, err
			}
		} else {
			// Drift Detection & Spec Synchronization: update deployment if spec changed
			needsUpdate := false
			if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != replicas {
				deploy.Spec.Replicas = &replicas
				needsUpdate = true
			}
			if len(deploy.Spec.Template.Spec.Containers) > 0 {
				c := &deploy.Spec.Template.Spec.Containers[0]
				if c.Image != w.Image {
					c.Image = w.Image
					needsUpdate = true
				}
				if len(c.Ports) > 0 && c.Ports[0].ContainerPort != w.Port {
					c.Ports[0].ContainerPort = w.Port
					needsUpdate = true
				}
				if !reflect.DeepEqual(c.Env, w.Env) {
					c.Env = w.Env
					needsUpdate = true
				}
			}
			if needsUpdate {
				if err := r.Update(ctx, deploy); err != nil {
					return totalReplicas, readyReplicas, err
				}
			}
			readyReplicas += deploy.Status.ReadyReplicas
		}

		// 2. Reconcile Service
		svc := &corev1.Service{}
		svcName := w.Name
		err = r.Get(ctx, types.NamespacedName{Namespace: targetNamespace, Name: svcName}, svc)
		if err != nil {
			if errors.IsNotFound(err) {
				newSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcName,
						Namespace: targetNamespace,
						Labels:    labels,
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app.kubernetes.io/component": w.Name,
							EnvironmentLabelKey:         previewEnv.Name,
						},
						Ports: []corev1.ServicePort{
							{
								Name:       "http",
								Port:       w.Port,
								TargetPort: intstr.FromInt(int(w.Port)),
							},
						},
						Type: corev1.ServiceTypeClusterIP,
					},
				}
				if err := r.Create(ctx, newSvc); err != nil {
					return totalReplicas, readyReplicas, err
				}
			} else {
				return totalReplicas, readyReplicas, err
			}
		} else {
			// Service port drift detection
			if len(svc.Spec.Ports) > 0 && svc.Spec.Ports[0].Port != w.Port {
				svc.Spec.Ports[0].Port = w.Port
				svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(w.Port))
				if err := r.Update(ctx, svc); err != nil {
					return totalReplicas, readyReplicas, err
				}
			}
		}
	}

	return totalReplicas, readyReplicas, nil
}
