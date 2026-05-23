package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wasilak/nim/pkg/cmdutil"
	"github.com/wasilak/nim/pkg/style"
	"github.com/wasilak/nim/pkg/ui"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:          "pull",
	SilenceUsage: true,
	Short:        "Pull latest changes from git remote",
	Long:         "If ~/.config/nim is a git repository, pulls the latest changes from its remote origin.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPull(cmd.Context())
	},
}

func runPull(ctx context.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "nim")

	headerBox := style.InfoBox.Render(
		style.Header.Render("nim pull") + "\n\n" +
			style.DimStyle.Render("Pulling latest changes for ~/.config/nim"),
	)
	fmt.Println(headerBox)
	fmt.Println()

	gitPath := cmdutil.CheckExecutable("git")
	if gitPath == "" {
		fmt.Println(style.Iconf(style.IconError, style.Error, "git is not installed or not in PATH"))
		os.Exit(1)
	}

	gitDir := filepath.Join(configDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Println(style.Iconf(style.IconWarning, style.Warning, "Directory %s is not a git repository (no .git directory)", configDir))
		fmt.Println(style.Iconf(style.IconInfo, style.Info, "Initialize a git repository in %s first, or clone your dotfiles repo there.", configDir))
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check git repository: %w", err)
	}

	fmt.Println(style.Iconf(style.StyledIconSuccess, style.Success, "Found git repository at %s", configDir))

	opts := &cmdutil.RunOptions{
		Dir: configDir,
	}

	statusResult := cmdutil.Run(ctx, "git", []string{"status", "--porcelain"}, opts)
	if statusResult.Error != nil {
		return fmt.Errorf("failed to check git status: %w\n%s", statusResult.Error, statusResult.Stderr)
	}

	if statusResult.Stdout != "" {
		fmt.Println()
		fmt.Println(style.Iconf(style.IconWarning, style.Warning, "Local uncommitted changes detected:"))
		fmt.Println()
		fmt.Println(statusResult.Stdout)
		fmt.Println()
		fmt.Println(style.Iconf(style.IconError, style.Error, "Aborting pull. Commit or stash your changes first."))
		os.Exit(1)
	}

	var pullErr error
	err = ui.RunWithSpinner(ctx, style.Info, "Pulling latest changes from remote...", "pull cancelled", func(ctx context.Context, publish func(ui.MessageLevel, string)) error {
		fetchResult := cmdutil.Run(ctx, "git", []string{"fetch", "--prune"}, opts)
		if fetchResult.Error != nil {
			pullErr = fmt.Errorf("git fetch failed: %w\n%s", fetchResult.Error, fetchResult.Stderr)
			return pullErr
		}

		pullResult := cmdutil.Run(ctx, "git", []string{"pull"}, opts)
		if pullResult.Error != nil {
			pullErr = fmt.Errorf("git pull failed: %w\n%s", pullResult.Error, pullResult.Stderr)
			return pullErr
		}

		if pullResult.Stdout != "" {
			fmt.Println()
			fmt.Println(pullResult.Stdout)
		}
		if pullResult.Stderr != "" {
			fmt.Fprintln(os.Stderr, pullResult.Stderr)
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		return err
	}

	if pullErr != nil {
		fmt.Println()
		fmt.Println(style.Iconf(style.IconError, style.Error, "Pull failed"))
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(style.Iconf(style.StyledIconSuccess, style.Success, "Pull completed successfully"))
	return nil
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
