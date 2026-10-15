package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

// reconcileMockDatabase deploys an ephemeral lightweight database in the preview namespace
func (r *PreviewEnvironmentReconciler) reconcileMockDatabase(ctx context.Context, previewEnv *idpv1alpha1.PreviewEnvironment, targetNamespace string) error {
	dbSpec := previewEnv.Spec.MockDatabase
	if dbSpec == nil {
		return nil
	}

	dbName := fmt.Sprintf("%s-mockdb", previewEnv.Name)
	labels := map[string]string{
		ManagedByLabelKey:   ManagedByLabelVal,
		EnvironmentLabelKey: previewEnv.Name,
		"app.kubernetes.io/component": "database",
	}

	image, port, envVars := r.getDatabaseConfig(dbSpec)

	// 1. Reconcile Deployment
	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Namespace: targetNamespace, Name: dbName}, deploy)
	if err != nil {
		if errors.IsNotFound(err) {
			replicas := int32(1)
			newDeploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dbName,
					Namespace: targetNamespace,
					Labels:    labels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/component": "database",
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
									Name:  "database",
									Image: image,
									Ports: []corev1.ContainerPort{{ContainerPort: port}},
									Env:   envVars,
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("500m"),
											corev1.ResourceMemory: resource.MustParse("512Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("256Mi"),
										},
									},
								},
							},
						},
					},
				},
			}
			if err := r.Create(ctx, newDeploy); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// 2. Reconcile Service
	svc := &corev1.Service{}
	svcName := "database"
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
						"app.kubernetes.io/component": "database",
						EnvironmentLabelKey:         previewEnv.Name,
					},
					Ports: []corev1.ServicePort{
						{
							Name:       "db",
							Port:       port,
							TargetPort: intstr.FromInt(int(port)),
						},
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			}
			return r.Create(ctx, newSvc)
		}
		return err
	}

	return nil
}

func (r *PreviewEnvironmentReconciler) getDatabaseConfig(dbSpec *idpv1alpha1.MockDatabaseSpec) (string, int32, []corev1.EnvVar) {
	dbName := "preview_db"
	if dbSpec.DatabaseName != "" {
		dbName = dbSpec.DatabaseName
	}

	switch dbSpec.Engine {
	case idpv1alpha1.DatabaseEngineRedis:
		return "redis:7-alpine", 6379, nil
	case idpv1alpha1.DatabaseEngineMongoDB:
		return "mongo:7.0", 27017, []corev1.EnvVar{
			{Name: "MONGO_INITDB_DATABASE", Value: dbName},
		}
	case idpv1alpha1.DatabaseEngineMySQL:
		return "mysql:8.0", 3306, []corev1.EnvVar{
			{Name: "MYSQL_ROOT_PASSWORD", Value: "previewsecret"},
			{Name: "MYSQL_DATABASE", Value: dbName},
			{Name: "MYSQL_USER", Value: "preview"},
			{Name: "MYSQL_PASSWORD", Value: "previewpass"},
		}
	case idpv1alpha1.DatabaseEnginePostgres:
		fallthrough
	default:
		return "postgres:16-alpine", 5432, []corev1.EnvVar{
			{Name: "POSTGRES_DB", Value: dbName},
			{Name: "POSTGRES_USER", Value: "preview"},
			{Name: "POSTGRES_PASSWORD", Value: "previewpass"},
		}
	}
}
