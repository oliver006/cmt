package ai

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCodexModelConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name         string
		config       *ProviderConfig
		requestModel string
		wantArgs     []string
	}{
		{name: "CLI default"},
		{name: "empty configured model", config: &ProviderConfig{Timeout: 5}},
		{
			name: "configured model and effort",
			config: &ProviderConfig{
				DefaultModel: "gpt-6-astra", ModelReasoningEffort: "high", Timeout: 5,
			},
			wantArgs: []string{"--model", "gpt-6-astra", "--config", "model_reasoning_effort=\"high\""},
		},
		{
			name:         "request overrides configured model",
			config:       &ProviderConfig{DefaultModel: "configured-model", Timeout: 5},
			requestModel: "request-model",
			wantArgs:     []string{"--model", "request-model"},
		},
		{
			name:         "explicit CLI default",
			config:       &ProviderConfig{DefaultModel: "configured-model", Timeout: 5},
			requestModel: "default",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			argsPath := filepath.Join(dir, "args")
			promptPath := filepath.Join(dir, "prompt")
			t.Setenv("CMT_TEST_ARGS", argsPath)
			t.Setenv("CMT_TEST_PROMPT", promptPath)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CMT_TEST_ARGS\"\ncat > \"$CMT_TEST_PROMPT\"\nprintf 'fix: use configured model\\n'\n"
			if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			provider, err := NewCodeX(tc.config)
			if err != nil {
				t.Fatal(err)
			}
			response, err := provider.GenerateCommitMessage(context.Background(), &CommitRequest{
				Diff: "+configured model", Model: tc.requestModel,
			})
			if err != nil {
				t.Fatal(err)
			}
			if response.Message != "fix: use configured model" {
				t.Fatalf("unexpected response: %q", response.Message)
			}
			args, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]string{"exec", "--sandbox", "read-only", "--skip-git-repo-check"}, tc.wantArgs...)
			if got := strings.Split(strings.TrimSpace(string(args)), "\n"); !reflect.DeepEqual(got, want) {
				t.Errorf("codex arguments = %q, want %q", got, want)
			}
			prompt, err := os.ReadFile(promptPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(prompt), "+configured model") {
				t.Error("diff was not sent to codex on stdin")
			}
		})
	}
}
