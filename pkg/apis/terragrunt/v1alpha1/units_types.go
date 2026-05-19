package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvVar represents a Kubernetes environment variable, supporting secret references.
// +kubebuilder:object:generate=true
type EnvVar struct {
	// Name of the environment variable.
	// +required
	Name string `json:"name"`

	// Value of the environment variable.
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom is the source of the environment variable's value.
	// +optional
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource represents the source of an environment variable's value.
type EnvVarSource struct {
	// SecretKeyRef selects a key from a secret in the pod's namespace.
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`

	// ConfigMapKeyRef selects a key from a configmap in the pod's namespace.
	// +optional
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
}

// TfVarEntry represents a Terraform/OpenTofu variable passed as an environment variable.
// Values are automatically prefixed with TF_VAR_ when injected into the pod.
// +kubebuilder:object:generate=true
type TfVarEntry struct {
	// Name is the name of the Terraform variable (without TF_VAR_ prefix).
	// +required
	Name string `json:"name"`

	// Value is a static value for the variable.
	// +optional
	Value string `json:"value,omitempty"`

	// SecretRef references a Kubernetes secret and key for the variable value.
	// When using this, the value will not appear in logs, kubectl describe, etc.
	// +optional
	SecretRef *TfVarSecretRef `json:"secretRef,omitempty"`
}

// TfVarSecretRef references a key in a Kubernetes secret.
type TfVarSecretRef struct {
	// Name of the secret.
	// +required
	Name string `json:"name"`

	// Key in the secret.
	// +required
	Key string `json:"key"`
}

// UnitsSpec defines the spec for a Units resource.
type UnitsSpec struct {
	// Filters are the paths within the GitRepository to monitor for changes.
	// +required
	Filters []string `json:"filters"`

	// Branch is the Git branch to monitor.
	// +required
	Branch string `json:"branch"`

	// Parallelism is the number of concurrent executions allowed.
	// +optional
	// +default=1
	Parallelism int `json:"parallelism,omitempty"`

	// Env is a list of environment variables to inject into the runner pod.
	// Supports both static values and references to secrets/configmaps.
	// +optional
	Env []EnvVar `json:"env,omitempty"`

	// TfVars is a list of Terraform/OpenTofu variables to inject as environment
	// variables. Each variable is injected as TF_VAR_<name>. Values can be static
	// or referenced from secrets for security.
	// +optional
	TfVars []TfVarEntry `json:"tf-vars,omitempty"`
}

// UnitsStatus defines the observed state of a Units resource.
type UnitsStatus struct {
	// LastHandledReconcileAt tracks the last reconciliation timestamp.
	// +optional
	LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`

	// LastCommitSHA is the SHA of the last detected change.
	// +optional
	LastCommitSHA string `json:"lastCommitSHA,omitempty"`

	// LastRunnerJob is the name of the last runner job spawned.
	// +optional
	LastRunnerJob string `json:"lastRunnerJob,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=units,shortName=unit
// +genclient
// +genclient:nonNamespaced

// Units is the Schema for the units API.
type Units struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnitsSpec   `json:"spec,omitempty"`
	Status UnitsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UnitsList contains a list of Units.
type UnitsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Units `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Units{}, &UnitsList{})
}