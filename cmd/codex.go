package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var codexCmd = &cobra.Command{
	Use:         "codex",
	Aliases:     []string{"cx"},
	Short:       "Codex usage deep-dive",
	Annotations: map[string]string{"group": groupTools},
	RunE: func(cmd *cobra.Command, args []string) error {
		res := fetchWithSpinner()
		if res.Data == nil || res.Data.Codex == nil {
			printErrors(res.Data)
			return fmt.Errorf("no Codex data")
		}
		if jsonOutput {
			return emitJSON(res)
		}
		printErrors(res.Data)
		return renderToolDeepDive("Codex", res.Data.Codex)
	},
}

func init() {
	rootCmd.AddCommand(codexCmd)
}
