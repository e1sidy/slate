package main

import (
	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for Slate.

  bash:  source <(slate completion bash)
  zsh:   source <(slate completion zsh)
  fish:  slate completion fish | source`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return cmd.Help()
			}
		},
	}
}

// taskIDCompletion provides dynamic task ID completion.
func taskIDCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if store == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	tasks, err := store.List(cmd.Context(), slate.ListParams{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var ids []string
	for _, t := range tasks {
		ids = append(ids, t.ID+"\t"+t.Title)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

// statusCompletion provides static status completion.
func statusCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"open", "in_progress", "blocked", "deferred", "closed", "cancelled"}, cobra.ShellCompDirectiveNoFileComp
}

// taskTypeCompletion provides static task type completion.
func taskTypeCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"task", "bug", "feature", "epic", "chore"}, cobra.ShellCompDirectiveNoFileComp
}

// priorityCompletion provides static priority completion.
func priorityCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"0\tcritical", "1\thigh", "2\tmedium", "3\tlow", "4\tbacklog"}, cobra.ShellCompDirectiveNoFileComp
}
