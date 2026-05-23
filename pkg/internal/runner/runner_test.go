package runner

import (
	"testing"

	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name       string
		filters    []string
		subCommand string
		want       string
	}{
		{
			name:       "single filter",
			filters:    []string{"terraform/services/a"},
			subCommand: "plan",
			want:       "terragrunt run --filter terraform/services/a --all plan",
		},
		{
			name:       "multiple filters",
			filters:    []string{"terraform/a", "terraform/b", "modules/c"},
			subCommand: "plan",
			want:       "terragrunt run --filter terraform/a --filter terraform/b --filter modules/c --all plan",
		},
		{
			name:       "apply subcommand",
			filters:    []string{"prod"},
			subCommand: "apply",
			want:       "terragrunt run --filter prod --all apply",
		},
		{
			name:       "empty filters",
			filters:    []string{},
			subCommand: "plan",
			want:       "terragrunt run --all plan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCommand(tt.filters, tt.subCommand)
			if got != tt.want {
				t.Errorf("BuildCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildEnvVars(t *testing.T) {
	tests := []struct {
		name       string
		units      *terragruntv1alpha1.Units
		commitSHA  string
		wantNames  []string
		wantPrefix bool
	}{
		{
			name: "fixed env vars only",
			units: &terragruntv1alpha1.Units{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-units",
				},
				Spec: terragruntv1alpha1.UnitsSpec{},
			},
			commitSHA: "abc123",
			wantNames: []string{"COMMIT_SHA", "UNITS_NAME"},
		},
		{
			name: "custom env vars",
			units: &terragruntv1alpha1.Units{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-units",
				},
				Spec: terragruntv1alpha1.UnitsSpec{
					Env: []terragruntv1alpha1.EnvVar{
						{Name: "MY_VAR", Value: "my-value"},
						{Name: "ANOTHER_VAR", Value: "another-value"},
					},
				},
			},
			commitSHA: "abc123",
			wantNames: []string{"COMMIT_SHA", "UNITS_NAME", "MY_VAR", "ANOTHER_VAR"},
		},
		{
			name: "secret-backed env vars",
			units: &terragruntv1alpha1.Units{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-units",
				},
				Spec: terragruntv1alpha1.UnitsSpec{
					Env: []terragruntv1alpha1.EnvVar{
						{
							Name: "SECRET_VAR",
							ValueFrom: &terragruntv1alpha1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "my-secret",
									},
									Key: "password",
								},
							},
						},
					},
				},
			},
			commitSHA: "abc123",
			wantNames: []string{"COMMIT_SHA", "UNITS_NAME", "SECRET_VAR"},
		},
		{
			name: "tf-vars with static values",
			units: &terragruntv1alpha1.Units{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-units",
				},
				Spec: terragruntv1alpha1.UnitsSpec{
					TfVars: []terragruntv1alpha1.TfVarEntry{
						{Name: "region", Value: "us-east-1"},
						{Name: "environment", Value: "production"},
					},
				},
			},
			commitSHA: "abc123",
			wantNames: []string{"COMMIT_SHA", "UNITS_NAME", "TF_VAR_region", "TF_VAR_environment"},
		},
		{
			name: "tf-vars with secret references",
			units: &terragruntv1alpha1.Units{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-units",
				},
				Spec: terragruntv1alpha1.UnitsSpec{
					TfVars: []terragruntv1alpha1.TfVarEntry{
						{
							Name: "api_key",
							SecretRef: &terragruntv1alpha1.TfVarSecretRef{
								Name: "tf-secrets",
								Key:  "api-key",
							},
						},
					},
				},
			},
			commitSHA: "abc123",
			wantNames: []string{"COMMIT_SHA", "UNITS_NAME", "TF_VAR_api_key"},
		},
		{
			name: "mixed env and tf-vars",
			units: &terragruntv1alpha1.Units{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-units",
				},
				Spec: terragruntv1alpha1.UnitsSpec{
					Env: []terragruntv1alpha1.EnvVar{
						{Name: "LOG_LEVEL", Value: "debug"},
					},
					TfVars: []terragruntv1alpha1.TfVarEntry{
						{Name: "bucket", Value: "my-bucket"},
					},
				},
			},
			commitSHA: "xyz789",
			wantNames: []string{"COMMIT_SHA", "UNITS_NAME", "LOG_LEVEL", "TF_VAR_bucket"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildEnvVars(tt.units, tt.commitSHA)

			if len(got) != len(tt.wantNames) {
				t.Errorf("BuildEnvVars() returned %d vars, want %d", len(got), len(tt.wantNames))
			}

			gotNames := make(map[string]bool)
			for _, env := range got {
				gotNames[env.Name] = true
			}

			for _, want := range tt.wantNames {
				if !gotNames[want] {
					t.Errorf("BuildEnvVars() missing expected var %q", want)
				}
			}

			// Verify COMMIT_SHA has correct value
			for _, env := range got {
				if env.Name == "COMMIT_SHA" && env.Value != tt.commitSHA {
					t.Errorf("COMMIT_SHA value = %q, want %q", env.Value, tt.commitSHA)
				}
			}
		})
	}
}
