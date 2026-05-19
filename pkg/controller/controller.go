package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"
	fluxv1 "flux-terragrunt-controller/pkg/apis/flux/v1"
	"flux-terragrunt-controller/pkg/internal/git"
)

// UnitsReconciler reconciles a Units object.
type UnitsReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	GitClientFactory func(repoURL, branch string, filters []string) gitChecker
	TempDir          string
}

type gitChecker interface {
	HasChanges(workDir, lastKnownCommit string) (string, bool, error)
	GetChangedFiles(workDir, fromCommit, toCommit string) ([]string, error)
	CloneAtCommit(workDir, commitSHA string) (string, error)
}

//+kubebuilder:rbac:groups=terragrunt.run,resources=units,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=terragrunt.run,resources=units/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch

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

	// Find the GitRepository in the same namespace
	gitRepos := &fluxv1.GitRepositoryList{}
	if err := r.List(ctx, gitRepos, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list GitRepositories: %w", err)
	}

	var gitRepo *fluxv1.GitRepository
	for i := range gitRepos.Items {
		if gitRepos.Items[i].Status.Artifact != nil {
			gitRepo = &gitRepos.Items[i]
			break
		}
	}

	if gitRepo == nil {
		r.Recorder.Eventf(units, "Normal", "NoGitRepository", "No ready GitRepository found in namespace %s", req.Namespace)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if gitRepo.Status.Artifact == nil {
		r.Recorder.Eventf(units, "Normal", "GitRepositoryNotReady", "GitRepository %s has no artifact yet", gitRepo.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check for changes across all filter paths
	gitClient := r.newGitClient(gitRepo.Spec.URL, units.Spec.Branch, units.Spec.Filters)

	workDir := r.TempDir
	if workDir == "" {
		workDir = "/tmp"
	}

	currentCommit := gitRepo.Status.Artifact.Revision
	lastKnownCommit := units.Status.LastCommitSHA

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
	} else {
		log.Info("no changes detected", "filters", units.Spec.Filters)
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
		Complete(r)
}

// GetObjectIdentifier returns a namespaced name for the object.
func GetObjectIdentifier(obj client.Object) string {
	return types.ObjectKeyFromObject(obj).String()
}

// Setup adds a new controller to the manager.
func Register(mgr ctrl.Manager) error {
	recorder := mgr.GetEventRecorderFor("units-controller")
	reconciler := &UnitsReconciler{
		Client:             mgr.GetClient(),
		Log:                mgr.GetLogger().WithName("Units"),
		Scheme:             mgr.GetScheme(),
		Recorder:           recorder,
		GitClientFactory:   nil,
		TempDir:            "/tmp",
	}

	return reconciler.SetupWithManager(mgr)
}

// Verify UnitsReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &UnitsReconciler{}