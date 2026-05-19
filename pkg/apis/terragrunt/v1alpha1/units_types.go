package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
}

// UnitsStatus defines the observed state of a Units resource.
type UnitsStatus struct {
	// LastHandledReconcileAt tracks the last reconciliation timestamp.
	// +optional
	LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`

	// LastCommitSHA is the SHA of the last detected change.
	// +optional
	LastCommitSHA string `json:"lastCommitSHA,omitempty"`
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