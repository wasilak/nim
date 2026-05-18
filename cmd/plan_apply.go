package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/wasilak/nim/pkg/engine"
	"github.com/wasilak/nim/pkg/lock"
	"github.com/wasilak/nim/pkg/output"
	"github.com/wasilak/nim/pkg/style"
	"github.com/wasilak/nim/pkg/ui"
)

type PlanApplyOptions struct {
	IsApply      bool
	Confirm      bool
	OutputFormat string
	Targets      []string
	ShowDiff     bool   // If true, print contextual (syntax) diffs in output
	Namespace    string // Namespace is the active namespace for this invocation (resolved by config.GetActiveNamespace).
}

func runPlanApply(ctx context.Context, opts PlanApplyOptions) error {
	if ctx == nil {
		return fmt.Errorf("internal: context is nil")
	}

	// Acquire process lock to prevent concurrent nim invocations
	holdingPID, err := lock.Acquire()
	if err != nil {
		// Another nim process is running - print friendly error
		fmt.Println()
		fmt.Println(style.Error.Render("╔══════════════════════════════════════════════════════════════╗"))
		fmt.Println(style.Error.Render("║  ERROR                                                       ║"))
		fmt.Println(style.Error.Render("╚══════════════════════════════════════════════════════════════╝"))
		fmt.Println()
		fmt.Println("Another nim process is already running.")
		fmt.Println()
		fmt.Printf("  %s: %d\n", style.Bold.Render("PID"), holdingPID)
		fmt.Println()
		fmt.Println("Wait for it to complete, or terminate it with:")
		fmt.Printf("  %s\n", style.DimStyle.Render(fmt.Sprintf("kill %d", holdingPID)))
		fmt.Println()
		return fmt.Errorf("lock held by process %d", holdingPID)
	}
	// Always release lock on exit
	defer lock.Release()

	eng, err := engine.NewEngine()
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Inject active namespace into template context for rendering
	eng.TemplateContext.Namespace = opts.Namespace

	// Run plan (show spinner while planning)
	var result *engine.PlanResult
	var planErr error
	// Run plan with spinner (captures result and error)
	planErr = ui.RunWithSpinner(ctx, style.Info, "Planning...", "planning cancelled", func(ctx context.Context, publish func(ui.MessageLevel, string)) error {
		var err error
		result, err = eng.Plan(ctx, engine.PlanOptions{Targets: opts.Targets, ShowDiff: opts.ShowDiff, Namespace: opts.Namespace})
		return err
	})
	if planErr != nil {
		return fmt.Errorf("plan failed: %w", planErr)
	}

	// Print warnings for unmatched targets
	if len(result.UnmatchedTargets) > 0 {
		for _, t := range result.UnmatchedTargets {
			fmt.Fprintf(os.Stderr, "%s target %q did not match any resources\n", style.Warning.Render("Warning:"), t)
		}
	}
	// Early exit if nothing to do
	allEmpty := true
	for _, groupPlan := range result.ProviderPlans {
		if len(groupPlan.Additions) > 0 || len(groupPlan.Removals) > 0 || len(groupPlan.Modifications) > 0 || len(groupPlan.Cleanup) > 0 || len(groupPlan.Drifted) > 0 {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		// Show the friendly no-changes card for both plan and apply flows.
		// Previously apply printed a plain message while plan used the
		// celebratory UI; unify behaviour so users see the same output.
		ui.RenderNoChanges()
		return nil
	}
	if !result.HasChanges {
		ui.RenderNoChanges()
		return nil
	}

	// Plan display
	if err := ui.DisplayPlanResult(result, output.Format(opts.OutputFormat), opts.ShowDiff); err != nil {
		return fmt.Errorf("plan output failed: %w", err)
	}

	if !opts.IsApply {
		return nil
	}

	// ===== Handle confirmation (before progress setup!) =====
	confirmed := opts.Confirm
	if !confirmed {
		fmt.Fprint(os.Stdout, "\n") // ensure visual separation, required for some terminals
		totalChanges := result.TotalAdditions + result.TotalModifications + result.TotalRemovals + result.TotalDrifted
		var changeSummary string
		if totalChanges == 1 {
			changeSummary = "Apply 1 change?"
		} else {
			changeSummary = fmt.Sprintf("Apply %d changes?", totalChanges)
		}
		var confirmErr error
		// Prompt for a single keypress. If terminal raw mode isn't available
		// (e.g., input redirected), fall back to line-oriented Scanln.
		fmt.Print(style.PromptPrefix(changeSummary))
		key, err := ui.ReadSingleKey()
		if err != nil {
			// fallback to Scanln
			var resp string
			_, confirmErr = fmt.Scanln(&resp)
			if confirmErr != nil && confirmErr.Error() != "unexpected newline" {
				return fmt.Errorf("confirmation prompt error: %w", confirmErr)
			}
			key = strings.ToLower(resp)
		}
		if key != "y" && key != "yes" {
			fmt.Println()
			fmt.Println("→ Apply cancelled.")
			return nil
		}
		confirmed = true
	}

	// ==== Now, setup progress and actually apply ====
	// TODO: Add progress bar with new UI toolkit
	appOpts := engine.ApplyOptions{Confirm: confirmed}

	// Run apply under the helper that manages spinner lifecycle and cancellation
	if err := ui.RunWithSpinner(ctx, style.Info, "Applying changes...", "apply cancelled", func(ctx context.Context, publish func(ui.MessageLevel, string)) error {
		// Wire per-item callbacks so we publish transient updates to the
		// spinner. publish is safe to call from any goroutine.
		appOpts.OnItemStart = func(kind, group, item string) {
			publish(ui.LevelInfo, fmt.Sprintf("Applying %s/%s[%s]", kind, group, item))
		}

		appOpts.OnItemComplete = func(kind, group, item string, err error) {
			if err != nil {
				publish(ui.LevelError, fmt.Sprintf("Failed %s/%s[%s]", kind, group, item))
			} else {
				publish(ui.LevelSuccess, fmt.Sprintf("Done %s/%s[%s]", kind, group, item))
			}
		}

		// Also wire engine OnMessage to publish final/summary messages when
		// available so the spinner can display them.
		appOpts.OnMessage = func(level engine.MessageLevel, msg string) {
			switch level {
			case engine.MessageLevelSuccess:
				publish(ui.LevelSuccess, msg)
			case engine.MessageLevelError:
				publish(ui.LevelError, msg)
			case engine.MessageLevelWarning:
				publish(ui.LevelInfo, msg)
			default:
				publish(ui.LevelInfo, msg)
			}
		}

		return eng.Apply(ctx, result, appOpts)
	}); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}
	fmt.Println(style.StyledIconText(style.StyledIconSuccess, style.Success, "Changes applied successfully."))
	return nil

}
