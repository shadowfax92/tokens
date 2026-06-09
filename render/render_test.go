package render

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestBarRows(t *testing.T) {
	cases := []struct {
		value, maxVal float64
		height, want  int
	}{
		{100, 100, 8, 8},
		{50, 100, 8, 4},
		{1, 100, 8, 1},   // tiny but positive stays visible
		{0, 100, 8, 0},   // zero draws nothing
		{100, 0, 8, 0},   // no scale
		{100, 100, 0, 0}, // no height
		{200, 100, 8, 8}, // clamps to height
	}
	for _, c := range cases {
		if got := barRows(c.value, c.maxVal, c.height); got != c.want {
			t.Errorf("barRows(%g, %g, %d) = %d, want %d", c.value, c.maxVal, c.height, got, c.want)
		}
	}
}

func TestChooseBarLayout(t *testing.T) {
	full := []string{"Mon 02", "Today", "Yest"}
	short := []string{"02", "03", "04"}
	rep := func(base []string, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = base[i%len(base)]
		}
		return out
	}
	cases := []struct {
		name          string
		cols, nSeries int
		usable        int
		wantBarW      int
	}{
		{"wide 14d two tools", 14, 2, 158, 2},
		{"narrow 14d two tools", 14, 2, 78, 1},
		{"single tool wide", 14, 1, 158, 2},
		{"dense 30d narrow", 30, 2, 78, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lay := chooseBarLayout(c.cols, rep(full, c.cols), rep(short, c.cols), c.nSeries, c.usable)
			if lay.barW != c.wantBarW {
				t.Errorf("barW = %d, want %d", lay.barW, c.wantBarW)
			}
			if lay.barW < 1 {
				t.Fatalf("barW must be >= 1, got %d", lay.barW)
			}
			if lay.every < 1 {
				t.Errorf("every must be >= 1, got %d", lay.every)
			}
			if c.cols*lay.colWidth > c.usable {
				t.Errorf("layout overflows: cols(%d) * colWidth(%d) = %d > usable %d",
					c.cols, lay.colWidth, c.cols*lay.colWidth, c.usable)
			}
		})
	}
}

func TestGroupedVerticalBarsRendersSideBySide(t *testing.T) {
	// EnableColor forces ANSI regardless of TTY detection so the test can locate
	// each series' color in the output.
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.EnableColor()
	magenta := color.New(color.FgMagenta, color.Bold)
	magenta.EnableColor()

	out := captureStdout(t, func() {
		series := []Series{
			{Name: "Claude", Color: cyan, Values: []float64{10}},
			{Name: "Codex", Color: magenta, Values: []float64{10}},
		}
		GroupedVerticalBars(series, []string{"Today"}, []string{"02"}, 80)
	})

	// Grouped (not stacked): with equal values both bars fill the column, so at
	// least one row carries BOTH colors side by side. A stacked renderer would
	// never co-locate them on one row.
	const cyanCode, magentaCode = "\x1b[36", "\x1b[35"
	sameRow := false
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, cyanCode) && strings.Contains(ln, magentaCode) {
			sameRow = true
			break
		}
	}
	if !sameRow {
		t.Fatalf("expected both series' colors on the same row (side-by-side):\n%q", out)
	}
}

func TestGroupedVerticalBarsScalesEachSeriesIndependently(t *testing.T) {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.EnableColor()
	magenta := color.New(color.FgMagenta, color.Bold)
	magenta.EnableColor()

	out := captureStdout(t, func() {
		series := []Series{
			{Name: "Claude", Color: cyan, Values: []float64{100}},
			{Name: "Codex", Color: magenta, Values: []float64{10}},
		}
		GroupedVerticalBars(series, []string{"Today"}, []string{"02"}, 80)
	})

	const cyanCode, magentaCode = "\x1b[36", "\x1b[35"
	cyanRows, magentaRows := 0, 0
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, cyanCode) {
			cyanRows++
		}
		if strings.Contains(ln, magentaCode) {
			magentaRows++
		}
	}
	if cyanRows <= magentaRows {
		t.Fatalf("dominant series should occupy more rows: cyan=%d magenta=%d\n%q", cyanRows, magentaRows, out)
	}
	if magentaRows == 0 {
		t.Fatalf("small but non-zero series must stay visible (>=1 row):\n%q", out)
	}
}

func TestGroupedVerticalBarsTruncatesWhenTooNarrow(t *testing.T) {
	// 60 two-tool days cannot fit in ~30 columns without wrapping, so the
	// renderer should drop to the most-recent days that fit and say so.
	const n = 60
	vals := make([]float64, n)
	full := make([]string, n)
	short := make([]string, n)
	for i := range vals {
		vals[i] = float64(i + 1)
		full[i] = "Mon 02"
		short[i] = "02"
	}
	out := captureStdout(t, func() {
		series := []Series{
			{Name: "A", Color: Dim, Values: vals},
			{Name: "B", Color: Dim, Values: vals},
		}
		GroupedVerticalBars(series, full, short, 30)
	})
	if !strings.Contains(out, "showing last") {
		t.Fatalf("expected a truncation note at a too-narrow width:\n%q", out)
	}
}

func TestGroupedVerticalBarsNoTruncationWhenItFits(t *testing.T) {
	// 14 two-tool days fit comfortably at 80 columns — no truncation note.
	const n = 14
	vals := make([]float64, n)
	full := make([]string, n)
	short := make([]string, n)
	for i := range vals {
		vals[i] = float64(i + 1)
		full[i] = "Mon 02"
		short[i] = "02"
	}
	out := captureStdout(t, func() {
		series := []Series{
			{Name: "A", Color: Dim, Values: vals},
			{Name: "B", Color: Dim, Values: vals},
		}
		GroupedVerticalBars(series, full, short, 80)
	})
	if strings.Contains(out, "showing last") {
		t.Fatalf("did not expect truncation at 80 cols for 14 days:\n%q", out)
	}
}

func TestGroupedVerticalBarsEmptyPrintsNone(t *testing.T) {
	out := captureStdout(t, func() {
		series := []Series{{Name: "Claude", Color: Dim, Values: []float64{0, 0}}}
		GroupedVerticalBars(series, []string{"a", "b"}, []string{"a", "b"}, 80)
	})
	if !strings.Contains(out, "(none)") {
		t.Fatalf("expected (none) for an all-zero series:\n%q", out)
	}
}
