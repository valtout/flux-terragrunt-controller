package v1

import (
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apis "k8s.io/apimachinery/pkg/runtime"
)

// GitRepositorySpec defines the spec for a GitRepository.
// Matches the flux source-controller API.
type GitRepositorySpec struct {
	// URL is the repository URL.
	// +required
	URL string `json:"url"`

	// SecretRef is a reference to the secret containing the credentials.
	// +optional
	SecretRef *fluxmeta.LocalObjectReference `json:"secretRef,omitempty"`

	// Interval is the frequency at which to check for updates.
	// +required
	Interval metav1.Duration `json:"interval"`

	// Timeout is the maximum time to clone the repository.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// GitImplementation specifies which git client to use.
	// +optional
	GitImplementation string `json:"gitImplementation,omitempty"`

	// Ref defines the Git reference to check.
	// +optional
	Ref *GitRef `json:"ref,omitempty"`
}

// GitRef defines a Git reference.
type GitRef struct {
	// Branch is the branch to checkout.
	// +optional
	Branch string `json:"branch,omitempty"`

	// Tag is the tag to checkout.
	// +optional
	Tag string `json:"tag,omitempty"`

	// SemVer is the semantic version expression to checkout.
	// +optional
	SemVer string `json:"semver,omitempty"`

	// Commit is the commit SHA to checkout.
	// +optional
	Commit string `json:"commit,omitempty"`
}

// GitRepositoryStatus defines the observed state of a GitRepository.
type GitRepositoryStatus struct {
	// URL is the canonical URL of the artifact.
	// +optional
	URL string `json:"url,omitempty"`

	// Artifact is the artifact representing the cloned repository.
	// +optional
	Artifact *Artifact `json:"artifact,omitempty"`

	// Conditions holds the conditions for the GitRepository.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last observed generation of the resource.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// Artifact represents the output of a source reconciliation.
type Artifact struct {
	// Path is the relative file path of the artifact.
	Path string `json:"path"`

	// Revision is the resolved Git revision of the artifact.
	Revision string `json:"revision"`

	// Digest is the digest of the artifact.
	Digest string `json:"digest"`

	// LastUpdateTime is the timestamp of the last update.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.url"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// GitRepository is the Schema for the gitrepositories API.
type GitRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitRepositorySpec   `json:"spec,omitempty"`
	Status GitRepositoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GitRepositoryList contains a list of GitRepository.
type GitRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(AddToScheme)
}

// DeepCopyObject implements runtime.Object
func (in *GitRepository) DeepCopyObject() apis.Object {
	if in == nil {
		return nil
	}
	out := &GitRepository{}
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object
func (in *GitRepositoryList) DeepCopyObject() apis.Object {
	if in == nil {
		return nil
	}
	out := &GitRepositoryList{}
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *GitRepository) DeepCopyInto(out *GitRepository) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy creates a deep copy of the GitRepository.
func (in *GitRepository) DeepCopy() *GitRepository {
	if in == nil {
		return nil
	}
	out := new(GitRepository)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *GitRepositoryList) DeepCopyInto(out *GitRepositoryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GitRepository, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy creates a deep copy of GitRepositoryList.
func (in *GitRepositoryList) DeepCopy() *GitRepositoryList {
	if in == nil {
		return nil
	}
	out := new(GitRepositoryList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *GitRepositorySpec) DeepCopyInto(out *GitRepositorySpec) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(fluxmeta.LocalObjectReference)
		**out = **in
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.Ref != nil {
		in, out := &in.Ref, &out.Ref
		*out = new(GitRef)
		**out = **in
	}
}

// DeepCopyInto copies the receiver into out.
func (in *GitRepositoryStatus) DeepCopyInto(out *GitRepositoryStatus) {
	*out = *in
	if in.Artifact != nil {
		in, out := &in.Artifact, &out.Artifact
		*out = new(Artifact)
		(*in).DeepCopyInto(*out)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		copy(*out, *in)
	}
}

// DeepCopyInto copies the receiver into out.
func (in *Artifact) DeepCopyInto(out *Artifact) {
	*out = *in
	out.LastUpdateTime = in.LastUpdateTime
}

// SchemeBuilder registers types with the scheme.
var SchemeBuilder = apis.NewSchemeBuilder()

// AddToScheme is a convenience function for adding the types to the scheme.
func AddToScheme(s *apis.Scheme) error {
	return SchemeBuilder.AddToScheme(s)
}