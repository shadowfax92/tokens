package ccusage

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type DailyEntry struct {
	Date         time.Time `json:"date"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	CacheTokens  int64     `json:"cache_tokens"`
	TotalTokens  int64     `json:"total_tokens"`
	Cost         float64   `json:"cost"`
}

type ToolUsage struct {
	Tool  string       `json:"tool"`
	Daily []DailyEntry `json:"daily"`
	Total DailyEntry   `json:"total"`
}

type UsageData struct {
	Claude *ToolUsage `json:"claude,omitempty"`
	Codex  *ToolUsage `json:"codex,omitempty"`
	Errors []string   `json:"errors,omitempty"`
}

type claudeResponse struct {
	Daily []struct {
		Date                string  `json:"date"`
		InputTokens         int64   `json:"inputTokens"`
		OutputTokens        int64   `json:"outputTokens"`
		CacheCreationTokens int64   `json:"cacheCreationTokens"`
		CacheReadTokens     int64   `json:"cacheReadTokens"`
		TotalTokens         int64   `json:"totalTokens"`
		TotalCost           float64 `json:"totalCost"`
	} `json:"daily"`
}

type codexResponse struct {
	Daily []struct {
		Date                  string  `json:"date"`
		InputTokens           int64   `json:"inputTokens"`
		CachedInputTokens     int64   `json:"cachedInputTokens"`
		OutputTokens          int64   `json:"outputTokens"`
		ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
		TotalTokens           int64   `json:"totalTokens"`
		CostUSD               float64 `json:"costUSD"`
	} `json:"daily"`
}

func runNpx(args ...string) ([]byte, error) {
	cmd := exec.Command("npx", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("npx %v failed: %s", args, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("npx %v: %w", args, err)
	}
	return out, nil
}

func fetchClaude() (*ToolUsage, error) {
	out, err := runNpx("ccusage@latest", "daily", "--json")
	if err != nil {
		return nil, err
	}

	var resp claudeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse ccusage JSON: %w", err)
	}

	usage := &ToolUsage{Tool: "Claude Code"}
	for _, d := range resp.Daily {
		t, err := parseLocalDate(d.Date)
		if err != nil {
			continue
		}
		entry := DailyEntry{
			Date:         t,
			InputTokens:  d.InputTokens,
			OutputTokens: d.OutputTokens,
			CacheTokens:  d.CacheCreationTokens + d.CacheReadTokens,
			TotalTokens:  d.TotalTokens,
			Cost:         d.TotalCost,
		}
		usage.Daily = append(usage.Daily, entry)
		usage.Total.InputTokens += entry.InputTokens
		usage.Total.OutputTokens += entry.OutputTokens
		usage.Total.CacheTokens += entry.CacheTokens
		usage.Total.TotalTokens += entry.TotalTokens
		usage.Total.Cost += entry.Cost
	}

	return usage, nil
}

func fetchCodex() (*ToolUsage, error) {
	out, err := runNpx("@ccusage/codex@latest", "daily", "--json", "--locale", "en-CA")
	if err != nil {
		return nil, err
	}

	var resp codexResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse codex JSON: %w", err)
	}

	usage := &ToolUsage{Tool: "Codex"}
	for _, d := range resp.Daily {
		t, err := parseLocalDate(d.Date)
		if err != nil {
			continue
		}
		entry := DailyEntry{
			Date:         t,
			InputTokens:  d.InputTokens,
			OutputTokens: d.OutputTokens,
			CacheTokens:  d.CachedInputTokens,
			TotalTokens:  d.TotalTokens,
			Cost:         d.CostUSD,
		}
		usage.Daily = append(usage.Daily, entry)
		usage.Total.InputTokens += entry.InputTokens
		usage.Total.OutputTokens += entry.OutputTokens
		usage.Total.CacheTokens += entry.CacheTokens
		usage.Total.TotalTokens += entry.TotalTokens
		usage.Total.Cost += entry.Cost
	}

	return usage, nil
}

func parseLocalDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "Jan 2, 2006", "January 2, 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %s", s)
}

func fetchAll() *UsageData {
	data := &UsageData{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(2)
	go func() {
		defer wg.Done()
		claude, err := fetchClaude()
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			data.Errors = append(data.Errors, fmt.Sprintf("Claude Code: %v", err))
		} else {
			data.Claude = claude
		}
	}()

	go func() {
		defer wg.Done()
		codex, err := fetchCodex()
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			data.Errors = append(data.Errors, fmt.Sprintf("Codex: %v", err))
		} else {
			data.Codex = codex
		}
	}()

	wg.Wait()
	return data
}
