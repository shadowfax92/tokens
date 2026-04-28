package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var claudeCmd = &cobra.Command{
	Use:         "claude",
	Aliases:     []string{"cc"},
	Short:       "Claude Code usage deep-dive",
	Annotations: map[string]string{"group": groupTools},
	RunE: func(cmd *cobra.Command, args []string) error {
		res := fetchWithSpinner()
		if res.Data == nil || res.Data.Claude == nil {
			printErrors(res.Data)
			return fmt.Errorf("no Claude Code data")
		}
		if jsonOutput {
			return emitJSON(res)
		}
		printErrors(res.Data)
		return renderToolDeepDive("Claude Code", res.Data.Claude)
	},
}

func init() {
	rootCmd.AddCommand(claudeCmd)
}
