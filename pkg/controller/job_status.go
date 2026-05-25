package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"

	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"
	"flux-terragrunt-controller/pkg/internal/runner"
)

func (r *UnitsReconciler) updateLastSuccessfulCommitFromJob(
	ctx context.Context,
	units *terragruntv1alpha1.Units,
	jobName string,
	currentCommit string,
) error {
	// Get the job.
	tgRunner := runner.NewRunner(
		r.KubernetesClient,
		r.Scheme,
		r.RunnerImage,
		units.Namespace,
		r.RunnerServiceAccount,
	)

	job, err := tgRunner.GetJob(ctx, jobName)
	if err != nil {
		// Job may not exist yet; treat as no-op and requeue.
		return err
	}

	// Job completed successfully.
	if job.Status.Conditions != nil {
		for _, c := range job.Status.Conditions {
			if c.Type == batchv1.JobComplete && c.Status == "True" {
				// Only advance if it matches the commit this controller intended to run.
				// Runner labels the job with terragrunt.run/commit.
				if job.Labels["terragrunt.run/commit"] == currentCommit {
					units.Status.LastSuccessfulCommitSHA = currentCommit
				}
				return nil
			}
		}
	}

	// If Succeeded condition not present, do nothing (Failed/Running/etc.).
	// Note: we do not mutate lastSuccessfulCommitSHA.
	if job.Status.CompletionTime == nil {
		// Not completed yet.
		return nil
	}

	// Completed but not succeeded.
	return nil
}
