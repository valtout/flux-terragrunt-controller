package controller

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	fluxv1 "flux-terragrunt-controller/pkg/apis/flux/v1"
	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	testEnv *envtest.Environment
	scheme  *runtime.Scheme
	ctx     context.Context
	cancel  context.CancelFunc
)

func TestMain(m *testing.M) {
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)

	log.SetLogger(zap.New(zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			"../../config/crd/bases",
		},
		ErrorIfCRDPathMissing: true,
	}

	_, err := testEnv.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start envtest: %v", err))
	}

	scheme = runtime.NewScheme()
	if err := terragruntv1alpha1.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("Failed to add terragrunt scheme: %v", err))
	}
	if err := fluxv1.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("Failed to add flux scheme: %v", err))
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("Failed to add core scheme: %v", err))
	}

	code := m.Run()

	cancel()
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop envtest: %v", err))
	}

	os.Exit(code)
}

func setupReconciler(k8sClient client.Client) *UnitsReconciler {
	recorder := record.NewFakeRecorder(10)
	return &UnitsReconciler{
		Client:   k8sClient,
		Log:      log.Log.WithName("test"),
		Scheme:   scheme,
		Recorder: recorder,
		TempDir:  "/tmp",
	}
}

func TestIntegration_UnitsReconciliation(t *testing.T) {
	k8sClient, err := client.New(testEnv.Config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("Failed to create k8s client: %v", err)
	}

	reconciler := setupReconciler(k8sClient)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}
	if err := k8sClient.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer k8sClient.Delete(ctx, ns)

	gitRepo := &fluxv1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: ns.Name,
		},
		Spec: fluxv1.GitRepositorySpec{
			URL:      "https://github.com/example/repo.git",
			Interval: metav1.Duration{Duration: 1 * time.Minute},
		},
		Status: fluxv1.GitRepositoryStatus{
			Artifact: &fluxv1.Artifact{
				Path:     "gitrepository/test-namespace/test-repo/def456.tar.gz",
				Revision: "def456",
				Digest:   "sha256:abc",
			},
		},
	}
	if err := k8sClient.Create(ctx, gitRepo); err != nil {
		t.Fatalf("Failed to create GitRepository: %v", err)
	}
	defer k8sClient.Delete(ctx, gitRepo)

	units := &terragruntv1alpha1.Units{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-units",
			Namespace: ns.Name,
		},
		Spec: terragruntv1alpha1.UnitsSpec{
			Filters:     []string{"terraform/"},
			Branch:      "main",
			Parallelism: 2,
		},
	}
	if err := k8sClient.Create(ctx, units); err != nil {
		t.Fatalf("Failed to create Units: %v", err)
	}
	defer k8sClient.Delete(ctx, units)

	req := ctrl.Request{
		NamespacedName: client.ObjectKey{Name: units.Name, Namespace: units.Namespace},
	}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("Expected RequeueAfter to be set")
	}

	updatedUnits := &terragruntv1alpha1.Units{}
	if err := k8sClient.Get(ctx, req.NamespacedName, updatedUnits); err != nil {
		t.Fatalf("Failed to get updated Units: %v", err)
	}

	if updatedUnits.Status.LastCommitSHA == "" {
		t.Error("Expected LastCommitSHA to be set")
	}
}

func TestIntegration_UnitsWithoutGitRepository(t *testing.T) {
	k8sClient, err := client.New(testEnv.Config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("Failed to create k8s client: %v", err)
	}

	reconciler := setupReconciler(k8sClient)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-no-repo",
		},
	}
	if err := k8sClient.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer k8sClient.Delete(ctx, ns)

	units := &terragruntv1alpha1.Units{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-units-no-repo",
			Namespace: ns.Name,
		},
		Spec: terragruntv1alpha1.UnitsSpec{
			Filters: []string{"terraform/"},
			Branch:  "main",
		},
	}
	if err := k8sClient.Create(ctx, units); err != nil {
		t.Fatalf("Failed to create Units: %v", err)
	}
	defer k8sClient.Delete(ctx, units)

	req := ctrl.Request{
		NamespacedName: client.ObjectKey{Name: units.Name, Namespace: units.Namespace},
	}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.RequeueAfter != 30*time.Second {
		t.Errorf("Expected RequeueAfter to be 30s, got %v", result.RequeueAfter)
	}
}

func TestIntegration_UnitsDeleted(t *testing.T) {
	k8sClient, err := client.New(testEnv.Config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("Failed to create k8s client: %v", err)
	}

	reconciler := setupReconciler(k8sClient)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-deleted",
		},
	}
	if err := k8sClient.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer k8sClient.Delete(ctx, ns)

	req := ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "non-existent", Namespace: ns.Name},
	}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.Requeue {
		t.Error("Did not expect Requeue for non-existent resource")
	}
}
