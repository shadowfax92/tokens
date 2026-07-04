package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/render"
)

func TestRootDashboardWithExplicitDaysShowsWindowTotal(t *testing.T) {
	today := startOfDay(time.Now())
	data := sampleUsageData(today)
	out, err := runTokensWithCache(t, data, "--days", "10")
	if err != nil {
		t.Fatalf("tokens --days 10: %v\n%s", err, out)
	}

	current, _ := render.WindowTotals(render.CombineDaily(data), today, 10)
	for _, want := range []string{"10-day total", render.FormatTokens(current.TotalTokens), render.FormatCost(current.Cost)} {
		if !strings.Contains(out, want) {
			t.Fatalf("dashboard output missing %q:\n%s", want, out)
		}
	}
}

func TestTodayWithExplicitDaysRendersDailyWindow(t *testing.T) {
	today := startOfDay(time.Now())
	data := sampleUsageData(today)
	out, err := runTokensWithCache(t, data, "today", "--days", "5")
	if err != nil {
		t.Fatalf("tokens today --days 5: %v\n%s", err, out)
	}

	start := today.AddDate(0, 0, -4).Format("Mon Jan 2")
	current, _ := render.WindowTotals(render.CombineDaily(data), today, 5)
	for _, want := range []string{"Last 5 days", start, "Today", "Window total", render.FormatTokens(current.TotalTokens), render.FormatCost(current.Cost)} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Today ·") {
		t.Fatalf("expected window output, got single-day header:\n%s", out)
	}
	if got := strings.Count(out, "Total"); got < 5 {
		t.Fatalf("expected a daily total for each day, got %d totals:\n%s", got, out)
	}
}

func TestToolDeepDiveShowsWindowTotal(t *testing.T) {
	today := startOfDay(time.Now())
	data := sampleUsageData(today)
	out, err := runTokensWithCache(t, data, "codex", "--days", "5")
	if err != nil {
		t.Fatalf("tokens codex --days 5: %v\n%s", err, out)
	}

	current, _ := render.WindowTotals(data.Codex.Daily, today, 5)
	for _, want := range []string{"Total " + render.FormatTokens(current.TotalTokens), render.FormatCost(current.Cost), "avg "} {
		if !strings.Contains(out, want) {
			t.Fatalf("tool output missing %q:\n%s", want, out)
		}
	}
}

func TestGrowthWithExplicitDaysComparesMatchingWindow(t *testing.T) {
	today := startOfDay(time.Now())
	out, err := runTokensWithCache(t, sampleUsageData(today), "growth", "--days", "14")
	if err != nil {
		t.Fatalf("tokens growth --days 14: %v\n%s", err, out)
	}

	for _, want := range []string{"Last 14 days", "Previous 14 days"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Day-over-Day") || strings.Contains(out, "Week-over-Week") || strings.Contains(out, "Month-over-Month") {
		t.Fatalf("expected explicit --days to use a window comparison, got fixed calendar periods:\n%s", out)
	}
}

func TestChartDetailedAddsTokenBreakdown(t *testing.T) {
	today := startOfDay(time.Now())
	data := sampleUsageData(today)

	plain, err := runTokensWithCache(t, data, "chart", "--days", "3")
	if err != nil {
		t.Fatalf("tokens chart --days 3: %v\n%s", err, plain)
	}
	if strings.Contains(plain, "Breakdown · last 3 days") {
		t.Fatalf("plain chart should not include detailed breakdown:\n%s", plain)
	}

	detailedOut, err := runTokensWithCache(t, data, "chart", "--days", "3", "-d")
	if err != nil {
		t.Fatalf("tokens chart --days 3 -d: %v\n%s", err, detailedOut)
	}
	for _, want := range []string{"Breakdown · last 3 days", "Input", "Output", "Cache"} {
		if !strings.Contains(detailedOut, want) {
			t.Fatalf("detailed chart output missing %q:\n%s", want, detailedOut)
		}
	}
}

func TestChartBreaksDownByProvider(t *testing.T) {
	today := startOfDay(time.Now())
	out, err := runTokensWithCache(t, sampleUsageData(today), "chart", "--days", "5")
	if err != nil {
		t.Fatalf("tokens chart --days 5: %v\n%s", err, out)
	}
	for _, want := range []string{"Tokens · last 5 days", "Cost · last 5 days", "Claude Code", "Codex", "Total", "peak "} {
		if !strings.Contains(out, want) {
			t.Fatalf("chart output missing %q:\n%s", want, out)
		}
	}
}

func TestChartSingleProviderOmitsAbsentTool(t *testing.T) {
	today := startOfDay(time.Now())
	data := &ccusage.UsageData{Claude: sampleTool("Claude Code", today, 1_000_000)}

	out, err := runTokensWithCache(t, data, "chart", "--days", "5")
	if err != nil {
		t.Fatalf("tokens chart (claude only): %v\n%s", err, out)
	}
	if !strings.Contains(out, "Claude Code") {
		t.Fatalf("expected Claude Code in single-provider output:\n%s", out)
	}
	if strings.Contains(out, "Codex") {
		t.Fatalf("did not expect Codex chrome with no Codex data:\n%s", out)
	}
	if strings.Contains(out, "Total") {
		t.Fatalf("did not expect a Total line for a single provider:\n%s", out)
	}
	for _, want := range []string{"Tokens · last 5 days", "Cost · last 5 days"} {
		if !strings.Contains(out, want) {
			t.Fatalf("chart output missing %q:\n%s", want, out)
		}
	}
}

func TestChartDetailedBreakdownIsPerProvider(t *testing.T) {
	today := startOfDay(time.Now())
	out, err := runTokensWithCache(t, sampleUsageData(today), "chart", "--days", "3", "-d")
	if err != nil {
		t.Fatalf("tokens chart --days 3 -d: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Breakdown · last 3 days") {
		t.Fatalf("missing breakdown header:\n%s", out)
	}
	// one Input/Output/Cache table per provider → the column header repeats.
	if got := strings.Count(out, "Input"); got < 2 {
		t.Fatalf("expected a breakdown table per provider (>=2 Input headers), got %d:\n%s", got, out)
	}
}

func TestByModelRendersModelsAndDetailedRows(t *testing.T) {
	today := startOfDay(time.Now())
	data := sampleUsageData(today)

	out, err := runTokensWithCache(t, data, "by-model", "--days", "5")
	if err != nil {
		t.Fatalf("tokens by-model --days 5: %v\n%s", err, out)
	}
	start := today.AddDate(0, 0, -4).Format("Mon Jan 2")
	for _, want := range []string{"By model · last 5 days", start, "Claude Code", "Codex", "fable-5", "haiku-4-5", "gpt-5.5", "subtotal", "Total"} {
		if !strings.Contains(out, want) {
			t.Fatalf("by-model output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "claude-fable-5") || strings.Contains(out, "20251001") {
		t.Fatalf("display output should use cleaned Claude model names:\n%s", out)
	}

	detailedOut, err := runTokensWithCache(t, data, "models", "--days", "5", "-d")
	if err != nil {
		t.Fatalf("tokens models --days 5 -d: %v\n%s", err, detailedOut)
	}
	for _, want := range []string{"By model · last 5 days", "in ", "out ", "cache "} {
		if !strings.Contains(detailedOut, want) {
			t.Fatalf("detailed by-model output missing %q:\n%s", want, detailedOut)
		}
	}
}

func TestByModelSingleProviderOmitsGrandTotal(t *testing.T) {
	today := startOfDay(time.Now())
	data := &ccusage.UsageData{Claude: sampleTool("Claude Code", today, 1_000_000)}

	out, err := runTokensWithCache(t, data, "by-model", "--days", "5")
	if err != nil {
		t.Fatalf("tokens by-model (claude only): %v\n%s", err, out)
	}
	if !strings.Contains(out, "Claude Code") || !strings.Contains(out, "subtotal") {
		t.Fatalf("expected Claude section and subtotal:\n%s", out)
	}
	if strings.Contains(out, "Codex") {
		t.Fatalf("did not expect absent Codex section:\n%s", out)
	}
	if strings.Contains(out, "Total") {
		t.Fatalf("did not expect grand Total for a single provider:\n%s", out)
	}
}

func TestByModelJSONKeepsRawModelNames(t *testing.T) {
	today := startOfDay(time.Now())
	out, err := runTokensWithCache(t, sampleUsageData(today), "by-model", "--json")
	if err != nil {
		t.Fatalf("tokens by-model --json: %v\n%s", err, out)
	}
	for _, want := range []string{`"models"`, `"model": "claude-fable-5"`, `"model": "claude-haiku-4-5-20251001"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON output missing raw model field %q:\n%s", want, out)
		}
	}
}

func runTokensWithCache(t *testing.T, data *ccusage.UsageData, args ...string) (string, error) {
	t.Helper()
	resetCommandState(t)

	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	writeTestCache(t, data)

	oldStdout := os.Stdout
	oldColorOutput := color.Output
	oldNoColor := color.NoColor
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = w
	color.Output = w
	color.NoColor = true
	rootCmd.SetArgs(args)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	readDone := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		readDone <- buf.String()
	}()

	execErr := Execute()
	_ = w.Close()
	out := <-readDone

	os.Stdout = oldStdout
	color.Output = oldColorOutput
	color.NoColor = oldNoColor
	_ = r.Close()

	return out, execErr
}

func resetCommandState(t *testing.T) {
	t.Helper()

	for name, value := range map[string]string{
		"json":     "false",
		"no-cache": "false",
		"days":     "0",
		"detailed": "false",
	} {
		flag := rootCmd.PersistentFlags().Lookup(name)
		if flag == nil {
			t.Fatalf("missing persistent flag %q", name)
		}
		if err := flag.Value.Set(value); err != nil {
			t.Fatalf("reset flag %q: %v", name, err)
		}
		flag.Changed = false
	}

	jsonOutput = false
	noCache = false
	days = 0
	detailed = false
	cfg = nil
	rootCmd.SetArgs(nil)
}

func writeTestCache(t *testing.T, data *ccusage.UsageData) {
	t.Helper()

	cacheFile := struct {
		Version   int                `json:"version"`
		FetchedAt time.Time          `json:"fetched_at"`
		Data      *ccusage.UsageData `json:"data"`
	}{
		Version:   2,
		FetchedAt: time.Now(),
		Data:      data,
	}

	path := ccusage.CachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func sampleUsageData(today time.Time) *ccusage.UsageData {
	return &ccusage.UsageData{
		Claude: sampleTool("Claude Code", today, 1_000_000),
		Codex:  sampleTool("Codex", today, 500_000),
	}
}

func sampleTool(name string, today time.Time, base int64) *ccusage.ToolUsage {
	usage := &ccusage.ToolUsage{Tool: name}
	for i := 27; i >= 0; i-- {
		date := today.AddDate(0, 0, -i)
		multiplier := int64(28 - i)
		entry := ccusage.DailyEntry{
			Date:         date,
			InputTokens:  base * multiplier,
			OutputTokens: base * multiplier / 2,
			CacheTokens:  base * multiplier / 4,
			Cost:         float64(multiplier) * 1.25,
		}
		entry.TotalTokens = entry.InputTokens + entry.OutputTokens + entry.CacheTokens
		entry.Models = sampleModels(name, entry)
		usage.Daily = append(usage.Daily, entry)
		usage.Total.InputTokens += entry.InputTokens
		usage.Total.OutputTokens += entry.OutputTokens
		usage.Total.CacheTokens += entry.CacheTokens
		usage.Total.TotalTokens += entry.TotalTokens
		usage.Total.Cost += entry.Cost
	}
	return usage
}

func sampleModels(tool string, entry ccusage.DailyEntry) []ccusage.ModelEntry {
	halfInput := entry.InputTokens / 2
	halfOutput := entry.OutputTokens / 2
	halfCache := entry.CacheTokens / 2
	halfTotal := halfInput + halfOutput + halfCache
	halfCost := entry.Cost / 2

	if tool == "Claude Code" {
		return []ccusage.ModelEntry{
			{
				Model:        "claude-fable-5",
				InputTokens:  halfInput,
				OutputTokens: halfOutput,
				CacheTokens:  halfCache,
				TotalTokens:  halfTotal,
				Cost:         halfCost,
			},
			{
				Model:        "claude-haiku-4-5-20251001",
				InputTokens:  entry.InputTokens - halfInput,
				OutputTokens: entry.OutputTokens - halfOutput,
				CacheTokens:  entry.CacheTokens - halfCache,
				TotalTokens:  entry.TotalTokens - halfTotal,
				Cost:         entry.Cost - halfCost,
			},
		}
	}

	return []ccusage.ModelEntry{
		{
			Model:        "gpt-5.5",
			InputTokens:  entry.InputTokens,
			OutputTokens: entry.OutputTokens,
			CacheTokens:  entry.CacheTokens,
			TotalTokens:  entry.TotalTokens,
			Cost:         entry.Cost,
		},
	}
}
