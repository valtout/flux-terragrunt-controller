package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TerragruntStackSpec defines the spec for a TerragruntStack.
type TerragruntStackSpec struct {
	// Filter is the path within the GitRepository to monitor for changes.
	// +required
	Filter string `json:"filter"`

	// Branch is the Git branch to monitor.
	// +required
	Branch string `json:"branch"`
}

// TerragruntStackStatus defines the observed state of a TerragruntStack.
type TerragruntStackStatus struct {
	// LastHandledReconcileAt tracks the last reconciliation timestamp.
	// +optional
	LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`

	// LastCommitSHA is the SHA of the last detected change.
	// +optional
	LastCommitSHA string `json:"lastCommitSHA,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=terragruntstacks,shortName=tgstack
// +genclient
// +genclient:nonNamespaced

// TerragruntStack is the Schema for the terragruntstacks API.
type TerragruntStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerragruntStackSpec   `json:"spec,omitempty"`
	Status TerragruntStackStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TerragruntStackList contains a list of TerragruntStack.
type TerragruntStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerragruntStack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerragruntStack{}, &TerragruntStackList{})
}