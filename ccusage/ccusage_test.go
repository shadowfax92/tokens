package ccusage

import (
	"math"
	"testing"
)

// codex daily JSON as emitted by `ccusage codex daily --json` (current shape).
// cacheCreationTokens is non-zero here on purpose so the creation+read sum is
// verifiable — a regression back to the old `cachedInputTokens` field would
// read 0 and fail these assertions.
const codexDailyJSON = `{
  "daily": [
    {
      "date": "2025-09-10",
      "inputTokens": 29748,
      "cacheCreationTokens": 1024,
      "cacheReadTokens": 436096,
      "outputTokens": 9370,
      "reasoningOutputTokens": 4608,
      "totalTokens": 475214,
      "costUSD": 0.370794
    },
    {
      "date": "2025-09-11",
      "inputTokens": 105764,
      "cacheCreationTokens": 0,
      "cacheReadTokens": 610944,
      "outputTokens": 9608,
      "reasoningOutputTokens": 6720,
      "totalTokens": 726316,
      "costUSD": 0.609306
    }
  ],
  "totals": {}
}`

func TestParseCodexDaily(t *testing.T) {
	usage, err := parseCodex([]byte(codexDailyJSON))
	if err != nil {
		t.Fatalf("parseCodex: %v", err)
	}
	if usage.Tool != "Codex" {
		t.Fatalf("Tool = %q, want Codex", usage.Tool)
	}
	if len(usage.Daily) != 2 {
		t.Fatalf("len(Daily) = %d, want 2", len(usage.Daily))
	}

	day0 := usage.Daily[0]
	if got := day0.Date.Format("2006-01-02"); got != "2025-09-10" {
		t.Errorf("day0 date = %q, want 2025-09-10", got)
	}
	if day0.InputTokens != 29748 {
		t.Errorf("day0 input = %d, want 29748", day0.InputTokens)
	}
	if day0.OutputTokens != 9370 {
		t.Errorf("day0 output = %d, want 9370", day0.OutputTokens)
	}
	// cache = cacheCreationTokens + cacheReadTokens = 1024 + 436096
	if day0.CacheTokens != 437120 {
		t.Errorf("day0 cache = %d, want 437120 (creation+read)", day0.CacheTokens)
	}
	if day0.TotalTokens != 475214 {
		t.Errorf("day0 total = %d, want 475214", day0.TotalTokens)
	}
	if !approxEqual(day0.Cost, 0.370794) {
		t.Errorf("day0 cost = %v, want 0.370794", day0.Cost)
	}

	if usage.Total.CacheTokens != 437120+610944 {
		t.Errorf("total cache = %d, want %d", usage.Total.CacheTokens, 437120+610944)
	}
	if usage.Total.TotalTokens != 475214+726316 {
		t.Errorf("total tokens = %d, want %d", usage.Total.TotalTokens, 475214+726316)
	}
	if !approxEqual(usage.Total.Cost, 0.370794+0.609306) {
		t.Errorf("total cost = %v, want %v", usage.Total.Cost, 0.370794+0.609306)
	}
}

// claude daily JSON as emitted by `ccusage claude daily --json`.
const claudeDailyJSON = `{
  "daily": [
    {
      "date": "2026-04-07",
      "inputTokens": 37,
      "cacheCreationTokens": 65210,
      "cacheReadTokens": 1307368,
      "outputTokens": 11654,
      "totalCost": 0.9583914,
      "totalTokens": 1384269
    }
  ]
}`

func TestParseClaudeDaily(t *testing.T) {
	usage, err := parseClaude([]byte(claudeDailyJSON))
	if err != nil {
		t.Fatalf("parseClaude: %v", err)
	}
	if usage.Tool != "Claude Code" {
		t.Fatalf("Tool = %q, want Claude Code", usage.Tool)
	}
	if len(usage.Daily) != 1 {
		t.Fatalf("len(Daily) = %d, want 1", len(usage.Daily))
	}
	day := usage.Daily[0]
	if got := day.Date.Format("2006-01-02"); got != "2026-04-07" {
		t.Errorf("date = %q, want 2026-04-07", got)
	}
	if day.CacheTokens != 65210+1307368 {
		t.Errorf("cache = %d, want %d (creation+read)", day.CacheTokens, 65210+1307368)
	}
	if !approxEqual(day.Cost, 0.9583914) {
		t.Errorf("cost = %v, want 0.9583914", day.Cost)
	}
}

// The bare `ccusage daily --json` command aggregates every detected agent and
// keys each row by `period`, not `date`. parseClaude must yield no entries from
// that shape — this is precisely why fetchClaude uses the `claude` subcommand.
// If anyone repoints it at bare `daily`, this guard documents the silent-empty
// failure mode.
const aggregatedDailyJSON = `{
  "daily": [
    {
      "agent": "all",
      "period": "2025-08-28",
      "inputTokens": 37,
      "cacheCreationTokens": 41174,
      "cacheReadTokens": 139857,
      "outputTokens": 2147,
      "totalCost": 0.7354092,
      "totalTokens": 183215
    }
  ]
}`

func TestParseClaudeIgnoresAggregatedPeriodShape(t *testing.T) {
	usage, err := parseClaude([]byte(aggregatedDailyJSON))
	if err != nil {
		t.Fatalf("parseClaude: %v", err)
	}
	if len(usage.Daily) != 0 {
		t.Fatalf("len(Daily) = %d, want 0 (period rows lack a usable date)", len(usage.Daily))
	}
}

func TestParseCodexEmptyDaily(t *testing.T) {
	usage, err := parseCodex([]byte(`{"daily": []}`))
	if err != nil {
		t.Fatalf("parseCodex: %v", err)
	}
	if len(usage.Daily) != 0 {
		t.Fatalf("len(Daily) = %d, want 0", len(usage.Daily))
	}
	if usage.Total.TotalTokens != 0 {
		t.Fatalf("total = %d, want 0", usage.Total.TotalTokens)
	}
}

// The deprecated @ccusage/codex package prints "use npx ccusage instead" — make
// sure non-JSON output surfaces as an error rather than empty-but-fine data.
func TestParseCodexRejectsNonJSON(t *testing.T) {
	if _, err := parseCodex([]byte("use npx ccusage instead\n")); err == nil {
		t.Fatal("expected parse error for non-JSON output, got nil")
	}
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
