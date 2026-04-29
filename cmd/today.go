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
	Short:       "Show today's tokens and cost, or a compact daily window",
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
		if daysFlagChanged(cmd) && days > 1 {
			renderTodayWindow(data, today, days)
			return nil
		}

		render.Dim.Printf("Today · %s\n", today.Format("Mon Jan 2"))
		renderTodayRows(data, today)
		return nil
	},
}

func renderTodayWindow(data *ccusage.UsageData, today time.Time, days int) {
	filled := render.FillMissingDays(render.CombineDaily(data), today, days)
	if len(filled) == 0 {
		return
	}

	start := filled[0].Date.Format("Mon Jan 2")
	end := today.Format("Mon Jan 2")
	render.Dim.Printf("Last %d days · %s → %s\n", days, start, end)

	for i, e := range filled {
		if i > 0 {
			fmt.Println()
		}
		render.Bold.Println(render.DayLabel(e.Date, today))
		renderTodayRows(data, e.Date)
	}
}

func renderTodayRows(data *ccusage.UsageData, date time.Time) {
	claudeByDate := mapByDate(data.Claude)
	codexByDate := mapByDate(data.Codex)
	key := date.Format("2006-01-02")

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
		emit("Claude Code", claudeByDate[key], render.CyanBold)
	}
	if data.Codex != nil {
		emit("Codex", codexByDate[key], render.GreenBold)
	}

	render.Dim.Printf("              %s   %s\n",
		strings.Repeat("─", 10),
		strings.Repeat("─", 9))
	render.Bold.Printf("%-13s", "Total")
	fmt.Printf(" %10s   ", render.FormatTokens(totalTok))
	render.GreenBold.Printf("%9s\n", render.FormatCost(totalCost))
}

func init() {
	rootCmd.AddCommand(todayCmd)
}
