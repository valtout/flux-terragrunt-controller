package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"
	fluxv1 "flux-terragrunt-controller/pkg/apis/flux/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testGitRepoURL     = "https://github.com/example/repo.git"
	testBranch         = "main"
	testFilter         = "terraform/"
	testLastCommitSHA  = "abc123"
	testCurrentCommit  = "def456"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	terragruntv1alpha1.AddToScheme(scheme)
	fluxv1.AddToScheme(scheme)
	return scheme
}

type mockGitChecker struct {
	hasChangesResult       (string, bool, error)
	getChangedFilesResult  ([]string, error)
	cloneAtCommitResult    (string, error)
	callLog                []string
}

func (m *mockGitChecker) HasChanges(workDir, lastKnownCommit string) (string, bool, error) {
	m.callLog = append(m.callLog, fmt.Sprintf("HasChanges(%s,%s)", workDir, lastKnownCommit))
	return m.hasChangesResult
}

func (m *mockGitChecker) GetChangedFiles(workDir, fromCommit, toCommit string) ([]string, error) {
	m.callLog = append(m.callLog, fmt.Sprintf("GetChangedFiles(%s,%s,%s)", workDir, fromCommit, toCommit))
	return m.getChangedFilesResult, nil
}

func (m *mockGitChecker) CloneAtCommit(workDir, commitSHA string) (string, error) {
	m.callLog = append(m.callLog, fmt.Sprintf("CloneAtCommit(%s,%s)", workDir, commitSHA))
	return m.cloneAtCommitResult
}

type mockStatusWriter struct {
	updateCalled bool
	updateObj    client.Object
	updateErr    error
	patchCalled  bool
	patchObj     client.Object
	err          error
}

func (m *mockStatusWriter) Update(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
	m.updateCalled = true
	m.updateObj = obj
	return m.updateErr
}

func (m *mockStatusWriter) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
	m.patchCalled = true
	m.patchObj = obj
	return m.err
}

type mockTracker struct {
	objects map[client.ObjectKey]client.Object
}

type mockClient struct {
	getResult   error
	listResult  error
	listObjs    []client.Object
	statusWriter *mockStatusWriter
	tracker      *mockTracker
}

func newMockClient() *mockClient {
	return &mockClient{
		statusWriter: &mockStatusWriter{},
		tracker: &mockTracker{
			objects: make(map[client.ObjectKey]client.Object),
		},
	}
}

func (m *mockClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if o, ok := m.tracker.objects[key]; ok {
		switch target := obj.(type) {
		case *terragruntv1alpha1.TerragruntStack:
			*target = *(o.(*terragruntv1alpha1.TerragruntStack))
		case *fluxv1.GitRepository:
			*target = *(o.(*fluxv1.GitRepository))
		}
		return nil
	}
	return m.getResult
}

func (m *mockClient) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	if m.listResult != nil {
		return m.listResult
	}
	switch target := list.(type) {
	case *terragruntv1alpha1.TerragruntStackList:
		var items []terragruntv1alpha1.TerragruntStack
		for _, obj := range m.tracker.objects {
			if ts, ok := obj.(*terragruntv1alpha1.TerragruntStack); ok {
				items = append(items, *ts)
			}
		}
		target.Items = items
	case *fluxv1.GitRepositoryList:
		var items []fluxv1.GitRepository
		for _, obj := range m.tracker.objects {
			if gr, ok := obj.(*fluxv1.GitRepository); ok {
				items = append(items, *gr)
			}
		}
		target.Items = items
	}
	return nil
}

func (m *mockClient) Status() client.StatusWriter {
	return m.statusWriter
}

func (m *mockClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	key := client.ObjectKeyFromObject(obj)
	m.tracker.objects[key] = obj
	return nil
}

func (m *mockClient) Update(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
	key := client.ObjectKeyFromObject(obj)
	m.tracker.objects[key] = obj
	return nil
}

func (m *mockClient) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	key := client.ObjectKeyFromObject(obj)
	delete(m.tracker.objects, key)
	return nil
}

func (m *mockClient) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
	return nil
}

func (m *mockClient) DeleteAllOf(_ context.Context, obj client.Object, _ ...client.DeleteAllOfOption) error {
	return nil
}

func (m *mockClient) DryRun(_ context.Context) client.Writer {
	return m
}

func (m *mockClient) FieldValidator() client.FieldValidator {
	return nil
}

func TestReconcile_WithChanges(t *testing.T) {
	mockCli := newMockClient()

	stack := &terragruntv1alpha1.TerragruntStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stack",
			Namespace: "default",
		},
		Spec: terragruntv1alpha1.TerragruntStackSpec{
			Filter: testFilter,
			Branch: testBranch,
		},
		Status: terragruntv1alpha1.TerragruntStackStatus{
			LastCommitSHA: testLastCommitSHA,
		},
	}

	gitRepo := &fluxv1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "default",
		},
		Spec: fluxv1.GitRepositorySpec{
			URL:      testGitRepoURL,
			Interval: metav1.Duration{Duration: 1 * time.Minute},
		},
		Status: fluxv1.GitRepositoryStatus{
			Artifact: &fluxv1.Artifact{
				Path:     "gitrepository/default/test-repo/def456.tar.gz",
				Revision: testCurrentCommit,
				Digest:   "sha256:abc",
			},
		},
	}

	mockCli.tracker.objects[client.ObjectKey{Name: "test-stack", Namespace: "default"}] = stack
	mockCli.tracker.objects[client.ObjectKey{Name: "test-repo", Namespace: "default"}] = gitRepo

	mockGit := &mockGitChecker{
		hasChangesResult:       (testCurrentCommit, true, nil),
		getChangedFilesResult: ([]string{"terraform/main.tf", "terraform/vars.tf"}, nil),
	}

	recorder := record.NewFakeRecorder(10)
	logger := log.NullLogger{}

	reconciler := &TerragruntStackReconciler{
		Client:           mockCli,
		Log:              logger,
		Scheme:           newTestScheme(),
		Recorder:         recorder,
		GitClientFactory: func(repoURL, branch, filter string) gitChecker { return mockGit },
		TempDir:          "/tmp",
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{Name: "test-stack", Namespace: "default"},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("Expected RequeueAfter to be set")
	}

	if !mockCli.statusWriter.updateCalled {
		t.Error("Expected status update to be called")
	}

	if len(recorder.Events) == 0 {
		t.Error("Expected events to be recorded")
	}
}

func TestReconcile_NoChanges(t *testing.T) {
	mockCli := newMockClient()

	stack := &terragruntv1alpha1.TerragruntStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stack",
			Namespace: "default",
		},
		Spec: terragruntv1alpha1.TerragruntStackSpec{
			Filter: testFilter,
			Branch: testBranch,
		},
		Status: terragruntv1alpha1.TerragruntStackStatus{
			LastCommitSHA: testCurrentCommit,
		},
	}

	gitRepo := &fluxv1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "default",
		},
		Spec: fluxv1.GitRepositorySpec{
			URL:      testGitRepoURL,
			Interval: metav1.Duration{Duration: 1 * time.Minute},
		},
		Status: fluxv1.GitRepositoryStatus{
			Artifact: &fluxv1.Artifact{
				Path:     "gitrepository/default/test-repo/def456.tar.gz",
				Revision: testCurrentCommit,
				Digest:   "sha256:abc",
			},
		},
	}

	mockCli.tracker.objects[client.ObjectKey{Name: "test-stack", Namespace: "default"}] = stack
	mockCli.tracker.objects[client.ObjectKey{Name: "test-repo", Namespace: "default"}] = gitRepo

	mockGit := &mockGitChecker{
		hasChangesResult:      (testCurrentCommit, false, nil),
		getChangedFilesResult: (nil, nil),
	}

	recorder := record.NewFakeRecorder(10)
	logger := log.NullLogger{}

	reconciler := &TerragruntStackReconciler{
		Client:           mockCli,
		Log:              logger,
		Scheme:           newTestScheme(),
		Recorder:         recorder,
		GitClientFactory: func(repoURL, branch, filter string) gitChecker { return mockGit },
		TempDir:          "/tmp",
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{Name: "test-stack", Namespace: "default"},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("Expected RequeueAfter to be set")
	}
}

func TestReconcile_NoGitRepository(t *testing.T) {
	mockCli := newMockClient()

	stack := &terragruntv1alpha1.TerragruntStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stack",
			Namespace: "default",
		},
		Spec: terragruntv1alpha1.TerragruntStackSpec{
			Filter: testFilter,
			Branch: testBranch,
		},
	}

	mockCli.tracker.objects[client.ObjectKey{Name: "test-stack", Namespace: "default"}] = stack

	recorder := record.NewFakeRecorder(10)
	logger := log.NullLogger{}

	reconciler := &TerragruntStackReconciler{
		Client:   mockCli,
		Log:      logger,
		Scheme:   newTestScheme(),
		Recorder: recorder,
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{Name: "test-stack", Namespace: "default"},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("Expected RequeueAfter to be set when no GitRepository is found")
	}
}

func TestReconcile_StackNotFound(t *testing.T) {
	mockCli := newMockClient()

	recorder := record.NewFakeRecorder(10)
	logger := log.NullLogger{}

	reconciler := &TerragruntStackReconciler{
		Client:   mockCli,
		Log:      logger,
		Scheme:   newTestScheme(),
		Recorder: recorder,
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{Name: "nonexistent", Namespace: "default"},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	if result.Requeue {
		t.Error("Did not expect Requeue when stack is not found")
	}
}

func TestReconcile_FirstReconciliation(t *testing.T) {
	mockCli := newMockClient()

	stack := &terragruntv1alpha1.TerragruntStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stack",
			Namespace: "default",
		},
		Spec: terragruntv1alpha1.TerragruntStackSpec{
			Filter: testFilter,
			Branch: testBranch,
		},
		Status: terragruntv1alpha1.TerragruntStackStatus{
			LastCommitSHA: "",
		},
	}

	gitRepo := &fluxv1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "default",
		},
		Spec: fluxv1.GitRepositorySpec{
			URL:      testGitRepoURL,
			Interval: metav1.Duration{Duration: 1 * time.Minute},
		},
		Status: fluxv1.GitRepositoryStatus{
			Artifact: &fluxv1.Artifact{
				Path:     "gitrepository/default/test-repo/def456.tar.gz",
				Revision: testCurrentCommit,
				Digest:   "sha256:abc",
			},
		},
	}

	mockCli.tracker.objects[client.ObjectKey{Name: "test-stack", Namespace: "default"}] = stack
	mockCli.tracker.objects[client.ObjectKey{Name: "test-repo", Namespace: "default"}] = gitRepo

	mockGit := &mockGitChecker{
		getChangedFilesResult: ([]string{"terraform/main.tf"}, nil),
	}

	recorder := record.NewFakeRecorder(10)
	logger := log.NullLogger{}

	reconciler := &TerragruntStackReconciler{
		Client:           mockCli,
		Log:              logger,
		Scheme:           newTestScheme(),
		Recorder:         recorder,
		GitClientFactory: func(repoURL, branch, filter string) gitChecker { return mockGit },
		TempDir:          "/tmp",
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{Name: "test-stack", Namespace: "default"},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	if !mockCli.statusWriter.updateCalled {
		t.Error("Expected status update to be called on first reconciliation")
	}
}

func TestReconcile_GitOperationError(t *testing.T) {
	mockCli := newMockClient()

	stack := &terragruntv1alpha1.TerragruntStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stack",
			Namespace: "default",
		},
		Spec: terragruntv1alpha1.TerragruntStackSpec{
			Filter: testFilter,
			Branch: testBranch,
		},
		Status: terragruntv1alpha1.TerragruntStackStatus{
			LastCommitSHA: testLastCommitSHA,
		},
	}

	gitRepo := &fluxv1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "default",
		},
		Spec: fluxv1.GitRepositorySpec{
			URL:      testGitRepoURL,
			Interval: metav1.Duration{Duration: 1 * time.Minute},
		},
		Status: fluxv1.GitRepositoryStatus{
			Artifact: &fluxv1.Artifact{
				Path:     "gitrepository/default/test-repo/def456.tar.gz",
				Revision: testCurrentCommit,
				Digest:   "sha256:abc",
			},
		},
	}

	mockCli.tracker.objects[client.ObjectKey{Name: "test-stack", Namespace: "default"}] = stack
	mockCli.tracker.objects[client.ObjectKey{Name: "test-repo", Namespace: "default"}] = gitRepo

	mockGit := &mockGitChecker{
		getChangedFilesResult: (nil, fmt.Errorf("git operation failed")),
	}

	recorder := record.NewFakeRecorder(10)
	logger := log.NullLogger{}

	reconciler := &TerragruntStackReconciler{
		Client:           mockCli,
		Log:              logger,
		Scheme:           newTestScheme(),
		Recorder:         recorder,
		GitClientFactory: func(repoURL, branch, filter string) gitChecker { return mockGit },
		TempDir:          "/tmp",
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{Name: "test-stack", Namespace: "default"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)

	if err == nil {
		t.Fatal("Expected error when git operation fails")
	}
}

func TestTerragruntStackSpec(t *testing.T) {
	spec := terragruntv1alpha1.TerragruntStackSpec{
		Filter: "terraform/",
		Branch: "main",
	}

	if spec.Filter != "terraform/" {
		t.Errorf("Expected Filter to be 'terraform/', got %s", spec.Filter)
	}
	if spec.Branch != "main" {
		t.Errorf("Expected Branch to be 'main', got %s", spec.Branch)
	}
}

func TestTerragruntStackStatus(t *testing.T) {
	status := terragruntv1alpha1.TerragruntStackStatus{
		LastHandledReconcileAt: "1234567890",
		LastCommitSHA:          "abc123",
	}

	if status.LastHandledReconcileAt != "1234567890" {
		t.Errorf("Expected LastHandledReconcileAt to be '1234567890', got %s", status.LastHandledReconcileAt)
	}
	if status.LastCommitSHA != "abc123" {
		t.Errorf("Expected LastCommitSHA to be 'abc123', got %s", status.LastCommitSHA)
	}
}

func TestGitRepositorySpec(t *testing.T) {
	spec := fluxv1.GitRepositorySpec{
		URL:      "https://github.com/example/repo.git",
		Interval: metav1.Duration{Duration: 1 * time.Minute},
		Ref: &fluxv1.GitRef{
			Branch: "main",
		},
	}

	if spec.URL != "https://github.com/example/repo.git" {
		t.Errorf("Expected URL to be 'https://github.com/example/repo.git', got %s", spec.URL)
	}
	if spec.Ref == nil || spec.Ref.Branch != "main" {
		t.Errorf("Expected Ref.Branch to be 'main'")
	}
}

func TestGetObjectIdentifier(t *testing.T) {
	stack := &terragruntv1alpha1.TerragruntStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stack",
			Namespace: "default",
		},
	}

	id := GetObjectIdentifier(stack)
	expected := "default/test-stack"

	if id != expected {
		t.Errorf("Expected identifier to be '%s', got '%s'", expected, id)
	}
}