package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

// chartProvider is one tool's slice of the chart window, with the color it
// renders in. filled is always exactly `days` long (FillMissingDays), so every
// provider shares the same dates/indices and stacks cleanly.
type chartProvider struct {
	name   string
	col    *color.Color
	filled []ccusage.DailyEntry
}

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
		providers := chartProviders(data, today)
		labels := chartLabels(today, days)

		printChartLegend(providers)

		render.Bold.Printf("Tokens · last %d days\n", days)
		render.StackedVerticalBars(seriesFor(providers, tokensValue), labels, formatTokensFloat)
		printChartSummary(providers, tokensValue, formatTokensFloat)
		fmt.Println()

		if detailed {
			printChartBreakdown(providers, labels)
			fmt.Println()
		}

		render.Bold.Printf("Cost · last %d days\n", days)
		render.StackedVerticalBars(seriesFor(providers, costValue), labels, render.FormatCost)
		printChartSummary(providers, costValue, render.FormatCost)

		return nil
	},
}

func chartProviders(data *ccusage.UsageData, today time.Time) []chartProvider {
	var ps []chartProvider
	if data.Claude != nil {
		ps = append(ps, chartProvider{"Claude Code", render.CyanBold, render.FillMissingDays(data.Claude.Daily, today, days)})
	}
	if data.Codex != nil {
		ps = append(ps, chartProvider{"Codex", render.GreenBold, render.FillMissingDays(data.Codex.Daily, today, days)})
	}
	return ps
}

func chartLabels(today time.Time, n int) []string {
	labels := make([]string, n)
	for i := 0; i < n; i++ {
		labels[i] = render.DayLabel(today.AddDate(0, 0, -(n-1-i)), today)
	}
	return labels
}

func tokensValue(e ccusage.DailyEntry) float64 { return float64(e.TotalTokens) }
func costValue(e ccusage.DailyEntry) float64   { return e.Cost }
func formatTokensFloat(v float64) string       { return render.FormatTokens(int64(v)) }

func seriesFor(providers []chartProvider, value func(ccusage.DailyEntry) float64) []render.Series {
	series := make([]render.Series, len(providers))
	for i, p := range providers {
		vals := make([]float64, len(p.filled))
		for j, e := range p.filled {
			vals[j] = value(e)
		}
		series[i] = render.Series{Name: p.name, Color: p.col, Values: vals}
	}
	return series
}

// printChartLegend keys the bar colors once at the top — only when both tools
// are present (a single provider needs no key).
func printChartLegend(providers []chartProvider) {
	if len(providers) < 2 {
		return
	}
	fmt.Print("  ")
	for i, p := range providers {
		if i > 0 {
			fmt.Print("   ")
		}
		p.col.Printf("██ %s", p.name)
	}
	fmt.Println()
	fmt.Println()
}

func printChartSummary(providers []chartProvider, value func(ccusage.DailyEntry) float64, format func(float64) string) {
	n := chartWindow(providers)
	var grand float64
	for _, p := range providers {
		sum := sumValues(p.filled, value)
		grand += sum
		chartSummaryRow(p.col, p.name, format(sum), format(sum/float64(maxInt(1, n))))
	}
	if len(providers) > 1 {
		chartSummaryRow(nil, "Total", format(grand), format(grand/float64(maxInt(1, n))))
	}
}

func chartSummaryRow(c *color.Color, label, value, avg string) {
	if c != nil {
		c.Printf("  %-13s", label)
	} else {
		render.Bold.Printf("  %-13s", label)
	}
	fmt.Printf("%11s", value)
	render.Dim.Printf("  · avg %s/day\n", avg)
}

func printChartBreakdown(providers []chartProvider, labels []string) {
	render.Bold.Printf("Breakdown · last %d days\n", chartWindow(providers))
	for _, p := range providers {
		p.col.Printf("  %s\n", p.name)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "    Day\tInput\tOutput\tCache")
		for i, e := range p.filled {
			fmt.Fprintf(w, "    %s\t%s\t%s\t%s\n",
				labels[i],
				render.FormatTokens(e.InputTokens),
				render.FormatTokens(e.OutputTokens),
				render.FormatTokens(e.CacheTokens))
		}
		_ = w.Flush()
	}
}

func chartWindow(providers []chartProvider) int {
	if len(providers) == 0 {
		return 0
	}
	return len(providers[0].filled)
}

func sumValues(entries []ccusage.DailyEntry, value func(ccusage.DailyEntry) float64) float64 {
	var s float64
	for _, e := range entries {
		s += value(e)
	}
	return s
}

func init() {
	rootCmd.AddCommand(chartCmd)
}
