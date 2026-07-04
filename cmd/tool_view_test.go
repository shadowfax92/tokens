package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/nickhudkins/tokens/ccusage"
)

func TestToolDeepDiveRendersModelBreakdownWithCleanNames(t *testing.T) {
	today := startOfDay(time.Now())
	data := sampleUsageData(today)

	out, err := runTokensWithCache(t, data, "claude", "--days", "5")
	if err != nil {
		t.Fatalf("tokens claude --days 5: %v\n%s", err, out)
	}
	for _, want := range []string{"Models · last 5 days", "fable-5", "haiku-4-5"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Claude deep-dive output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "claude-fable-5") || strings.Contains(out, "20251001") {
		t.Fatalf("Claude deep-dive output should use cleaned model names:\n%s", out)
	}

	detailedOut, err := runTokensWithCache(t, data, "claude", "--days", "5", "-d")
	if err != nil {
		t.Fatalf("tokens claude --days 5 -d: %v\n%s", err, detailedOut)
	}
	modelSectionStart := strings.Index(detailedOut, "Models · last 5 days")
	if modelSectionStart == -1 {
		t.Fatalf("detailed Claude deep-dive output missing model section:\n%s", detailedOut)
	}
	modelSection := detailedOut[modelSectionStart:]
	for _, want := range []string{"in ", "out ", "cache "} {
		if !strings.Contains(modelSection, want) {
			t.Fatalf("detailed Claude deep-dive output missing %q:\n%s", want, detailedOut)
		}
	}

	codexOut, err := runTokensWithCache(t, data, "codex", "--days", "5")
	if err != nil {
		t.Fatalf("tokens codex --days 5: %v\n%s", err, codexOut)
	}
	for _, want := range []string{"Models · last 5 days", "gpt-5.5"} {
		if !strings.Contains(codexOut, want) {
			t.Fatalf("Codex deep-dive output missing %q:\n%s", want, codexOut)
		}
	}
}

func TestToolDeepDiveOmitsModelBreakdownWithoutModelData(t *testing.T) {
	today := startOfDay(time.Now())
	usage := sampleTool("Codex", today, 500_000)
	for i := range usage.Daily {
		usage.Daily[i].Models = nil
	}
	data := &ccusage.UsageData{Codex: usage}

	out, err := runTokensWithCache(t, data, "codex", "--days", "5")
	if err != nil {
		t.Fatalf("tokens codex --days 5: %v\n%s", err, out)
	}
	if strings.Contains(out, "Models · last 5 days") {
		t.Fatalf("did not expect a model section without model data:\n%s", out)
	}
}
