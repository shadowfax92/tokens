package cmd

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

var chartCmd = &cobra.Command{
	Use:         "chart",
	Short:       "Full-size daily bar charts for tokens and cost",
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
		filled := render.FillMissingDays(combined, today, days)

		tokVals := make([]float64, len(filled))
		costVals := make([]float64, len(filled))
		labels := make([]string, len(filled))
		var totalTok int64
		var totalCost float64
		for i, e := range filled {
			tokVals[i] = float64(e.TotalTokens)
			costVals[i] = e.Cost
			labels[i] = render.DayLabel(e.Date, today)
			totalTok += e.TotalTokens
			totalCost += e.Cost
		}

		render.Bold.Printf("Tokens · last %d days\n", days)
		render.VerticalBars(tokVals, labels, func(v float64) string {
			return render.FormatTokens(int64(v))
		}, color.New(color.FgCyan))
		render.Dim.Printf("  Total %s · avg %s/day\n\n",
			render.FormatTokens(totalTok),
			render.FormatTokens(totalTok/int64(maxInt(1, len(filled)))))

		render.Bold.Printf("Cost · last %d days\n", days)
		render.VerticalBars(costVals, labels, render.FormatCost, color.New(color.FgGreen))
		render.Dim.Printf("  Total %s · avg %s/day\n",
			render.FormatCost(totalCost),
			render.FormatCost(totalCost/float64(maxInt(1, len(filled)))))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(chartCmd)
}
