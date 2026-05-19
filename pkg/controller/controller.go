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

// TerragruntStackReconciler reconciles a TerragruntStack object.
type TerragruntStackReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	Recorder           record.EventRecorder
	GitClientFactory   func(repoURL, branch, filter string) gitChecker
	TempDir            string
}

type gitChecker interface {
	HasChanges(workDir, lastKnownCommit string) (string, bool, error)
	GetChangedFiles(workDir, fromCommit, toCommit string) ([]string, error)
	CloneAtCommit(workDir, commitSHA string) (string, error)
}

//+kubebuilder:rbac:groups=terragrunt.run,resources=terragruntstacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=terragrunt.run,resources=terragruntstacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch

// Reconcile handles create/update/delete events for TerragruntStack.
func (r *TerragruntStackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("terragruntstack", req.NamespacedName)
	log.Info("reconciling TerragruntStack")

	stack := &terragruntv1alpha1.TerragruntStack{}
	if err := r.Get(ctx, req.NamespacedName, stack); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get TerragruntStack: %w", err)
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
		r.Recorder.Eventf(stack, "Normal", "NoGitRepository", "No ready GitRepository found in namespace %s", req.Namespace)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if gitRepo.Status.Artifact == nil {
		r.Recorder.Eventf(stack, "Normal", "GitRepositoryNotReady", "GitRepository %s has no artifact yet", gitRepo.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check for changes in the filter path
	gitClient := r.newGitClient(gitRepo.Spec.URL, stack.Spec.Branch, stack.Spec.Filter)

	workDir := r.TempDir
	if workDir == "" {
		workDir = "/tmp"
	}

	currentCommit := gitRepo.Status.Artifact.Revision
	lastKnownCommit := stack.Status.LastCommitSHA

	changedFiles, err := gitClient.GetChangedFiles(workDir, lastKnownCommit, currentCommit)
	if err != nil {
		r.Recorder.Eventf(stack, "Warning", "GitError", "Failed to check for changes: %s", err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to check git changes: %w", err)
	}

	hasChanges := len(changedFiles) > 0

	if hasChanges {
		r.Recorder.Eventf(stack, "Normal", "ChangesDetected", "Detected %d changed file(s) in path %s", len(changedFiles), stack.Spec.Filter)
		log.Info("changes detected", "files", changedFiles, "lastCommit", lastKnownCommit, "currentCommit", currentCommit)
	} else {
		log.Info("no changes detected", "path", stack.Spec.Filter)
	}

	// Update status
	oldStatus := stack.Status
	stack.Status.LastCommitSHA = currentCommit
	stack.Status.LastHandledReconcileAt = fmt.Sprintf("%d", time.Now().Unix())

	if stack.Status != oldStatus {
		if err := r.Status().Update(ctx, stack); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
		}
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *TerragruntStackReconciler) newGitClient(repoURL, branch, filter string) gitChecker {
	if r.GitClientFactory != nil {
		return r.GitClientFactory(repoURL, branch, filter)
	}
	return git.NewClient(repoURL, branch, filter)
}

// SetupWithManager sets up the controller with the manager.
func (r *TerragruntStackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terragruntv1alpha1.TerragruntStack{}).
		Complete(r)
}

// GetObjectIdentifier returns a namespaced name for the object.
func GetObjectIdentifier(obj client.Object) string {
	return types.ObjectKeyFromObject(obj).String()
}

// GenerateEvent creates a Kubernetes event for the object.
// Controller implementation uses EventRecorder instead.
func GenerateEvent(obj client.Object, eventType, reason, message string) {
	// This is a placeholder - events are managed by the controller's EventRecorder
}

// Setup adds a new controller to the manager.
func Register(mgr ctrl.Manager) error {
	recorder := mgr.GetEventRecorderFor("terragruntstack-controller")
	reconciler := &TerragruntStackReconciler{
		Client:             mgr.GetClient(),
		Log:                mgr.GetLogger().WithName("TerragruntStack"),
		Scheme:             mgr.GetScheme(),
		Recorder:           recorder,
		GitClientFactory:   nil,
		TempDir:            "/tmp",
	}

	return reconciler.SetupWithManager(mgr)
}

// Verify TerragruntStackReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &TerragruntStackReconciler{}