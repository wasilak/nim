package providers

import (
	"context"
	"strings"
	"testing"

	"github.com/wasilak/nim/pkg/cmdutil"
	"github.com/wasilak/nim/pkg/provider"
	"github.com/wasilak/nim/pkg/resource"
)

func TestApply_ExecutableItem_RunsNpx(t *testing.T) {
	p := NewNpmProvider()

	var calls []string
	orig := cmdutil.RunSimpleFn
	defer func() { cmdutil.RunSimpleFn = orig }()
	cmdutil.RunSimpleFn = func(_ context.Context, name string, args ...string) (string, string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return "", "", nil
	}

	plan := provider.GroupPlan{
		Additions: []provider.GroupAddition{
			{
				Kind:  resource.KindNpmPackages,
				Group: "aitmpl",
				Items: []resource.ResourceItem{
					{
						Name:    "claude-code-templates",
						Version: "latest",
						Metadata: map[string]string{
							resource.MetaExecutable: "true",
							resource.MetaArgs:       `["--setting","statusline/context-monitor","--yes"]`,
						},
					},
				},
			},
		},
	}

	results, err := p.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("unexpected results: %+v", results)
	}
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 command, got %d: %v", len(calls), calls)
	}
	want := "npx --yes claude-code-templates@latest --setting statusline/context-monitor --yes"
	if calls[0] != want {
		t.Fatalf("wrong npx invocation:\n got: %q\nwant: %q", calls[0], want)
	}
	for _, c := range calls {
		if strings.HasPrefix(c, "npm install") {
			t.Fatalf("executable package must not be installed globally: %q", c)
		}
	}
}

func TestApply_MixedExecutableAndNormal(t *testing.T) {
	p := NewNpmProvider()

	var calls []string
	orig := cmdutil.RunSimpleFn
	defer func() { cmdutil.RunSimpleFn = orig }()
	cmdutil.RunSimpleFn = func(_ context.Context, name string, args ...string) (string, string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return "", "", nil
	}

	plan := provider.GroupPlan{
		Additions: []provider.GroupAddition{
			{
				Kind:  resource.KindNpmPackages,
				Group: "tools",
				Items: []resource.ResourceItem{
					{Name: "typescript"},
					{Name: "aitmpl", Metadata: map[string]string{resource.MetaExecutable: "true"}},
				},
			},
		},
	}

	if _, err := p.Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	var sawNpx, sawInstall bool
	for _, c := range calls {
		if c == "npx --yes aitmpl" {
			sawNpx = true
		}
		if strings.HasPrefix(c, "npm install -g") && strings.Contains(c, "typescript") {
			sawInstall = true
			if strings.Contains(c, "aitmpl") {
				t.Fatalf("executable package leaked into npm install: %q", c)
			}
		}
	}
	if !sawNpx {
		t.Fatalf("expected npx call for executable package; calls=%v", calls)
	}
	if !sawInstall {
		t.Fatalf("expected npm install -g for normal package; calls=%v", calls)
	}
}
