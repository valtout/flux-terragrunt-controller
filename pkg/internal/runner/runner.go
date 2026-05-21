package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"
)

// Runner handles spawning terragrunt runner pods.
type Runner struct {
	clientset      kubernetes.Interface
	scheme         *runtime.Scheme
	image          string
	namespace      string
	serviceAccount string
}

// NewRunner creates a new runner manager.
func NewRunner(clientset kubernetes.Interface, scheme *runtime.Scheme, image, namespace, serviceAccount string) *Runner {
	return &Runner{
		clientset:      clientset,
		scheme:         scheme,
		image:          image,
		namespace:      namespace,
		serviceAccount: serviceAccount,
	}
}

// BuildCommand constructs the terragrunt command from filters.
// Example: filters=["terraform/a", "terraform/b"] -> "terragrunt run --filter terraform/a --filter terraform/b --all plan"
func BuildCommand(filters []string, subCommand string) string {
	var args []string
	args = append(args, "terragrunt", "run")

	for _, filter := range filters {
		args = append(args, "--filter", filter)
	}

	args = append(args, "--all", subCommand)

	return strings.Join(args, " ")
}

// BuildEnvVars builds the environment variables for the runner pod.
// It combines:
// - Fixed env vars (COMMIT_SHA, UNITS_NAME)
// - Custom env vars from the Units spec (native K8s format, including secret refs)
// - TF_VAR_* vars derived from tf-vars spec (secret-protected)
func BuildEnvVars(units *terragruntv1alpha1.Units, commitSHA string) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Fixed env vars
	envVars = append(envVars, corev1.EnvVar{
		Name:  "COMMIT_SHA",
		Value: commitSHA,
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "UNITS_NAME",
		Value: units.Name,
	})

	// Custom env vars from Units spec
	for _, env := range units.Spec.Env {
		envVar := corev1.EnvVar{
			Name: env.Name,
		}

		if env.Value != "" {
			envVar.Value = env.Value
		} else if env.ValueFrom != nil {
			if env.ValueFrom.SecretKeyRef != nil {
				envVar.ValueFrom = &corev1.EnvVarSource{
					SecretKeyRef: env.ValueFrom.SecretKeyRef,
				}
			} else if env.ValueFrom.ConfigMapKeyRef != nil {
				envVar.ValueFrom = &corev1.EnvVarSource{
					ConfigMapKeyRef: env.ValueFrom.ConfigMapKeyRef,
				}
			}
		}

		envVars = append(envVars, envVar)
	}

	// TF_VAR_* vars from tf-vars spec
	// These are prefixed with TF_VAR_ and can reference secrets for protection
	for _, tfVar := range units.Spec.TfVars {
		envVar := corev1.EnvVar{
			Name: fmt.Sprintf("TF_VAR_%s", tfVar.Name),
		}

		if tfVar.SecretRef != nil {
			// Secret reference - value won't appear in logs/describe
			envVar.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: tfVar.SecretRef.Name,
					},
					Key: tfVar.SecretRef.Key,
				},
			}
		} else {
			// Static value - keep empty if not set, caller should ensure value is provided
			envVar.Value = tfVar.Value
		}

		envVars = append(envVars, envVar)
	}

	return envVars
}

// SpawnRunner creates a job to run terragrunt with the given parameters.
func (r *Runner) SpawnRunner(ctx context.Context, units *terragruntv1alpha1.Units, filters []string, subCommand string, commitSHA string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("tg-runner-%s-%s", units.Name, commitSHA[:8])

	command := BuildCommand(filters, subCommand)

	// Use the runner image that has terragrunt installed
	runnerImage := r.image
	if runnerImage == "" {
		runnerImage = "ghcr.io/<owner>/flux-terragrunt-controller/runner:latest"
	}

	backoffLimit := int32(0)

	// Build environment variables
	envVars := BuildEnvVars(units, commitSHA)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "flux-terragrunt-controller",
				"terragrunt.run/units":         units.Name,
				"terragrunt.run/commit":        commitSHA,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "flux-terragrunt-controller",
						"terragrunt.run/units":        units.Name,
						"terragrunt.run/commit":       commitSHA,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: r.serviceAccount,
					Containers: []corev1.Container{
						{
							Name:    "terragrunt",
							Image:   runnerImage,
							Command: []string{"/bin/bash", "-c"},
							Args:    []string{command},
							Env:     envVars,
						},
					},
				},
			},
		},
	}

	createdJob, err := r.clientset.BatchV1().Jobs(r.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner job: %w", err)
	}

	return createdJob, nil
}

// GetJob retrieves a job by name.
func (r *Runner) GetJob(ctx context.Context, name string) (*batchv1.Job, error) {
	return r.clientset.BatchV1().Jobs(r.namespace).Get(ctx, name, metav1.GetOptions{})
}

// DeleteJob deletes a job by name.
func (r *Runner) DeleteJob(ctx context.Context, name string) error {
	return r.clientset.BatchV1().Jobs(r.namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ListJobs lists all runner jobs for a given units resource.
func (r *Runner) ListJobs(ctx context.Context, unitsName string) (*batchv1.JobList, error) {
	return r.clientset.BatchV1().Jobs(r.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("terragrunt.run/units=%s", unitsName),
	})
}

// CleanupOldJobs removes completed jobs older than the retention period.
func (r *Runner) CleanupOldJobs(ctx context.Context, unitsName string, retentionDays int) error {
	jobs, err := r.ListJobs(ctx, unitsName)
	if err != nil {
		return err
	}

	cutoffTime := metav1.Now().Add(-24 * time.Hour * time.Duration(retentionDays))
	cutoff := &metav1.Time{Time: cutoffTime}

	for _, job := range jobs.Items {
		if job.Status.CompletionTime != nil && job.Status.CompletionTime.Before(cutoff) {
			if err := r.DeleteJob(ctx, job.Name); err != nil {
				// Log but continue
				continue
			}
		}
	}

	return nil
}