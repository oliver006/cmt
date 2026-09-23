package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gussy/cmt/internal/ai"
	"github.com/gussy/cmt/internal/config"
	"github.com/urfave/cli/v3"
)

func TestConfiguredProvider(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		provider string
		model    string
	}{
		{"commit config", nil, "codex-cli", "gpt-6-astra"},
		{"commit model override", []string{"-m", "override-model"}, "codex-cli", "override-model"},
		{"provider override", []string{"--provider", "claude", "--model", "sonnet"}, "claude-cli", "sonnet"},
		{"absorb config", []string{"absorb"}, "codex-cli", "gpt-6-astra"},
		{"absorb model override", []string{"absorb", "-m", "override-model"}, "codex-cli", "override-model"},
		{"absorb parent override", []string{"--model", "override-model", "absorb"}, "codex-cli", "override-model"},
		{"absorb provider override", []string{"absorb", "--provider", "claude", "-m", "sonnet"}, "claude-cli", "sonnet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			argsPath := filepath.Join(dir, "args")
			t.Setenv("CMT_TEST_ARGS", argsPath)
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CMT_TEST_ARGS\"\ncat > /dev/null\nprintf 'fix: use configured model\\n'\n"
			for _, name := range []string{"codex", "claude"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			cfg := &config.Config{Provider: "codex", Model: "gpt-6-astra", ModelReasoningEffort: "high"}
			called := false
			action := func(ctx context.Context, cmd *cli.Command) error {
				called = true
				provider, err := configuredProvider(cmd, cfg)
				if err != nil {
					return err
				}
				if provider.Name() != tc.provider || provider.GetDefaultModel() != tc.model {
					t.Fatalf("got %s/%s, want %s/%s", provider.Name(), provider.GetDefaultModel(), tc.provider, tc.model)
				}
				_, err = provider.GenerateCommitMessage(ctx, &ai.CommitRequest{Diff: "+configured model"})
				return err
			}
			app := newCommand()
			app.Action = action
			for _, command := range app.Commands {
				if command.Name == "absorb" {
					command.Action = action
				}
			}
			if err := app.Run(context.Background(), append([]string{"cmt"}, tc.args...)); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("command action was not called")
			}
			args, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			cliModel := tc.model
			if cliModel == "sonnet" {
				cliModel = "claude-sonnet-4-5"
			}
			if !strings.Contains(string(args), "--model\n"+cliModel+"\n") {
				t.Errorf("model was not forwarded to CLI: %s", args)
			}
			hasEffort := strings.Contains(string(args), "--config\nmodel_reasoning_effort=\"high\"\n")
			if hasEffort != (tc.provider == "codex-cli") {
				t.Errorf("reasoning effort should be forwarded only to Codex: %s", args)
			}
		})
	}
}
