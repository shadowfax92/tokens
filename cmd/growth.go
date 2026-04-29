package cmd

import (
	"fmt"
	"time"

	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

var growthCmd = &cobra.Command{
	Use:         "growth",
	Short:       "Compare usage growth across calendar periods or an explicit window",
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
		combined := render.CombineDaily(data)

		if daysFlagChanged(cmd) {
			current, previous := render.WindowTotals(combined, today, days)
			printGrowthWindow(days, today, current, previous)
			return nil
		}

		todayE := render.FindEntry(combined, today)
		yE := render.FindEntry(combined, today.AddDate(0, 0, -1))
		thisWeek, lastWeek := render.WeekTotals(combined, today)
		thisMonth, lastMonth := render.MonthTotals(combined, today)

		printGrowthBlock("Day-over-Day", todayE, yE)
		printGrowthBlock("Week-over-Week", thisWeek, lastWeek)
		printGrowthBlock("Month-over-Month", thisMonth, lastMonth)

		return nil
	},
}

func printGrowthWindow(days int, today time.Time, current, previous ccusage.DailyEntry) {
	currentStart := today.AddDate(0, 0, -(days - 1))
	previousStart := currentStart.AddDate(0, 0, -days)
	previousEnd := currentStart.AddDate(0, 0, -1)

	render.Bold.Printf("Last %d days\n", days)
	render.Dim.Printf("  Current %d days  · %s → %s\n",
		days, currentStart.Format("Jan 2"), today.Format("Jan 2"))
	render.Dim.Printf("  Previous %d days · %s → %s\n",
		days, previousStart.Format("Jan 2"), previousEnd.Format("Jan 2"))

	printGrowthRows(current, previous)
	if detailed {
		printGrowthDetailRows(current, previous)
	}
	fmt.Println()
}

func printGrowthBlock(label string, current, previous ccusage.DailyEntry) {
	render.Bold.Println(label)
	printGrowthRows(current, previous)
	if detailed {
		printGrowthDetailRows(current, previous)
	}
	fmt.Println()
}

func printGrowthRows(current, previous ccusage.DailyEntry) {
	tokC := render.PctColor(float64(current.TotalTokens), float64(previous.TotalTokens))
	costC := render.PctColor(current.Cost, previous.Cost)

	fmt.Printf("  Tokens  %10s  →  %-10s  ",
		render.FormatTokens(previous.TotalTokens),
		render.FormatTokens(current.TotalTokens))
	tokC.Printf("%s\n", render.FormatPct(float64(current.TotalTokens), float64(previous.TotalTokens)))

	fmt.Printf("  Cost    %10s  →  %-10s  ",
		render.FormatCost(previous.Cost),
		render.FormatCost(current.Cost))
	costC.Printf("%s\n", render.FormatPct(current.Cost, previous.Cost))
}

func printGrowthDetailRows(current, previous ccusage.DailyEntry) {
	printDetailGrowthRow("Input", float64(current.InputTokens), float64(previous.InputTokens), func(v float64) string {
		return render.FormatTokens(int64(v))
	})
	printDetailGrowthRow("Output", float64(current.OutputTokens), float64(previous.OutputTokens), func(v float64) string {
		return render.FormatTokens(int64(v))
	})
	printDetailGrowthRow("Cache", float64(current.CacheTokens), float64(previous.CacheTokens), func(v float64) string {
		return render.FormatTokens(int64(v))
	})
}

func printDetailGrowthRow(label string, current, previous float64, formatter func(float64) string) {
	c := render.PctColor(current, previous)
	render.Dim.Printf("  %-6s  %10s  →  %-10s  ",
		label,
		formatter(previous),
		formatter(current))
	c.Printf("%s\n", render.FormatPct(current, previous))
}

func init() {
	rootCmd.AddCommand(growthCmd)
}
