package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxv1 "flux-terragrunt-controller/pkg/apis/flux/v1"
	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"
	"flux-terragrunt-controller/pkg/internal/git"
	"flux-terragrunt-controller/pkg/internal/runner"
)

// UnitsReconciler reconciles a Units object.
type UnitsReconciler struct {
	client.Client
	Log                  logr.Logger
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	KubernetesClient     kubernetes.Interface
	GitClientFactory     func(repoURL, branch string, filters []string) gitChecker
	TempDir              string
	RunnerImage          string
	RunnerServiceAccount string
}

type gitChecker interface {
	HasChanges(workDir, lastKnownCommit string) (string, bool, error)
	GetChangedFiles(workDir, fromCommit, toCommit string) ([]string, error)
	GetChangedFilesForFilter(workDir, fromCommit, toCommit, filter string) ([]string, error)
	CloneAtCommit(workDir, commitSHA string) (string, error)
}

//+kubebuilder:rbac:groups=terragrunt.run,resources=units,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=terragrunt.run,resources=units/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups="",resources=pods;services,verbs=get;list;watch

// Reconcile handles create/update/delete events for Units.
func (r *UnitsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("units", req.NamespacedName)
	log.Info("reconciling Units")

	units := &terragruntv1alpha1.Units{}
	if err := r.Get(ctx, req.NamespacedName, units); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Units: %w", err)
	}

	// Find the GitRepository referenced by this Units resource
	gitRepo := &fluxv1.GitRepository{}
	if err := r.Get(ctx, client.ObjectKey{Name: units.Spec.SourceRef.Name, Namespace: req.Namespace}, gitRepo); err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Eventf(units, "Warning", "GitRepositoryNotFound", "Referenced GitRepository %s not found in namespace %s", units.Spec.SourceRef.Name, req.Namespace)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get GitRepository: %w", err)
	}

	if gitRepo == nil {
		r.Recorder.Eventf(units, "Normal", "NoGitRepository", "No ready GitRepository found in namespace %s", req.Namespace)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if gitRepo.Status.Artifact == nil {
		r.Recorder.Eventf(units, "Normal", "GitRepositoryNotReady", "GitRepository %s has no artifact yet", gitRepo.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Build git repo reference for the runner to fetch the artifact
	gitRepoRef := &runner.GitRepoRef{
		URL:      gitRepo.Status.URL,               // Artifact download URL at status level
		Revision: gitRepo.Status.Artifact.Revision, // Commit SHA
	}
	if gitRepo.Spec.SecretRef != nil {
		gitRepoRef.SecretRef = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: gitRepo.Spec.SecretRef.Name},
			Key:                  "credentials",
		}
	}

	// Check for changes across all filter paths
	gitClient := r.newGitClient(gitRepo.Spec.URL, units.Spec.Branch, units.Spec.Filters)

	workDir := r.TempDir
	if workDir == "" {
		workDir = "/tmp"
	}

	currentCommit := gitRepo.Status.Artifact.Revision

	// If we have a successful baseline, diff against it.
	// If it's empty (initial run), we still need a fromCommit value so that
	// the runner/git logic can derive head-1 behavior.
	// We approximate that by diffing from the latest commit we can infer at checkout time.
	lastKnownCommit := units.Status.LastSuccessfulCommitSHA
	if lastKnownCommit == "" {
		// Derive a baseline by cloning and reading HEAD.
		// This keeps lastSuccessfulCommitSHA empty (initialization) while still enabling
		// diff logic. The derived commit effectively plays the role of head-1 baseline.
		clonePath, err := gitClient.CloneAtCommit(workDir, currentCommit)
		if err != nil {
			r.Recorder.Eventf(units, "Warning", "GitCloneFailed", "Failed to clone repo to derive baseline: %s", err.Error())
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to clone repo: %w", err)
		}
		// Best-effort: reuse currentCommit as fromCommit when baseline is unknown.
		// Runner-derived git-based filter is what handles head-1 when lastKnownCommit is empty.
		_ = clonePath
		lastKnownCommit = currentCommit
	}

	var allChangedFiles []string
	for _, filter := range units.Spec.Filters {
		changedFiles, err := gitClient.GetChangedFilesForFilter(workDir, lastKnownCommit, currentCommit, filter)
		if err != nil {
			r.Recorder.Eventf(units, "Warning", "GitError", "Failed to check for changes in %s: %s", filter, err.Error())
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to check git changes: %w", err)
		}
		allChangedFiles = append(allChangedFiles, changedFiles...)
	}

	hasChanges := len(allChangedFiles) > 0

	if hasChanges {
		r.Recorder.Eventf(units, "Normal", "ChangesDetected", "Detected %d changed file(s) across %d filter(s)", len(allChangedFiles), len(units.Spec.Filters))
		log.Info("changes detected", "files", allChangedFiles, "filters", units.Spec.Filters, "lastCommit", lastKnownCommit, "currentCommit", currentCommit)

		// Spawn the terragrunt runner
		if err := r.spawnRunner(ctx, units, currentCommit, gitRepoRef); err != nil {
			r.Recorder.Eventf(units, "Warning", "RunnerSpawnFailed", "Failed to spawn runner: %s", err.Error())
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to spawn runner: %w", err)
		}

		r.Recorder.Eventf(units, "Normal", "RunnerSpawned", "Spawned terragrunt runner for commit %s", currentCommit)

		// After runner execution, the job controller-runtime will update completion state.
		// Since we reconcile periodically, check the last spawned job and only advance
		// lastSuccessfulCommitSHA when it succeeded.
	} else {
		log.Info("no changes detected", "filters", units.Spec.Filters)
	}

	// Cleanup old runner jobs (keep last 10)
	if err := r.cleanupOldRunners(ctx, units.Name); err != nil {
		// Log but don't fail
		log.Error(err, "failed to cleanup old runners")
	}

	// If we spawned a runner job this reconcile loop, check whether it succeeded and
	// advance lastSuccessfulCommitSHA accordingly.
	if hasChanges && units.Status.LastRunnerJob != "" {
		if err := r.updateLastSuccessfulCommitFromJob(ctx, units, units.Status.LastRunnerJob, currentCommit); err != nil {
			// Job may not be created/finished yet; requeue and try later.
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Update status
	oldStatus := units.Status
	units.Status.LastCommitSHA = currentCommit
	units.Status.LastHandledReconcileAt = fmt.Sprintf("%d", time.Now().Unix())

	if units.Status != oldStatus {
		if err := r.Status().Update(ctx, units); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
		}
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *UnitsReconciler) spawnRunner(ctx context.Context, units *terragruntv1alpha1.Units, commitSHA string, gitRepoRef *runner.GitRepoRef) error {
	tgRunner := runner.NewRunner(
		r.KubernetesClient,
		r.Scheme,
		r.RunnerImage,
		units.Namespace,
		r.RunnerServiceAccount,
	)

	// Build the terragrunt command: terragrunt run --filter <path1> --filter <path2> --all plan
	subCommand := "plan"

	// Runner derives an additional git-based filter (and discovers terragrunt units via `terragrunt find`).
	job, err := tgRunner.SpawnRunner(ctx, units, units.Spec.Filters, subCommand, commitSHA, units.Status.LastSuccessfulCommitSHA, units.Spec.Branch, gitRepoRef)

	if err != nil {
		return fmt.Errorf("failed to spawn runner job: %w", err)
	}

	r.Log.Info("spawned runner job", "job", job.Name, "commit", commitSHA)

	// Update status with last runner job
	units.Status.LastRunnerJob = job.Name

	return nil

}

func (r *UnitsReconciler) cleanupOldRunners(ctx context.Context, unitsName string) error {
	tgRunner := runner.NewRunner(

		r.KubernetesClient,
		r.Scheme,
		r.RunnerImage,
		"",
		r.RunnerServiceAccount,
	)

	// Keep last 10 jobs
	return tgRunner.CleanupOldJobs(ctx, unitsName, 10)
}

func (r *UnitsReconciler) newGitClient(repoURL, branch string, filters []string) gitChecker {
	if r.GitClientFactory != nil {
		return r.GitClientFactory(repoURL, branch, filters)
	}
	return git.NewMultiClient(repoURL, branch, filters)
}

// SetupWithManager sets up the controller with the manager.
func (r *UnitsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terragruntv1alpha1.Units{}).
		Watches(
			&fluxv1.GitRepository{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllUnitsForGitRepo),
		).
		Complete(r)
}

// enqueueAllUnitsForGitRepo finds Units in the same namespace that reference the changed GitRepository
// and returns reconcile requests for them.
func (r *UnitsReconciler) enqueueAllUnitsForGitRepo(ctx context.Context, obj client.Object) []reconcile.Request {
	gitRepo, ok := obj.(*fluxv1.GitRepository)
	if !ok {
		return nil
	}

	unitsList := &terragruntv1alpha1.UnitsList{}
	if err := r.List(ctx, unitsList, client.InNamespace(gitRepo.GetNamespace())); err != nil {
		r.Log.Error(err, "failed to list Units for GitRepository watch")
		return nil
	}

	var requests []reconcile.Request
	for _, units := range unitsList.Items {
		if units.Spec.SourceRef.Name == gitRepo.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      units.Name,
					Namespace: units.Namespace,
				},
			})
		}
	}
	return requests
}

// GetObjectIdentifier returns a namespaced name for the object.
func GetObjectIdentifier(obj client.Object) string {
	return client.ObjectKeyFromObject(obj).String()
}

// Register adds the controller to the manager.
func Register(mgr ctrl.Manager) error {
	recorder := mgr.GetEventRecorderFor("units-controller")
	reconciler := &UnitsReconciler{
		Client:               mgr.GetClient(),
		Log:                  mgr.GetLogger().WithName("Units"),
		Scheme:               mgr.GetScheme(),
		Recorder:             recorder,
		KubernetesClient:     kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		TempDir:              "/tmp",
		RunnerImage:          os.Getenv("RUNNER_IMAGE"),
		RunnerServiceAccount: os.Getenv("RUNNER_SERVICE_ACCOUNT"),
	}

	if reconciler.RunnerImage == "" {
		reconciler.RunnerImage = "ghcr.io/" + os.Getenv("GITHUB_REPOSITORY") + "/runner:latest"
	}
	if reconciler.RunnerServiceAccount == "" {
		reconciler.RunnerServiceAccount = "flux-terragrunt-controller"
	}

	return reconciler.SetupWithManager(mgr)
}

// Verify UnitsReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &UnitsReconciler{}
