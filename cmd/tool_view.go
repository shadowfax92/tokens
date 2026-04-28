package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/render"
)

func renderToolDeepDive(name string, usage *ccusage.ToolUsage) error {
	now := time.Now()
	today := startOfDay(now)

	render.Bold.Print(name)
	render.Dim.Printf("  ·  %s\n", today.Format("Mon Jan 2"))
	fmt.Println()

	todayE := render.FindEntry(usage.Daily, today)
	yE := render.FindEntry(usage.Daily, today.AddDate(0, 0, -1))
	thisWeek, lastWeek := render.WeekTotals(usage.Daily, today)
	thisMonth, lastMonth := render.MonthTotals(usage.Daily, today)

	emit := func(label string, e ccusage.DailyEntry, prev *ccusage.DailyEntry, suffix string) {
		render.Bold.Printf("  %-12s", label)
		fmt.Printf(" %10s   ", render.FormatTokens(e.TotalTokens))
		render.Green.Printf("%9s", render.FormatCost(e.Cost))
		if prev != nil {
			c := render.PctColor(float64(e.TotalTokens), float64(prev.TotalTokens))
			fmt.Printf("   ")
			c.Printf("%-7s", render.FormatPct(float64(e.TotalTokens), float64(prev.TotalTokens)))
		}
		if suffix != "" {
			render.Dim.Printf("   %s", suffix)
		}
		fmt.Println()
		if detailed && e.TotalTokens > 0 {
			render.Dim.Printf("               in %s · out %s · cache %s\n",
				render.FormatTokens(e.InputTokens),
				render.FormatTokens(e.OutputTokens),
				render.FormatTokens(e.CacheTokens))
		}
	}

	emit("Today", todayE, &yE, "vs yesterday")
	emit("This Week", thisWeek, &lastWeek, "Mon → today")
	emit("This Month", thisMonth, &lastMonth, "")
	emit("All-time", usage.Total, nil, fmt.Sprintf("%d days of data", len(usage.Daily)))
	fmt.Println()

	filled := render.FillMissingDays(usage.Daily, today, days)
	tokVals := make([]float64, len(filled))
	costVals := make([]float64, len(filled))
	for i, e := range filled {
		tokVals[i] = float64(e.TotalTokens)
		costVals[i] = e.Cost
	}

	first := filled[0].Date.Format("Jan 02")
	last := "Today"
	pad := strings.Repeat(" ", maxInt(0, len(filled)-len(first)-len(last)))

	render.Bold.Printf("Last %d days\n", days)
	render.Cyan.Printf("  %s   tokens\n", render.Sparkline(tokVals))
	render.Green.Printf("  %s   cost\n", render.Sparkline(costVals))
	render.Dim.Printf("  %s%s%s\n", first, pad, last)

	var totalTok int64
	var totalCost float64
	for _, e := range filled {
		totalTok += e.TotalTokens
		totalCost += e.Cost
	}
	avgTok := totalTok / int64(maxInt(1, len(filled)))
	avgCost := totalCost / float64(maxInt(1, len(filled)))
	render.Dim.Printf("  Avg %s/day · %s/day\n",
		render.FormatTokens(avgTok),
		render.FormatCost(avgCost))

	return nil
}
