package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

const (
	PreviewEnvironmentFinalizer = "idp.cloudnative.io/finalizer"
	ManagedByLabelKey           = "app.kubernetes.io/managed-by"
	ManagedByLabelVal           = "ephemeral-env-operator"
	EnvironmentLabelKey         = "idp.cloudnative.io/preview-env"
	PRLabelKey                  = "idp.cloudnative.io/pr-number"
)

// PreviewEnvironmentReconciler reconciles a PreviewEnvironment object
type PreviewEnvironmentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=idp.cloudnative.io,resources=previewenvironments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=idp.cloudnative.io,resources=previewenvironments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=idp.cloudnative.io,resources=previewenvironments/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces;services;configmaps;secrets;resourcequotas;limitranges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *PreviewEnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("previewenvironment", req.NamespacedName)

	// 1. Fetch the PreviewEnvironment instance
	previewEnv := &idpv1alpha1.PreviewEnvironment{}
	err := r.Get(ctx, req.NamespacedName, previewEnv)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PreviewEnvironment resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PreviewEnvironment")
		return ctrl.Result{}, err
	}

	targetNamespace := r.computeTargetNamespace(previewEnv)

	// 2. Handle deletion and finalizer cleanup
	if !previewEnv.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, previewEnv, targetNamespace)
	}

	// 3. Register finalizer if not present
	if !controllerutil.ContainsFinalizer(previewEnv, PreviewEnvironmentFinalizer) {
		controllerutil.AddFinalizer(previewEnv, PreviewEnvironmentFinalizer)
		if err := r.Update(ctx, previewEnv); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Initialize Status if needed
	if previewEnv.Status.Phase == "" {
		previewEnv.Status.Phase = idpv1alpha1.PhasePending
		previewEnv.Status.TargetNamespace = targetNamespace
		now := metav1.Now()
		previewEnv.Status.CreatedAt = &now

		ttlDuration, err := time.ParseDuration(previewEnv.Spec.TTL)
		if err != nil {
			ttlDuration = 4 * time.Hour
		}
		expiresAt := metav1.NewTime(now.Add(ttlDuration))
		previewEnv.Status.ExpiresAt = &expiresAt

		if err := r.Status().Update(ctx, previewEnv); err != nil {
			logger.Error(err, "Failed to initialize PreviewEnvironment status")
			return ctrl.Result{}, err
		}
		RecordEnvironmentCreated(previewEnv.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	// 5. TTL Expiry Check
	now := time.Now()
	if previewEnv.Status.ExpiresAt != nil && now.After(previewEnv.Status.ExpiresAt.Time) {
		logger.Info("Preview environment TTL has expired", "pr", previewEnv.Spec.PRNumber, "expiresAt", previewEnv.Status.ExpiresAt.Time)
		previewEnv.Status.Phase = idpv1alpha1.PhaseExpired
		r.setCondition(previewEnv, "TTLExpired", metav1.ConditionTrue, "TTLExceeded", "Configured environment TTL duration reached")

		autoCleanup := true
		if previewEnv.Spec.AutoCleanup != nil {
			autoCleanup = *previewEnv.Spec.AutoCleanup
		}

		if autoCleanup {
			logger.Info("AutoCleanup enabled: Terminating target namespace", "namespace", targetNamespace)
			if err := r.cleanupNamespace(ctx, targetNamespace); err != nil {
				logger.Error(err, "Failed to cleanup expired namespace", "namespace", targetNamespace)
			}
			previewEnv.Status.Phase = idpv1alpha1.PhaseTerminating
		}

		if err := r.Status().Update(ctx, previewEnv); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 6. Reconciliation sequence
	previewEnv.Status.Phase = idpv1alpha1.PhaseProvisioning

	// 6.1 Reconcile Target Namespace
	if err := r.reconcileNamespace(ctx, previewEnv, targetNamespace); err != nil {
		r.setCondition(previewEnv, "NamespaceReady", metav1.ConditionFalse, "ProvisioningFailed", err.Error())
		_ = r.Status().Update(ctx, previewEnv)
		return ctrl.Result{}, err
	}
	r.setCondition(previewEnv, "NamespaceReady", metav1.ConditionTrue, "Provisioned", "Target namespace allocated successfully")

	// 6.2 Reconcile ResourceQuota and LimitRange
	if err := r.reconcileResourceQuota(ctx, previewEnv, targetNamespace); err != nil {
		logger.Error(err, "Failed to apply resource quota", "namespace", targetNamespace)
	}

	// 6.3 Reconcile Zero-Trust NetworkPolicy
	if previewEnv.Spec.IsolatedNetwork == nil || *previewEnv.Spec.IsolatedNetwork {
		if err := r.reconcileNetworkPolicy(ctx, previewEnv, targetNamespace); err != nil {
			logger.Error(err, "Failed to configure network isolation policy", "namespace", targetNamespace)
		}
	}

	// 6.4 Reconcile Ephemeral Mock Database
	if previewEnv.Spec.MockDatabase != nil {
		if err := r.reconcileMockDatabase(ctx, previewEnv, targetNamespace); err != nil {
			r.setCondition(previewEnv, "DatabaseReady", metav1.ConditionFalse, "DBProvisionFailed", err.Error())
			_ = r.Status().Update(ctx, previewEnv)
			return ctrl.Result{}, err
		}
		r.setCondition(previewEnv, "DatabaseReady", metav1.ConditionTrue, "DBProvisioned", "Ephemeral mock database ready")
	}

	// 6.5 Reconcile Application Workloads
	totalReplicas, readyReplicas, err := r.reconcileWorkloads(ctx, previewEnv, targetNamespace)
	if err != nil {
		r.setCondition(previewEnv, "WorkloadsReady", metav1.ConditionFalse, "WorkloadDeployFailed", err.Error())
		_ = r.Status().Update(ctx, previewEnv)
		return ctrl.Result{}, err
	}
	previewEnv.Status.TotalReplicas = totalReplicas
	previewEnv.Status.ReadyReplicas = readyReplicas

	// 6.6 Reconcile Ingress Routing & DNS
	endpoints, err := r.reconcileIngress(ctx, previewEnv, targetNamespace)
	if err != nil {
		r.setCondition(previewEnv, "IngressReady", metav1.ConditionFalse, "IngressRouteFailed", err.Error())
		_ = r.Status().Update(ctx, previewEnv)
		return ctrl.Result{}, err
	}
	previewEnv.Status.Endpoints = endpoints
	r.setCondition(previewEnv, "IngressReady", metav1.ConditionTrue, "RoutesActive", "Ingress endpoints configured and live")

	// 7. Update status to Ready if all workloads are ready
	if totalReplicas > 0 && readyReplicas == totalReplicas {
		previewEnv.Status.Phase = idpv1alpha1.PhaseReady
		r.setCondition(previewEnv, "Ready", metav1.ConditionTrue, "AllSystemsOperational", "Ephemeral environment is live and reachable")
	} else {
		previewEnv.Status.Phase = idpv1alpha1.PhaseProvisioning
		r.setCondition(previewEnv, "Ready", metav1.ConditionFalse, "WorkloadsPending", fmt.Sprintf("%d/%d replicas ready", readyReplicas, totalReplicas))
	}

	if err := r.Status().Update(ctx, previewEnv); err != nil {
		return ctrl.Result{}, err
	}

	// 8. Requeue before TTL expires or periodic heartbeat
	requeueDuration := 1 * time.Minute
	if previewEnv.Status.ExpiresAt != nil {
		remaining := time.Until(previewEnv.Status.ExpiresAt.Time)
		if remaining > 0 && remaining < requeueDuration {
			requeueDuration = remaining
		}
	}

	return ctrl.Result{RequeueAfter: requeueDuration}, nil
}

// computeTargetNamespace generates sanitized unique namespace name for the PR preview
func (r *PreviewEnvironmentReconciler) computeTargetNamespace(previewEnv *idpv1alpha1.PreviewEnvironment) string {
	prefix := previewEnv.Spec.NamespacePrefix
	if prefix == "" {
		prefix = "preview-pr"
	}
	return fmt.Sprintf("%s-%d", prefix, previewEnv.Spec.PRNumber)
}

// handleDeletion cleans up resources and releases finalizer
func (r *PreviewEnvironmentReconciler) handleDeletion(ctx context.Context, previewEnv *idpv1alpha1.PreviewEnvironment, targetNamespace string) (ctrl.Result, error) {
	log.FromContext(ctx).Info("Handling graceful teardown of preview environment", "namespace", targetNamespace)

	// Clean up namespace
	_ = r.cleanupNamespace(ctx, targetNamespace)

	RecordEnvironmentDestroyed(previewEnv.Name)

	controllerutil.RemoveFinalizer(previewEnv, PreviewEnvironmentFinalizer)
	if err := r.Update(ctx, previewEnv); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// setCondition updates or appends a condition
func (r *PreviewEnvironmentReconciler) setCondition(previewEnv *idpv1alpha1.PreviewEnvironment, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	for i, c := range previewEnv.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				previewEnv.Status.Conditions[i].Status = status
				previewEnv.Status.Conditions[i].LastTransitionTime = now
			}
			previewEnv.Status.Conditions[i].Reason = reason
			previewEnv.Status.Conditions[i].Message = message
			return
		}
	}
	previewEnv.Status.Conditions = append(previewEnv.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// SetupWithManager sets up the controller with the Manager and watches sub-resources
func (r *PreviewEnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Map child resource events back to the owning PreviewEnvironment CR
	mapFn := func(ctx context.Context, obj client.Object) []reconcile.Request {
		labels := obj.GetLabels()
		if envName, ok := labels[EnvironmentLabelKey]; ok && envName != "" {
			ns := obj.GetNamespace()
			// Check if annotation contains the origin namespace of the CR
			if originNs, hasNs := obj.GetAnnotations()["idp.cloudnative.io/owner-namespace"]; hasNs && originNs != "" {
				ns = originNs
			}
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: ns, Name: envName}},
			}
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&idpv1alpha1.PreviewEnvironment{}).
		Owns(&corev1.Namespace{}).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFn)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFn)).
		Watches(&networkingv1.Ingress{}, handler.EnqueueRequestsFromMapFunc(mapFn)).
		Complete(r)
}
