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
// provider shares the same dates/indices and lines up column-for-column.
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
		byModel, err := cmd.Flags().GetBool("by-model")
		if err != nil {
			return err
		}
		defer resetChartByModelFlag(cmd)

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
		fullLabels, shortLabels := chartLabels(today, days)
		width := render.TermWidth()

		tokSeries := seriesFor(providers, tokensValue)
		costSeries := seriesFor(providers, costValue)
		if byModel {
			tokSeries, costSeries = render.ModelSeries(data, today, days)
			cleanModelSeriesNames(tokSeries)
			cleanModelSeriesNames(costSeries)
		}

		printChartLegend(tokSeries)

		render.Bold.Printf("Tokens · last %d days%s\n", days, peakSuffix(tokSeries, formatTokensFloat))
		render.GroupedVerticalBars(tokSeries, fullLabels, shortLabels, width)
		printChartSummary(tokSeries, formatTokensFloat)
		fmt.Println()

		if detailed && !byModel {
			printChartBreakdown(providers, today)
			fmt.Println()
		}

		render.Bold.Printf("Cost · last %d days%s\n", days, peakSuffix(costSeries, render.FormatCost))
		render.GroupedVerticalBars(costSeries, fullLabels, shortLabels, width)
		printChartSummary(costSeries, render.FormatCost)

		return nil
	},
}

func chartProviders(data *ccusage.UsageData, today time.Time) []chartProvider {
	var ps []chartProvider
	if data.Claude != nil {
		ps = append(ps, chartProvider{"Claude Code", render.CyanBold, render.FillMissingDays(data.Claude.Daily, today, days)})
	}
	if data.Codex != nil {
		ps = append(ps, chartProvider{"Codex", render.MagentaBold, render.FillMissingDays(data.Codex.Daily, today, days)})
	}
	return ps
}

// chartLabels builds parallel full and short per-column labels for the window.
// Full ("Mon 02"/"Today") are used when the layout has room; short (day-of-month)
// keep narrow terminals legible. The grouped renderer picks whichever fits.
func chartLabels(today time.Time, n int) (full, short []string) {
	full = make([]string, n)
	short = make([]string, n)
	for i := 0; i < n; i++ {
		d := today.AddDate(0, 0, -(n - 1 - i))
		full[i] = render.DayLabel(d, today)
		short[i] = d.Format("02")
	}
	return
}

// peakSuffix annotates a chart title with the window's largest single-series day
// (" · peak <v>/day"), restoring a quantitative anchor for grouped bars whose
// heights no longer map to a printed per-column total. Empty when there's no data.
func peakSuffix(series []render.Series, format func(float64) string) string {
	var peak float64
	for _, s := range series {
		for _, v := range s.Values {
			if v > peak {
				peak = v
			}
		}
	}
	if peak <= 0 {
		return ""
	}
	return fmt.Sprintf(" · peak %s/day", format(peak))
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

func cleanModelSeriesNames(series []render.Series) {
	for i := range series {
		series[i].Name = displayModelName(series[i].Name)
	}
}

// printChartLegend keys the bar colors once at the top. A single series needs
// no key.
func printChartLegend(series []render.Series) {
	if len(series) < 2 {
		return
	}
	fmt.Print("  ")
	for i, s := range series {
		if i > 0 {
			fmt.Print("   ")
		}
		s.Color.Printf("██ %s", s.Name)
	}
	fmt.Println()
	fmt.Println()
}

func printChartSummary(series []render.Series, format func(float64) string) {
	n := seriesWindow(series)
	var grand float64
	for _, s := range series {
		sum := sumSeriesValues(s.Values)
		grand += sum
		chartSummaryRow(s.Color, s.Name, format(sum), format(sum/float64(maxInt(1, n))))
	}
	if len(series) > 1 {
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

func printChartBreakdown(providers []chartProvider, today time.Time) {
	render.Bold.Printf("Breakdown · last %d days\n", chartWindow(providers))
	for _, p := range providers {
		p.col.Printf("  %s\n", p.name)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "    Day\tInput\tOutput\tCache")
		for _, e := range p.filled {
			fmt.Fprintf(w, "    %s\t%s\t%s\t%s\n",
				render.DayLabel(e.Date, today),
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

func seriesWindow(series []render.Series) int {
	if len(series) == 0 {
		return 0
	}
	return len(series[0].Values)
}

func sumSeriesValues(values []float64) float64 {
	var s float64
	for _, v := range values {
		s += v
	}
	return s
}

func resetChartByModelFlag(cmd *cobra.Command) {
	flag := cmd.Flags().Lookup("by-model")
	if flag == nil {
		return
	}
	_ = flag.Value.Set("false")
	flag.Changed = false
}

func init() {
	chartCmd.Flags().BoolP("by-model", "m", false, "Split chart bars by model instead of tool")
	rootCmd.AddCommand(chartCmd)
}
