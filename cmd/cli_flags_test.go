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
)

func TestTodayWithExplicitDaysRendersDailyWindow(t *testing.T) {
	today := startOfDay(time.Now())
	out, err := runTokensWithCache(t, sampleUsageData(today), "today", "--days", "5")
	if err != nil {
		t.Fatalf("tokens today --days 5: %v\n%s", err, out)
	}

	start := today.AddDate(0, 0, -4).Format("Mon Jan 2")
	for _, want := range []string{"Last 5 days", start, "Today"} {
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
		Version:   1,
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
		usage.Daily = append(usage.Daily, entry)
		usage.Total.InputTokens += entry.InputTokens
		usage.Total.OutputTokens += entry.OutputTokens
		usage.Total.CacheTokens += entry.CacheTokens
		usage.Total.TotalTokens += entry.TotalTokens
		usage.Total.Cost += entry.Cost
	}
	return usage
}
