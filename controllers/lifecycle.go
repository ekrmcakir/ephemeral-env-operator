package controllers

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	idpv1alpha1 "github.com/ekrmcakir/ephemeral-env-operator/api/v1alpha1"
)

// TTLGarbageCollector periodically scans and terminates stale environments
type TTLGarbageCollector struct {
	Client   client.Client
	Interval time.Duration
}

// Start runs the garbage collection worker until context cancellation
func (gc *TTLGarbageCollector) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("ttl-garbage-collector")
	logger.Info("Starting TTL Garbage Collection worker", "interval", gc.Interval)

	ticker := time.NewTicker(gc.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping TTL Garbage Collection worker")
			return nil
		case <-ticker.C:
			if err := gc.reapExpiredEnvironments(ctx); err != nil {
				logger.Error(err, "Error occurred during TTL garbage collection sweep")
			}
		}
	}
}

// reapExpiredEnvironments finds expired environments and marks them for cleanup
func (gc *TTLGarbageCollector) reapExpiredEnvironments(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("ttl-garbage-collector")

	envList := &idpv1alpha1.PreviewEnvironmentList{}
	if err := gc.Client.List(ctx, envList); err != nil {
		return fmt.Errorf("failed to list preview environments: %w", err)
	}

	now := time.Now()
	for _, env := range envList.Items {
		if env.Status.Phase == idpv1alpha1.PhaseExpired || env.Status.Phase == idpv1alpha1.PhaseTerminating {
			continue
		}

		if env.Status.ExpiresAt != nil && now.After(env.Status.ExpiresAt.Time) {
			logger.Info("TTL exceeded. Marking environment as Expired", "name", env.Name, "pr", env.Spec.PRNumber)
			envCopy := env.DeepCopy()
			envCopy.Status.Phase = idpv1alpha1.PhaseExpired
			if envCopy.Status.Conditions == nil {
				envCopy.Status.Conditions = []metav1.Condition{}
			}
			envCopy.Status.Conditions = append(envCopy.Status.Conditions, metav1.Condition{
				Type:               "TTLExpired",
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "TTLExceeded",
				Message:            fmt.Sprintf("Environment lifespan reached configured TTL limit (%s)", env.Spec.TTL),
			})

			if err := gc.Client.Status().Update(ctx, envCopy); err != nil {
				logger.Error(err, "Failed to update status for expired environment", "name", env.Name)
			}
		}
	}

	return nil
}

// ParseTTL parses string representation of duration with fallback
func ParseTTL(ttlStr string) time.Duration {
	d, err := time.ParseDuration(ttlStr)
	if err != nil || d <= 0 {
		return 4 * time.Hour
	}
	return d
}
