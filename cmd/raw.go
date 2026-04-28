package cmd

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

var rawCmd = &cobra.Command{
	Use:         "raw",
	Aliases:     []string{"table"},
	Short:       "Tabular daily breakdown — pipeable",
	Annotations: map[string]string{"group": groupData},
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
		filled := render.FillMissingDays(render.CombineDaily(data), today, days)

		claudeByDate := mapByDate(data.Claude)
		codexByDate := mapByDate(data.Codex)

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tCLAUDE TOKENS\tCLAUDE $\tCODEX TOKENS\tCODEX $\tTOTAL TOKENS\tTOTAL $")
		for _, e := range filled {
			key := e.Date.Format("2006-01-02")
			cE := claudeByDate[key]
			xE := codexByDate[key]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				key,
				render.FormatTokens(cE.TotalTokens),
				render.FormatCost(cE.Cost),
				render.FormatTokens(xE.TotalTokens),
				render.FormatCost(xE.Cost),
				render.FormatTokens(e.TotalTokens),
				render.FormatCost(e.Cost))
		}
		return w.Flush()
	},
}

func mapByDate(usage *ccusage.ToolUsage) map[string]ccusage.DailyEntry {
	m := map[string]ccusage.DailyEntry{}
	if usage == nil {
		return m
	}
	for _, d := range usage.Daily {
		m[d.Date.Format("2006-01-02")] = d
	}
	return m
}

func init() {
	rootCmd.AddCommand(rawCmd)
}
