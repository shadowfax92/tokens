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
	Short:       "Day-over-day, week-over-week, month-over-month deltas",
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

func printGrowthBlock(label string, current, previous ccusage.DailyEntry) {
	render.Bold.Println(label)
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
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(growthCmd)
}
