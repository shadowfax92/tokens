package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/config"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	noCache    bool
	days       int
	detailed   bool
	cfg        *config.Config
)

const (
	groupViews    = "Views:"
	groupTools    = "Tools:"
	groupData     = "Data:"
	groupSettings = "Settings:"
)

var rootCmd = &cobra.Command{
	Use:           "tokens",
	Short:         "AI token usage stats — Claude Code + Codex",
	Long:          "View daily token consumption and cost for Claude Code and OpenAI Codex.\nRequires npx (Node.js) for the underlying ccusage packages.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		if days <= 0 {
			days = cfg.DefaultDays
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return dashboard()
	},
}

func dashboard() error {
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

	now := time.Now()
	today := startOfDay(now)

	render.Bold.Print("Token Usage")
	render.Dim.Printf("  ·  %s", today.Format("Mon Jan 2"))
	if res.FromCache {
		render.Dim.Printf("  ·  cached %s ago", humanizeDuration(res.CacheAge))
	}
	fmt.Println()
	fmt.Println()

	renderToday(data, today)
	renderTrends(data, today)
	renderSparklines(data, today, days)
	return nil
}

func renderToday(data *ccusage.UsageData, today time.Time) {
	render.Bold.Println("Today")

	type row struct {
		name  string
		entry ccusage.DailyEntry
		c     *color.Color
	}
	var rows []row
	if data.Claude != nil {
		rows = append(rows, row{"Claude Code", render.FindEntry(data.Claude.Daily, today), render.CyanBold})
	}
	if data.Codex != nil {
		rows = append(rows, row{"Codex", render.FindEntry(data.Codex.Daily, today), render.GreenBold})
	}

	for _, r := range rows {
		r.c.Printf("  %-13s", r.name)
		if r.entry.TotalTokens > 0 {
			fmt.Printf(" %10s   ", render.FormatTokens(r.entry.TotalTokens))
			render.Green.Printf("%9s", render.FormatCost(r.entry.Cost))
		} else {
			render.Dim.Printf(" %10s   %9s", "—", "—")
		}
		fmt.Println()
		if detailed && r.entry.TotalTokens > 0 {
			render.Dim.Printf("                in %s · out %s · cache %s\n",
				render.FormatTokens(r.entry.InputTokens),
				render.FormatTokens(r.entry.OutputTokens),
				render.FormatTokens(r.entry.CacheTokens))
		}
	}

	if len(rows) > 1 {
		var totalTokens int64
		var totalCost float64
		for _, r := range rows {
			totalTokens += r.entry.TotalTokens
			totalCost += r.entry.Cost
		}
		render.Dim.Printf("                %s   %s\n",
			strings.Repeat("─", 10),
			strings.Repeat("─", 9))
		render.Bold.Printf("  %-13s", "Total")
		fmt.Printf(" %10s   ", render.FormatTokens(totalTokens))
		render.GreenBold.Printf("%9s", render.FormatCost(totalCost))
		fmt.Println()
	}
	fmt.Println()
}

func renderTrends(data *ccusage.UsageData, today time.Time) {
	combined := render.CombineDaily(data)
	if len(combined) == 0 {
		return
	}

	yesterday := today.AddDate(0, 0, -1)
	todayE := render.FindEntry(combined, today)
	yE := render.FindEntry(combined, yesterday)
	thisWeek, lastWeek := render.WeekTotals(combined, today)

	filled := render.FillMissingDays(combined, today, days)
	var totalTok int64
	var totalCost float64
	for _, e := range filled {
		totalTok += e.TotalTokens
		totalCost += e.Cost
	}
	avgTok := totalTok / int64(maxInt(1, len(filled)))
	avgCost := totalCost / float64(maxInt(1, len(filled)))

	render.Bold.Println("Trends")
	printTrendRow("Day-over-Day",
		float64(todayE.TotalTokens), float64(yE.TotalTokens),
		todayE.Cost, yE.Cost)
	printTrendRow("Week-over-Week",
		float64(thisWeek.TotalTokens), float64(lastWeek.TotalTokens),
		thisWeek.Cost, lastWeek.Cost)

	fmt.Printf("  %-16s ", fmt.Sprintf("%d-day avg/day", days))
	render.Bold.Printf("%-10s   ", render.FormatTokens(avgTok))
	render.GreenBold.Printf("%9s\n", render.FormatCost(avgCost))
	fmt.Println()
}

func printTrendRow(label string, curTok, prevTok, curCost, prevCost float64) {
	tokPctC := render.PctColor(curTok, prevTok)
	costPctC := render.PctColor(curCost, prevCost)

	fmt.Printf("  %-16s ", label)
	tokPctC.Printf("%-10s   ", render.FormatPct(curTok, prevTok))
	costPctC.Printf("%9s\n", render.FormatPct(curCost, prevCost))
}

func renderSparklines(data *ccusage.UsageData, today time.Time, window int) {
	combined := render.CombineDaily(data)
	if len(combined) == 0 {
		return
	}
	filled := render.FillMissingDays(combined, today, window)

	tokVals := make([]float64, len(filled))
	costVals := make([]float64, len(filled))
	for i, e := range filled {
		tokVals[i] = float64(e.TotalTokens)
		costVals[i] = e.Cost
	}

	first := filled[0].Date.Format("Jan 02")
	last := "Today"
	pad := strings.Repeat(" ", maxInt(1, len(filled)-len(first)-len(last)))

	render.Bold.Printf("Last %d days\n", window)
	render.Cyan.Printf("  %s", render.Sparkline(tokVals))
	render.Dim.Printf("   tokens\n")
	render.Green.Printf("  %s", render.Sparkline(costVals))
	render.Dim.Printf("   cost\n")
	render.Dim.Printf("  %s%s%s\n", first, pad, last)
	fmt.Println()
}

func fetchWithSpinner() *ccusage.FetchResult {
	opts := ccusage.FetchOptions{
		NoCache:  noCache,
		CacheTTL: time.Duration(cfg.CacheTTLMinutes) * time.Minute,
	}

	if jsonOutput {
		return ccusage.Fetch(opts)
	}

	stop := startSpinner("Fetching usage data from ccusage and @ccusage/codex...")
	res := ccusage.Fetch(opts)
	stop()

	if !res.FromCache {
		render.Dim.Fprintf(os.Stderr, "Fetched in %s.\n", humanizeDuration(res.FetchTook))
	}
	return res
}

func printErrors(data *ccusage.UsageData) {
	if data == nil {
		return
	}
	for _, e := range data.Errors {
		color.New(color.FgYellow).Fprintf(os.Stderr, "  warning: %s\n", e)
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func humanizeDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func daysFlagChanged(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if flag := cmd.Flags().Lookup("days"); flag != nil {
		return flag.Changed
	}
	if flag := cmd.Root().PersistentFlags().Lookup("days"); flag != nil {
		return flag.Changed
	}
	return false
}

const usageTemplate = `{{helpHeader "Usage:"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{helpHeader "Aliases:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{helpHeader "Examples:"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}
{{groupedHelp .}}{{end}}{{if .HasAvailableLocalFlags}}

{{helpHeader "Flags:"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{helpHeader "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

{{helpHint (printf "Use \"%s [command] --help\" for more information." .CommandPath)}}{{end}}
`

var (
	helpHeaderColor = color.New(color.Bold, color.FgCyan)
	helpCmdColor    = color.New(color.FgHiGreen)
	helpAliasColor  = color.New(color.FgYellow)
	helpHintColor   = color.New(color.Faint)
)

func helpHeader(s string) string { return helpHeaderColor.Sprint(s) }
func helpCmdCol(s string) string { return helpCmdColor.Sprint(s) }
func helpHint(s string) string   { return helpHintColor.Sprint(s) }
func helpAliases(aliases []string) string {
	return helpAliasColor.Sprintf("(aliases: %s)", strings.Join(aliases, ", "))
}

var groupOrder = []string{groupViews, groupTools, groupData, groupSettings, "Other:"}

func groupedHelp(cmd *cobra.Command) string {
	groups := map[string][]*cobra.Command{}
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() && c.Name() != "help" {
			continue
		}
		g := c.Annotations["group"]
		if g == "" {
			g = "Other:"
		}
		groups[g] = append(groups[g], c)
	}
	var b strings.Builder
	for _, name := range groupOrder {
		cmds, ok := groups[name]
		if !ok {
			continue
		}
		b.WriteString("\n" + helpHeader(name) + "\n")
		for _, c := range cmds {
			line := "  " + helpCmdCol(fmt.Sprintf("%-12s", c.Name())) + " " + c.Short
			if len(c.Aliases) > 0 {
				line += " " + helpAliases(c.Aliases)
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func init() {
	cobra.AddTemplateFunc("helpHeader", helpHeader)
	cobra.AddTemplateFunc("helpCmdCol", helpCmdCol)
	cobra.AddTemplateFunc("helpAliases", helpAliases)
	cobra.AddTemplateFunc("helpHint", helpHint)
	cobra.AddTemplateFunc("groupedHelp", groupedHelp)

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "Bypass cache, force re-fetch")
	rootCmd.PersistentFlags().IntVar(&days, "days", 0, "Window in days for charts, raw tables, sparklines, and explicit daily/growth views (default 14)")
	rootCmd.PersistentFlags().BoolVarP(&detailed, "detailed", "d", false, "Show input/output/cache breakdown")
	rootCmd.SetUsageTemplate(usageTemplate)
}

func Execute() error {
	return rootCmd.Execute()
}
