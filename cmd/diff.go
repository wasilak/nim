package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/wasilak/nim/pkg/config"
	"github.com/wasilak/nim/pkg/output"
)

var diffOutputFlag string
var diffTargetFlags []string
var diffNamespaceFlag string

var diffCmd = &cobra.Command{
	Use:          "diff",
	SilenceUsage: true,
	Short:        "Alias for 'plan --diff'",
	Long:         "Show contextual diffs for what would change. Equivalent to 'nim plan --diff'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiff(cmd.Context())
	},
}

func init() {
	diffCmd.Flags().StringVarP(&diffOutputFlag, "output", "o", "", "Output format (plain, tree, json)")
	diffCmd.Flags().StringArrayVarP(&diffTargetFlags, "target", "t", nil, "Target specific resources (format: Kind, Kind/Group, or Kind/Group[Item])")
	diffCmd.Flags().StringVarP(&diffNamespaceFlag, "namespace", "n", "", "Active namespace for this invocation (overrides NIM_NAMESPACE env var)")

	rootCmd.AddCommand(diffCmd)
}

func runDiff(ctx context.Context) error {
	return runPlanApply(ctx, PlanApplyOptions{
		IsApply:      false,
		Confirm:      false,
		OutputFormat: string(output.Format(diffOutputFlag)),
		Targets:      diffTargetFlags,
		ShowDiff:     true,
		Namespace:    config.GetActiveNamespace(diffNamespaceFlag),
	})
}
