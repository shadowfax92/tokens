package render

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/nickhudkins/tokens/ccusage"
)

func TestModelSeriesAlignsDailyValuesAcrossTools(t *testing.T) {
	today := time.Date(2026, 7, 4, 0, 0, 0, 0, time.Local)
	data := &ccusage.UsageData{
		Claude: &ccusage.ToolUsage{
			Tool: "Claude Code",
			Daily: []ccusage.DailyEntry{
				{
					Date: today.AddDate(0, 0, -4),
					Models: []ccusage.ModelEntry{
						{Model: "outside", TotalTokens: 999, Cost: 9.99},
					},
				},
				{
					Date: today.AddDate(0, 0, -3),
					Models: []ccusage.ModelEntry{
						{Model: "fable", TotalTokens: 10, Cost: 0.10},
						{Model: "opus", TotalTokens: 5, Cost: 0.05},
					},
				},
				{
					Date: today.AddDate(0, 0, -1),
					Models: []ccusage.ModelEntry{
						{Model: "shared", TotalTokens: 30, Cost: 0.30},
						{Model: "fable", TotalTokens: 7, Cost: 0.07},
					},
				},
			},
		},
		Codex: &ccusage.ToolUsage{
			Tool: "Codex",
			Daily: []ccusage.DailyEntry{
				{
					Date: today.AddDate(0, 0, -3),
					Models: []ccusage.ModelEntry{
						{Model: "shared", TotalTokens: 20, Cost: 0.20},
					},
				},
				{
					Date: today.AddDate(0, 0, -2),
					Models: []ccusage.ModelEntry{
						{Model: "gpt", TotalTokens: 100, Cost: 1.00},
						{Model: "fable", TotalTokens: 3, Cost: 0.03},
					},
				},
				{
					Date: today,
					Models: []ccusage.ModelEntry{
						{Model: "shared", TotalTokens: 4, Cost: 0.04},
						{Model: "beta", TotalTokens: 5, Cost: 0.05},
						{Model: "alpha", TotalTokens: 5, Cost: 0.05},
					},
				},
			},
		},
	}

	tokenSeries, costSeries := ModelSeries(data, today, 4)

	wantNames := []string{"gpt", "shared", "fable", "alpha", "beta", "opus"}
	if got := seriesNames(tokenSeries); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("token series names = %#v, want %#v", got, wantNames)
	}
	if got := seriesNames(costSeries); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("cost series names = %#v, want %#v", got, wantNames)
	}

	assertSeriesValues(t, tokenSeries, "gpt", []float64{0, 100, 0, 0})
	assertSeriesValues(t, tokenSeries, "shared", []float64{20, 0, 30, 4})
	assertSeriesValues(t, tokenSeries, "fable", []float64{10, 3, 7, 0})
	assertSeriesValues(t, tokenSeries, "alpha", []float64{0, 0, 0, 5})
	assertSeriesValues(t, tokenSeries, "beta", []float64{0, 0, 0, 5})
	assertSeriesValues(t, tokenSeries, "opus", []float64{5, 0, 0, 0})

	assertSeriesValues(t, costSeries, "gpt", []float64{0, 1.00, 0, 0})
	assertSeriesValues(t, costSeries, "shared", []float64{0.20, 0, 0.30, 0.04})
	assertSeriesValues(t, costSeries, "fable", []float64{0.10, 0.03, 0.07, 0})
}

func TestModelSeriesCyclesModelPalette(t *testing.T) {
	if len(ModelPalette) == 0 {
		t.Fatal("ModelPalette must not be empty")
	}

	today := time.Date(2026, 7, 4, 0, 0, 0, 0, time.Local)
	models := make([]ccusage.ModelEntry, len(ModelPalette)+2)
	for i := range models {
		models[i] = ccusage.ModelEntry{
			Model:       fmt.Sprintf("m%02d", i),
			TotalTokens: int64(len(models) - i),
			Cost:        float64(i),
		}
	}
	data := &ccusage.UsageData{
		Claude: &ccusage.ToolUsage{
			Tool: "Claude Code",
			Daily: []ccusage.DailyEntry{{
				Date:   today,
				Models: models,
			}},
		},
	}

	tokenSeries, costSeries := ModelSeries(data, today, 1)
	if len(tokenSeries) != len(models) {
		t.Fatalf("len(tokenSeries) = %d, want %d", len(tokenSeries), len(models))
	}

	for i := range tokenSeries {
		wantColor := ModelPalette[i%len(ModelPalette)]
		if tokenSeries[i].Color != wantColor {
			t.Fatalf("tokenSeries[%d].Color did not cycle from ModelPalette", i)
		}
		if costSeries[i].Color != wantColor {
			t.Fatalf("costSeries[%d].Color did not cycle from ModelPalette", i)
		}
	}
}

func seriesNames(series []Series) []string {
	names := make([]string, len(series))
	for i, s := range series {
		names[i] = s.Name
	}
	return names
}

func assertSeriesValues(t *testing.T, series []Series, name string, want []float64) {
	t.Helper()
	for _, s := range series {
		if s.Name == name {
			if !reflect.DeepEqual(s.Values, want) {
				t.Fatalf("%s values = %#v, want %#v", name, s.Values, want)
			}
			return
		}
	}
	t.Fatalf("missing series %q in %#v", name, seriesNames(series))
}
