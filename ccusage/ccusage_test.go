package ccusage

import (
	"math"
	"testing"
)

// codex daily JSON as emitted by `ccusage codex daily --breakdown --json` (current shape).
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
      "costUSD": 0.370794,
      "models": {
        "gpt-5.5": {
          "inputTokens": 29748,
          "cacheCreationTokens": 1024,
          "cacheReadTokens": 436096,
          "outputTokens": 9370,
          "reasoningOutputTokens": 4608,
          "totalTokens": 475214,
          "isFallback": false
        }
      }
    },
    {
      "date": "2025-09-11",
      "inputTokens": 105764,
      "cacheCreationTokens": 0,
      "cacheReadTokens": 610944,
      "outputTokens": 9608,
      "reasoningOutputTokens": 6720,
      "totalTokens": 726316,
      "costUSD": 0.609306,
      "models": {
        "gpt-5.5": {
          "inputTokens": 55764,
          "cacheCreationTokens": 0,
          "cacheReadTokens": 464944,
          "outputTokens": 5608,
          "reasoningOutputTokens": 0,
          "totalTokens": 526316,
          "isFallback": false
        },
        "gpt-5.5-mini": {
          "inputTokens": 50000,
          "cacheCreationTokens": 0,
          "cacheReadTokens": 146000,
          "outputTokens": 4000,
          "reasoningOutputTokens": 6720,
          "totalTokens": 200000,
          "isFallback": true
        }
      }
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
	if len(day0.Models) != 1 {
		t.Fatalf("len(day0.Models) = %d, want 1", len(day0.Models))
	}
	gpt := findModel(t, day0.Models, "gpt-5.5")
	if gpt.TotalTokens != day0.TotalTokens {
		t.Errorf("single-model total = %d, want day total %d", gpt.TotalTokens, day0.TotalTokens)
	}
	if !approxEqual(gpt.Cost, day0.Cost) {
		t.Errorf("single-model cost = %v, want day cost %v", gpt.Cost, day0.Cost)
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

	day1 := usage.Daily[1]
	if len(day1.Models) != 2 {
		t.Fatalf("len(day1.Models) = %d, want 2", len(day1.Models))
	}
	mini := findModel(t, day1.Models, "gpt-5.5-mini")
	if mini.InputTokens != 50000 {
		t.Errorf("mini input = %d, want 50000", mini.InputTokens)
	}
	if mini.OutputTokens != 4000 {
		t.Errorf("mini output = %d, want 4000", mini.OutputTokens)
	}
	if mini.CacheTokens != 146000 {
		t.Errorf("mini cache = %d, want 146000", mini.CacheTokens)
	}
	if mini.TotalTokens != 200000 {
		t.Errorf("mini total = %d, want 200000", mini.TotalTokens)
	}
	wantMiniCost := day1.Cost * float64(200000) / float64(day1.Models[0].TotalTokens+day1.Models[1].TotalTokens)
	if !approxEqual(mini.Cost, wantMiniCost) {
		t.Errorf("mini cost = %v, want %v", mini.Cost, wantMiniCost)
	}
}

// claude daily JSON as emitted by `ccusage claude daily --breakdown --json`.
const claudeDailyJSON = `{
  "daily": [
    {
      "date": "2026-04-07",
      "inputTokens": 37,
      "cacheCreationTokens": 65210,
      "cacheReadTokens": 1307368,
      "outputTokens": 11654,
      "totalCost": 0.9583914,
      "totalTokens": 1384269,
      "modelBreakdowns": [
        {
          "modelName": "claude-fable-5",
          "inputTokens": 10,
          "cacheCreationTokens": 30,
          "cacheReadTokens": 40,
          "outputTokens": 20,
          "cost": 0.123
        },
        {
          "modelName": "claude-haiku-4-5-20251001",
          "inputTokens": 27,
          "cacheCreationTokens": 65180,
          "cacheReadTokens": 1307328,
          "outputTokens": 11634,
          "cost": 0.8353914
        }
      ]
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
	if len(day.Models) != 2 {
		t.Fatalf("len(Models) = %d, want 2", len(day.Models))
	}
	fable := findModel(t, day.Models, "claude-fable-5")
	if fable.InputTokens != 10 {
		t.Errorf("fable input = %d, want 10", fable.InputTokens)
	}
	if fable.OutputTokens != 20 {
		t.Errorf("fable output = %d, want 20", fable.OutputTokens)
	}
	if fable.CacheTokens != 70 {
		t.Errorf("fable cache = %d, want 70", fable.CacheTokens)
	}
	if fable.TotalTokens != 100 {
		t.Errorf("fable total = %d, want 100", fable.TotalTokens)
	}
	if !approxEqual(fable.Cost, 0.123) {
		t.Errorf("fable cost = %v, want 0.123", fable.Cost)
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

func findModel(t *testing.T, models []ModelEntry, name string) ModelEntry {
	t.Helper()
	for _, m := range models {
		if m.Model == name {
			return m
		}
	}
	t.Fatalf("model %q not found in %#v", name, models)
	return ModelEntry{}
}
