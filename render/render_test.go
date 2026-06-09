package render

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func sumInts(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func TestStackHeights(t *testing.T) {
	cases := []struct {
		name     string
		values   []float64
		maxTotal float64
		height   int
		want     []int
	}{
		{"equal split", []float64{50, 50}, 100, 8, []int{4, 4}},
		{"dominant plus tiny stays visible", []float64{95, 5}, 100, 8, []int{7, 1}},
		{"single nonzero fills the bar", []float64{100, 0}, 100, 8, []int{8, 0}},
		{"all zero draws nothing", []float64{0, 0}, 100, 8, []int{0, 0}},
		{"below-scale total rounds to one row", []float64{5, 5}, 100, 8, []int{1, 0}},
		{"proportional", []float64{30, 10}, 40, 8, []int{6, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stackHeights(tc.values, tc.maxTotal, tc.height)
			if len(got) != len(tc.want) {
				t.Fatalf("len(stackHeights(%v)) = %d, want %d", tc.values, len(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("stackHeights(%v, %g, %d) = %v, want %v", tc.values, tc.maxTotal, tc.height, got, tc.want)
				}
			}
		})
	}
}

func TestStackedVerticalBarsStacksColorsBottomToTop(t *testing.T) {
	// EnableColor forces these Colors to emit ANSI regardless of TTY detection,
	// so the test deterministically exercises the stacking order.
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.EnableColor()
	green := color.New(color.FgGreen, color.Bold)
	green.EnableColor()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	series := []Series{
		{Name: "Claude", Color: cyan, Values: []float64{10}},
		{Name: "Codex", Color: green, Values: []float64{10}},
	}
	StackedVerticalBars(series, []string{"Today"}, func(v float64) string { return fmt.Sprintf("%.0f", v) })

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	// First series is the bottom of the stack, so the second series' color
	// (green) must appear on an earlier (higher) row than the first's (cyan).
	const cyanCode, greenCode = "\x1b[36", "\x1b[32"
	greenRow, cyanRow := -1, -1
	for i, ln := range strings.Split(out, "\n") {
		if greenRow == -1 && strings.Contains(ln, greenCode) {
			greenRow = i
		}
		if cyanRow == -1 && strings.Contains(ln, cyanCode) {
			cyanRow = i
		}
	}
	if greenRow == -1 || cyanRow == -1 {
		t.Fatalf("expected both green and cyan bars in output:\n%q", out)
	}
	if greenRow >= cyanRow {
		t.Fatalf("expected second series (green) stacked above first (cyan): greenRow=%d cyanRow=%d", greenRow, cyanRow)
	}
}

func TestStackHeightsNeverOverflowsAndStaysVisible(t *testing.T) {
	values := []float64{17, 3, 9}
	got := stackHeights(values, 40, 8)
	if s := sumInts(got); s > 8 {
		t.Fatalf("rows sum %d exceeds chart height 8: %v", s, got)
	}
	for i, v := range values {
		if v > 0 && got[i] == 0 {
			t.Fatalf("series %d (value %g) collapsed to 0 rows: %v", i, v, got)
		}
	}
}
