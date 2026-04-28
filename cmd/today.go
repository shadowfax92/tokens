package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

var todayCmd = &cobra.Command{
	Use:         "today",
	Short:       "Show today's tokens and cost (compact, scriptable)",
	Annotations: map[string]string{"group": groupViews},
	RunE: func(cmd *cobra.Command, args []string) error {
		res := fetchWithSpinner()
		data := res.Data
		if data == nil || (data.Claude == nil && data.Codex == nil) {
			printErrors(data)
			return fmt.Errorf("could not fetch usage data")
		}
		if jsonOutput {
			return emitJSON(res)
		}
		printErrors(data)

		today := startOfDay(time.Now())
		render.Dim.Printf("Today · %s\n", today.Format("Mon Jan 2"))

		var totalTok int64
		var totalCost float64

		emit := func(name string, entry ccusage.DailyEntry, c *color.Color) {
			c.Printf("%-13s", name)
			fmt.Printf(" %10s   ", render.FormatTokens(entry.TotalTokens))
			render.Green.Printf("%9s\n", render.FormatCost(entry.Cost))
			if detailed && entry.TotalTokens > 0 {
				render.Dim.Printf("              in %s · out %s · cache %s\n",
					render.FormatTokens(entry.InputTokens),
					render.FormatTokens(entry.OutputTokens),
					render.FormatTokens(entry.CacheTokens))
			}
			totalTok += entry.TotalTokens
			totalCost += entry.Cost
		}

		if data.Claude != nil {
			emit("Claude Code", render.FindEntry(data.Claude.Daily, today), render.CyanBold)
		}
		if data.Codex != nil {
			emit("Codex", render.FindEntry(data.Codex.Daily, today), render.GreenBold)
		}

		render.Dim.Printf("              %s   %s\n",
			strings.Repeat("─", 10),
			strings.Repeat("─", 9))
		render.Bold.Printf("%-13s", "Total")
		fmt.Printf(" %10s   ", render.FormatTokens(totalTok))
		render.GreenBold.Printf("%9s\n", render.FormatCost(totalCost))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(todayCmd)
}
