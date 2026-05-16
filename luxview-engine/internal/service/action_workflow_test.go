package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

func TestParseGitHubWorkflowExtractsJobsAndSteps(t *testing.T) {
	workflow, err := ParseGitHubWorkflow(`name: CI

on:
  push:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
      - name: Install
        run: npm ci
      - name: Test
        run: npm test
`)
	if err != nil {
		t.Fatalf("ParseGitHubWorkflow() error = %v", err)
	}
	if workflow.Name != "CI" {
		t.Fatalf("workflow name = %q, want CI", workflow.Name)
	}
	if len(workflow.Jobs) != 1 {
		t.Fatalf("jobs len = %d, want 1", len(workflow.Jobs))
	}
	job := workflow.Jobs[0]
	if job.Name != "test" {
		t.Fatalf("job name = %q, want test", job.Name)
	}
	if job.Image != nodeActionImage {
		t.Fatalf("job image = %q, want %q", job.Image, nodeActionImage)
	}
	if len(job.Steps) != 4 {
		t.Fatalf("steps len = %d, want 4", len(job.Steps))
	}
	if job.Steps[2].Kind != actionKindRun || job.Steps[2].Command != "npm ci" {
		t.Fatalf("step 3 = %#v, want run npm ci", job.Steps[2])
	}
}

func TestExecuteUsesStepRejectsUnsupportedAction(t *testing.T) {
	svc := &ActionService{}
	status, exitCode, log := svc.executeUsesStep(context.Background(), uuid.Nil, t.TempDir(), model.ActionStep{
		Kind: actionKindUses,
		Uses: "unknown/action@v1",
	})

	if status != model.ActionFailed {
		t.Fatalf("status = %q, want %q", status, model.ActionFailed)
	}
	if exitCode == 0 {
		t.Fatal("exitCode = 0, want non-zero")
	}
	if log == "" {
		t.Fatal("log is empty")
	}
}

func TestParseGitHubWorkflowSupportsMultilineRun(t *testing.T) {
	workflow, err := ParseGitHubWorkflow(`name: CI
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Build
        run: |
          npm ci
          npm run build
`)
	if err != nil {
		t.Fatalf("ParseGitHubWorkflow() error = %v", err)
	}
	command := workflow.Jobs[0].Steps[0].Command
	want := "npm ci\nnpm run build"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestParseGitHubWorkflowCapturesArtifactInputs(t *testing.T) {
	workflow, err := ParseGitHubWorkflow(`name: CI
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Upload dist
        uses: actions/upload-artifact@v4
        with:
          name: web-dist
          path: dist
`)
	if err != nil {
		t.Fatalf("ParseGitHubWorkflow() error = %v", err)
	}
	step := workflow.Jobs[0].Steps[0]
	if step.Uses != "actions/upload-artifact@v4" {
		t.Fatalf("uses = %q, want upload-artifact", step.Uses)
	}
	if step.Inputs["name"] != "web-dist" || step.Inputs["path"] != "dist" {
		t.Fatalf("inputs = %#v, want name/path", step.Inputs)
	}
}

func TestParseGitHubWorkflowCapturesMultilineArtifactPath(t *testing.T) {
	workflow, err := ParseGitHubWorkflow(`name: CI
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/upload-artifact@v4
        with:
          name: binaries
          path: |
            dist/**
            build/*.zip
`)
	if err != nil {
		t.Fatalf("ParseGitHubWorkflow() error = %v", err)
	}
	want := "dist/**\nbuild/*.zip"
	if got := workflow.Jobs[0].Steps[0].Inputs["path"]; got != want {
		t.Fatalf("path input = %q, want %q", got, want)
	}
}

func TestReadWorkflowFileDiscoversDefaultWorkflow(t *testing.T) {
	repoDir := t.TempDir()
	workflowDir := filepath.Join(repoDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "pipeline.yml"), []byte("name: Pipeline\njobs: {}\n"), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	path, content, err := readWorkflowFile(repoDir, "")
	if err != nil {
		t.Fatalf("readWorkflowFile() error = %v", err)
	}
	if path != ".github/workflows/pipeline.yml" {
		t.Fatalf("path = %q, want pipeline workflow", path)
	}
	if content == "" {
		t.Fatal("content is empty")
	}
}

func TestReadWorkflowFileReturnsWorkflowNotFound(t *testing.T) {
	_, _, err := readWorkflowFile(t.TempDir(), "")
	if !errors.Is(err, ErrActionWorkflowNotFound) {
		t.Fatalf("err = %v, want ErrActionWorkflowNotFound", err)
	}
}

func TestMaskSecretValuesMasksOnlyRealSecrets(t *testing.T) {
	got := maskSecretValues("token=super-secret-value short=abc", map[string]string{
		"TOKEN": "super-secret-value",
		"TINY":  "abc",
	})
	want := "token=*** short=abc"
	if got != want {
		t.Fatalf("maskSecretValues() = %q, want %q", got, want)
	}
}

func TestIsValidActionSecretKey(t *testing.T) {
	valid := []string{"FIREBASE_TOKEN", "CODECOV_TOKEN", "SSH_KEY_1"}
	for _, key := range valid {
		if !IsValidActionSecretKey(key) {
			t.Fatalf("IsValidActionSecretKey(%q) = false, want true", key)
		}
	}

	invalid := []string{"", "firebase_token", "1TOKEN", "TOKEN-NAME", "TOKEN NAME"}
	for _, key := range invalid {
		if IsValidActionSecretKey(key) {
			t.Fatalf("IsValidActionSecretKey(%q) = true, want false", key)
		}
	}
}

func TestSafeWorkspacePathRejectsEscapes(t *testing.T) {
	if _, err := safeWorkspacePath(t.TempDir(), "../outside"); err == nil {
		t.Fatal("safeWorkspacePath() error = nil, want error")
	}
}

func TestExpandArtifactSourcesSupportsRecursiveGlob(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "dist", "app.js"), "app")
	writeTestFile(t, filepath.Join(workspace, "dist", "nested", "asset.css"), "css")
	writeTestFile(t, filepath.Join(workspace, "build", "app.zip"), "zip")
	writeTestFile(t, filepath.Join(workspace, "build", "notes.txt"), "notes")

	sources, err := expandArtifactSources(workspace, "dist/**\nbuild/*.zip")
	if err != nil {
		t.Fatalf("expandArtifactSources() error = %v", err)
	}
	got := make([]string, 0, len(sources))
	for _, source := range sources {
		got = append(got, source.RelPath)
	}
	want := []string{"build/app.zip", "dist"}
	if len(got) != len(want) {
		t.Fatalf("sources = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sources = %#v, want %#v", got, want)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir test file dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
