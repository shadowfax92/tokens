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

const byModelLabelWidth = 24

var byModelCmd = &cobra.Command{
	Use:         "by-model",
	Aliases:     []string{"models"},
	Short:       "Per-model token + cost breakdown",
	Annotations: map[string]string{"group": groupViews},
	RunE: func(cmd *cobra.Command, args []string) error {
		res := fetchWithSpinner()
		if jsonOutput {
			return emitJSON(res)
		}

		data := res.Data
		if data == nil || (data.Claude == nil && data.Codex == nil) {
			printErrors(data)
			return fmt.Errorf("could not fetch usage data")
		}
		printErrors(data)

		renderByModel(data, startOfDay(time.Now()), days)
		return nil
	},
}

func renderByModel(data *ccusage.UsageData, today time.Time, days int) {
	start := today.AddDate(0, 0, -(days - 1))
	render.Dim.Printf("By model · last %d days · %s → %s\n",
		days, start.Format("Mon Jan 2"), today.Format("Mon Jan 2"))

	type provider struct {
		name  string
		usage *ccusage.ToolUsage
		color *color.Color
	}
	var providers []provider
	if data.Claude != nil {
		providers = append(providers, provider{"Claude Code", data.Claude, render.CyanBold})
	}
	if data.Codex != nil {
		providers = append(providers, provider{"Codex", data.Codex, render.MagentaBold})
	}

	var grand ccusage.ModelEntry
	for i, p := range providers {
		if i > 0 {
			fmt.Println()
		}
		p.color.Println(p.name)
		subtotal := renderModelSection(p.usage, today, days)
		addModelEntry(&grand, subtotal)
	}

	if len(providers) > 1 {
		fmt.Println()
		printByModelRule()
		render.Bold.Printf("  %-24s", "Total")
		fmt.Printf(" %10s   ", render.FormatTokens(grand.TotalTokens))
		render.GreenBold.Printf("%9s\n", render.FormatCost(grand.Cost))
	}
}

func renderModelSection(usage *ccusage.ToolUsage, today time.Time, days int) ccusage.ModelEntry {
	models := render.ModelTotals(usage, today, days)
	var subtotal ccusage.ModelEntry
	for _, model := range models {
		printByModelRow(displayModelName(model.Model), model)
		if detailed && model.TotalTokens > 0 {
			render.Dim.Printf("  %-24s in %s · out %s · cache %s\n",
				"",
				render.FormatTokens(model.InputTokens),
				render.FormatTokens(model.OutputTokens),
				render.FormatTokens(model.CacheTokens))
		}
		addModelEntry(&subtotal, model)
	}

	printByModelRule()
	render.Bold.Printf("  %-24s", "subtotal")
	fmt.Printf(" %10s   ", render.FormatTokens(subtotal.TotalTokens))
	render.GreenBold.Printf("%9s\n", render.FormatCost(subtotal.Cost))
	return subtotal
}

func printByModelRow(model string, entry ccusage.ModelEntry) {
	fmt.Printf("  %-24s %10s   ", model, render.FormatTokens(entry.TotalTokens))
	render.Green.Printf("%9s\n", render.FormatCost(entry.Cost))
}

func printByModelRule() {
	render.Dim.Printf("  %-24s %s   %s\n",
		"",
		strings.Repeat("─", 10),
		strings.Repeat("─", 9))
}

func addModelEntry(total *ccusage.ModelEntry, entry ccusage.ModelEntry) {
	total.InputTokens += entry.InputTokens
	total.OutputTokens += entry.OutputTokens
	total.CacheTokens += entry.CacheTokens
	total.TotalTokens += entry.TotalTokens
	total.Cost += entry.Cost
}

func displayModelName(model string) string {
	out := strings.TrimPrefix(model, "claude-")
	lastDash := strings.LastIndex(out, "-")
	if lastDash == -1 {
		return out
	}
	suffix := out[lastDash+1:]
	if len(suffix) == 8 && allDigits(suffix) {
		return out[:lastDash]
	}
	return out
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func init() {
	rootCmd.AddCommand(byModelCmd)
}
