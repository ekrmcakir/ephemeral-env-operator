package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreviewEnvironmentPhase represents the operational phase of a preview environment
type PreviewEnvironmentPhase string

const (
	PhasePending      PreviewEnvironmentPhase = "Pending"
	PhaseProvisioning PreviewEnvironmentPhase = "Provisioning"
	PhaseReady        PreviewEnvironmentPhase = "Ready"
	PhaseExpired      PreviewEnvironmentPhase = "Expired"
	PhaseTerminating  PreviewEnvironmentPhase = "Terminating"
	PhaseFailed       PreviewEnvironmentPhase = "Failed"
)

// DatabaseEngine defines supported mock database engines
type DatabaseEngine string

const (
	DatabaseEnginePostgres DatabaseEngine = "postgres"
	DatabaseEngineRedis    DatabaseEngine = "redis"
	DatabaseEngineMongoDB  DatabaseEngine = "mongodb"
	DatabaseEngineMySQL    DatabaseEngine = "mysql"
)

// WorkloadSpec defines a containerized microservice running in the preview environment
type WorkloadSpec struct {
	// Name of the microservice component
	Name string `json:"name"`

	// Container image to deploy
	Image string `json:"image"`

	// Target port exposed by container
	Port int32 `json:"port"`

	// Number of replicas
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Environment variables injected into container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resource requests and limits for the workload
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// IngressSpec defines the external routing configuration for preview environments
type IngressSpec struct {
	// Enabled toggles ingress creation
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// HostTemplate is used to compute the FQDN. e.g. "pr-{{ .PRNumber }}.preview.example.com"
	// +kubebuilder:default="pr-{{.PRNumber}}.preview.cloudnative.io"
	HostTemplate string `json:"hostTemplate,omitempty"`

	// IngressClassName specifies the ingress controller to use (e.g. "nginx", "traefik")
	// +optional
	// +kubebuilder:default="nginx"
	IngressClassName string `json:"ingressClassName,omitempty"`

	// Path defines the ingress path prefix
	// +optional
	// +kubebuilder:default="/"
	Path string `json:"path,omitempty"`

	// TLS specifies whether to configure TLS termination with cert-manager
	// +optional
	// +kubebuilder:default=false
	TLS bool `json:"tls,omitempty"`
}

// ResourceQuotaSpec defines resource bounds allocated for the ephemeral namespace
type ResourceQuotaSpec struct {
	// CPU limit for the entire preview namespace (e.g. "2000m")
	// +optional
	// +kubebuilder:default="2000m"
	CPULimit string `json:"cpuLimit,omitempty"`

	// Memory limit for the entire preview namespace (e.g. "4Gi")
	// +optional
	// +kubebuilder:default="4Gi"
	MemoryLimit string `json:"memoryLimit,omitempty"`

	// Max Pods allowed in the preview environment
	// +optional
	// +kubebuilder:default=15
	MaxPods int32 `json:"maxPods,omitempty"`
}

// MockDatabaseSpec defines ephemeral stateful services spun up for the environment
type MockDatabaseSpec struct {
	// Engine type for mock database (postgres, redis, mongodb, mysql)
	Engine DatabaseEngine `json:"engine"`

	// Database name to automatically create
	// +optional
	// +kubebuilder:default="preview_db"
	DatabaseName string `json:"databaseName,omitempty"`

	// Ephemeral indicates storage will be lost on container restart (emptyDir)
	// +optional
	// +kubebuilder:default=true
	Ephemeral bool `json:"ephemeral,omitempty"`

	// SeedDataConfigMap optional ConfigMap containing initial SQL / Seed scripts
	// +optional
	SeedDataConfigMap string `json:"seedDataConfigMap,omitempty"`
}

// PreviewEnvironmentSpec defines the desired state of PreviewEnvironment
type PreviewEnvironmentSpec struct {
	// Git branch name triggering this preview environment
	Branch string `json:"branch"`

	// Pull Request / Merge Request number
	// +kubebuilder:validation:Minimum=1
	PRNumber int32 `json:"prNumber"`

	// Repository slug, e.g. "org/payment-service"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Author / Committer username
	// +optional
	Author string `json:"author,omitempty"`

	// TTL specifies the lifespan before automatic garbage collection (e.g. "2h", "24h", "72h")
	// +kubebuilder:default="4h"
	TTL string `json:"ttl"`

	// AutoCleanup triggers automatic namespace termination when TTL expires
	// +optional
	// +kubebuilder:default=true
	AutoCleanup *bool `json:"autoCleanup,omitempty"`

	// NamespacePrefix custom prefix for created namespace (defaults to "preview-pr")
	// +optional
	// +kubebuilder:default="preview-pr"
	NamespacePrefix string `json:"namespacePrefix,omitempty"`

	// Workloads list of application services to deploy
	Workloads []WorkloadSpec `json:"workloads"`

	// Ingress routing configuration
	// +optional
	Ingress IngressSpec `json:"ingress,omitempty"`

	// Resources quota constraints for developer cost control
	// +optional
	Resources ResourceQuotaSpec `json:"resources,omitempty"`

	// MockDatabase optional ephemeral mock database
	// +optional
	MockDatabase *MockDatabaseSpec `json:"mockDatabase,omitempty"`

	// IsolatedNetwork configures Zero-Trust NetworkPolicies between preview environments
	// +optional
	// +kubebuilder:default=true
	IsolatedNetwork *bool `json:"isolatedNetwork,omitempty"`
}

// EndpointStatus exposes active preview environment URL
type EndpointStatus struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Port int32  `json:"port"`
}

// PreviewEnvironmentStatus defines the observed state of PreviewEnvironment
type PreviewEnvironmentStatus struct {
	// Phase is the current lifecycle state
	Phase PreviewEnvironmentPhase `json:"phase,omitempty"`

	// TargetNamespace is the actual Kubernetes namespace allocated
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// Endpoints list of reachable ingress/service URLs
	// +optional
	Endpoints []EndpointStatus `json:"endpoints,omitempty"`

	// CreatedAt records when the environment was initially created
	// +optional
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// ExpiresAt computed timestamp when the environment will be garbage collected
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// TotalReplicas expected workload replicas
	// +optional
	TotalReplicas int32 `json:"totalReplicas,omitempty"`

	// ReadyReplicas healthy running replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Conditions details current status of sub-resources
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration matches metadata.generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PR",type="integer",JSONPath=".spec.prNumber",description="Pull Request Number"
// +kubebuilder:printcolumn:name="Branch",type="string",JSONPath=".spec.branch",description="Git Branch"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Lifecycle Phase"
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".status.targetNamespace",description="Allocated Namespace"
// +kubebuilder:printcolumn:name="TTL",type="string",JSONPath=".spec.ttl",description="Configured TTL"
// +kubebuilder:printcolumn:name="ExpiresAt",type="string",JSONPath=".status.expiresAt",description="TTL Expiry Date"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PreviewEnvironment is the Schema for the previewenvironments API
type PreviewEnvironment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PreviewEnvironmentSpec   `json:"spec,omitempty"`
	Status PreviewEnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PreviewEnvironmentList contains a list of PreviewEnvironment
type PreviewEnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreviewEnvironment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PreviewEnvironment{}, &PreviewEnvironmentList{})
}
