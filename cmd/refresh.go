package cmd

import (
	"fmt"
	"time"

	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:         "refresh",
	Short:       "Bust the cache and re-fetch usage data",
	Annotations: map[string]string{"group": groupData},
	RunE: func(cmd *cobra.Command, args []string) error {
		stop := startSpinner("Refreshing usage data from ccusage and @ccusage/codex...")
		res := ccusage.Fetch(ccusage.FetchOptions{
			NoCache:  true,
			CacheTTL: time.Duration(cfg.CacheTTLMinutes) * time.Minute,
		})
		stop()
		printErrors(res.Data)
		if res.Data == nil || (res.Data.Claude == nil && res.Data.Codex == nil) {
			return fmt.Errorf("could not fetch usage data")
		}
		render.Bold.Printf("Done in %s.\n", humanizeDuration(res.FetchTook))
		render.Dim.Printf("Cache: %s\n", ccusage.CachePath())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
