package render

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nickhudkins/tokens/ccusage"
)

var (
	Bold        = color.New(color.Bold)
	Dim         = color.New(color.Faint)
	Cyan        = color.New(color.FgCyan)
	CyanBold    = color.New(color.FgCyan, color.Bold)
	Green       = color.New(color.FgGreen)
	GreenBold   = color.New(color.FgGreen, color.Bold)
	Magenta     = color.New(color.FgMagenta)
	MagentaBold = color.New(color.FgMagenta, color.Bold)
	Yellow      = color.New(color.FgYellow)
	Red         = color.New(color.FgRed, color.Bold)
)

var ModelPalette = []*color.Color{
	color.New(color.FgCyan, color.Bold),
	color.New(color.FgMagenta, color.Bold),
	color.New(color.FgGreen, color.Bold),
	color.New(color.FgYellow, color.Bold),
	color.New(color.FgBlue, color.Bold),
	color.New(color.FgRed, color.Bold),
	color.New(color.FgHiCyan, color.Bold),
	color.New(color.FgHiMagenta, color.Bold),
}

// TermWidth reports the terminal's usable column count. It prefers the live tty
// size (ioctl) and falls back to $COLUMNS, then 80, so non-tty contexts (pipes,
// tests, SVG capture) and unsupported platforms stay deterministic.
func TermWidth() int {
	if w := osTermWidth(); w > 0 {
		return w
	}
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		return c
	}
	return 80
}

func FormatTokens(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func FormatCost(f float64) string {
	if f >= 1000 {
		return fmt.Sprintf("$%.0f", f)
	}
	return fmt.Sprintf("$%.2f", f)
}

func FormatPct(current, previous float64) string {
	if previous == 0 {
		return "—"
	}
	pct := ((current - previous) / previous) * 100
	sign := "+"
	if pct < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.1f%%", sign, pct)
}

func PctColor(current, previous float64) *color.Color {
	if previous == 0 {
		return Dim
	}
	switch {
	case current > previous:
		return GreenBold
	case current < previous:
		return Red
	default:
		return Dim
	}
}

func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var max float64
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return strings.Repeat(" ", len(values))
	}
	var b strings.Builder
	for _, v := range values {
		if v == 0 {
			b.WriteRune(' ')
			continue
		}
		idx := int((v / max) * float64(len(blocks)))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

// Series is one bar group in a GroupedVerticalBars chart — a labeled, colored
// value-per-day slice. One series renders one bar per column.
type Series struct {
	Name   string
	Color  *color.Color
	Values []float64
}

// barRows scales one value to a row count in [0,height] against the largest
// single value in the chart. Any positive value keeps at least one row so small
// days never vanish; zero (or a zero scale) draws nothing.
func barRows(value, maxVal float64, height int) int {
	if value <= 0 || maxVal <= 0 || height <= 0 {
		return 0
	}
	r := int(math.Round(value / maxVal * float64(height)))
	if r < 1 {
		r = 1
	}
	if r > height {
		r = height
	}
	return r
}

// barLayout is the per-column geometry GroupedVerticalBars draws with: bar width,
// the gap inside a day's cluster, the cluster's visible width, the full column
// stride, how often a label is printed, and which label set (full or short).
type barLayout struct {
	barW     int
	innerGap int
	groupW   int
	colWidth int
	every    int
	labels   []string
}

// chooseBarLayout picks the richest layout whose columns fit usable width. It
// walks a ladder from 2-wide bars with full labels down to 1-wide bars with
// short labels; if even the densest non-overlapping form overflows, it drops the
// inter-cluster gutter and thins labels so the chart still fits.
func chooseBarLayout(cols int, full, short []string, nSeries, usable int) barLayout {
	if nSeries < 1 {
		nSeries = 1
	}
	type cand struct {
		barW, innerGap int
		labels         []string
	}
	ladder := []cand{
		{2, 1, full},
		{2, 1, short},
		{1, 1, short},
		{1, 0, short},
	}
	for _, c := range ladder {
		groupW := nSeries*c.barW + (nSeries-1)*c.innerGap
		colWidth := groupW + 1
		if lw := maxLen(c.labels) + 1; lw > colWidth {
			colWidth = lw
		}
		if cols*colWidth <= usable {
			return barLayout{c.barW, c.innerGap, groupW, colWidth, 1, c.labels}
		}
	}

	groupW := nSeries
	colWidth := groupW + 1
	if cols*colWidth > usable {
		colWidth = groupW
	}
	if colWidth < 1 {
		colWidth = 1
	}
	every := 1
	if lw := maxLen(short); lw >= colWidth {
		every = lw/colWidth + 1
	}
	return barLayout{1, 0, groupW, colWidth, every, short}
}

func maxLen(ss []string) int {
	m := 0
	for _, s := range ss {
		if len(s) > m {
			m = len(s)
		}
	}
	return m
}

const chartHeight = 8

// GroupedVerticalBars draws one cluster of side-by-side bars per column (day) —
// one bar per series, each scaled against the largest single value across every
// series so the tallest bar fills the chart. The layout adapts to width: bar
// width, label form, and label thinning are chosen to fit. fullLabels and
// shortLabels are parallel per-column label sets; the chosen layout picks one.
func GroupedVerticalBars(series []Series, fullLabels, shortLabels []string, width int) {
	if len(series) == 0 {
		fmt.Println("  (none)")
		return
	}

	cols := 0
	for _, s := range series {
		if len(s.Values) > cols {
			cols = len(s.Values)
		}
	}
	if cols == 0 {
		fmt.Println("  (none)")
		return
	}

	var maxVal float64
	for _, s := range series {
		for _, v := range s.Values {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal == 0 {
		fmt.Println("  (none)")
		return
	}

	lay := chooseBarLayout(cols, fullLabels, shortLabels, len(series), width-2)

	// When even the densest layout can't fit every column, render only the most
	// recent days that fit and note it, rather than letting the chart wrap. Bars
	// still scale to the full-window max, so the title's peak stays accurate.
	start := 0
	if fit := (width - 2) / lay.colWidth; fit >= 1 && fit < cols {
		start = cols - fit
		fmt.Println(Dim.Sprintf("  (showing last %d of %d days — widen terminal for the full window)", fit, cols))
	}

	heights := make([][]int, len(series))
	for si, s := range series {
		hs := make([]int, cols)
		for c := 0; c < cols; c++ {
			if c < len(s.Values) {
				hs[c] = barRows(s.Values[c], maxVal, chartHeight)
			}
		}
		heights[si] = hs
	}

	bar := strings.Repeat("█", lay.barW)
	gap := strings.Repeat(" ", lay.innerGap)
	blank := strings.Repeat(" ", lay.barW)
	pad := strings.Repeat(" ", lay.colWidth-lay.groupW)

	for row := chartHeight; row >= 1; row-- {
		fmt.Print("  ")
		for c := start; c < cols; c++ {
			var cell strings.Builder
			for si := range series {
				if si > 0 {
					cell.WriteString(gap)
				}
				if heights[si][c] >= row {
					cell.WriteString(series[si].Color.Sprint(bar))
				} else {
					cell.WriteString(blank)
				}
			}
			fmt.Print(cell.String() + pad)
		}
		fmt.Println()
	}

	fmt.Print("  ")
	for c := start; c < cols; c++ {
		label := ""
		// Thin from the right so the most recent day is always labeled.
		if c < len(lay.labels) && (cols-1-c)%lay.every == 0 {
			label = lay.labels[c]
		}
		fmt.Printf("%-*s", lay.colWidth, label)
	}
	fmt.Println()
}

func DayLabel(date, today time.Time) string {
	if date.Equal(today) {
		return "Today"
	}
	if date.Equal(today.AddDate(0, 0, -1)) {
		return "Yest"
	}
	return date.Format("Mon 02")
}

func CombineDaily(data *ccusage.UsageData) []ccusage.DailyEntry {
	byDate := map[string]*ccusage.DailyEntry{}
	add := func(daily []ccusage.DailyEntry) {
		for _, d := range daily {
			key := d.Date.Format("2006-01-02")
			if existing, ok := byDate[key]; ok {
				existing.InputTokens += d.InputTokens
				existing.OutputTokens += d.OutputTokens
				existing.CacheTokens += d.CacheTokens
				existing.TotalTokens += d.TotalTokens
				existing.Cost += d.Cost
			} else {
				entry := d
				byDate[key] = &entry
			}
		}
	}
	if data.Claude != nil {
		add(data.Claude.Daily)
	}
	if data.Codex != nil {
		add(data.Codex.Daily)
	}
	var result []ccusage.DailyEntry
	for _, e := range byDate {
		result = append(result, *e)
	}
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].Date.Before(result[j-1].Date); j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

func FilterLastDays(entries []ccusage.DailyEntry, today time.Time, days int) []ccusage.DailyEntry {
	cutoff := today.AddDate(0, 0, -(days - 1))
	out := make([]ccusage.DailyEntry, 0, days)
	for _, e := range entries {
		if !e.Date.Before(cutoff) && !e.Date.After(today) {
			out = append(out, e)
		}
	}
	return out
}

func FillMissingDays(entries []ccusage.DailyEntry, today time.Time, days int) []ccusage.DailyEntry {
	byDate := map[string]ccusage.DailyEntry{}
	for _, e := range entries {
		byDate[e.Date.Format("2006-01-02")] = e
	}
	out := make([]ccusage.DailyEntry, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
		key := d.Format("2006-01-02")
		if e, ok := byDate[key]; ok {
			out = append(out, e)
		} else {
			out = append(out, ccusage.DailyEntry{Date: d})
		}
	}
	return out
}

func FindEntry(daily []ccusage.DailyEntry, date time.Time) ccusage.DailyEntry {
	key := date.Format("2006-01-02")
	for _, d := range daily {
		if d.Date.Format("2006-01-02") == key {
			return d
		}
	}
	return ccusage.DailyEntry{Date: date}
}

func WeekTotals(entries []ccusage.DailyEntry, today time.Time) (thisWeek, lastWeek ccusage.DailyEntry) {
	daysFromMonday := (int(today.Weekday()) + 6) % 7
	thisMonday := today.AddDate(0, 0, -daysFromMonday)
	lastMonday := thisMonday.AddDate(0, 0, -7)

	for _, e := range entries {
		if !e.Date.Before(thisMonday) && !e.Date.After(today) {
			thisWeek.TotalTokens += e.TotalTokens
			thisWeek.InputTokens += e.InputTokens
			thisWeek.OutputTokens += e.OutputTokens
			thisWeek.CacheTokens += e.CacheTokens
			thisWeek.Cost += e.Cost
		} else if !e.Date.Before(lastMonday) && e.Date.Before(thisMonday) {
			lastWeek.TotalTokens += e.TotalTokens
			lastWeek.InputTokens += e.InputTokens
			lastWeek.OutputTokens += e.OutputTokens
			lastWeek.CacheTokens += e.CacheTokens
			lastWeek.Cost += e.Cost
		}
	}
	return
}

func MonthTotals(entries []ccusage.DailyEntry, today time.Time) (thisMonth, lastMonth ccusage.DailyEntry) {
	thisStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	lastStart := thisStart.AddDate(0, -1, 0)

	for _, e := range entries {
		if !e.Date.Before(thisStart) && !e.Date.After(today) {
			thisMonth.TotalTokens += e.TotalTokens
			thisMonth.InputTokens += e.InputTokens
			thisMonth.OutputTokens += e.OutputTokens
			thisMonth.CacheTokens += e.CacheTokens
			thisMonth.Cost += e.Cost
		} else if !e.Date.Before(lastStart) && e.Date.Before(thisStart) {
			lastMonth.TotalTokens += e.TotalTokens
			lastMonth.InputTokens += e.InputTokens
			lastMonth.OutputTokens += e.OutputTokens
			lastMonth.CacheTokens += e.CacheTokens
			lastMonth.Cost += e.Cost
		}
	}
	return
}

func WindowTotals(entries []ccusage.DailyEntry, today time.Time, days int) (current, previous ccusage.DailyEntry) {
	if days <= 0 {
		return
	}

	currentStart := today.AddDate(0, 0, -(days - 1))
	previousStart := currentStart.AddDate(0, 0, -days)
	previousEnd := currentStart.AddDate(0, 0, -1)

	for _, e := range entries {
		switch {
		case !e.Date.Before(currentStart) && !e.Date.After(today):
			addEntry(&current, e)
		case !e.Date.Before(previousStart) && !e.Date.After(previousEnd):
			addEntry(&previous, e)
		}
	}
	return
}

func ModelTotals(usage *ccusage.ToolUsage, today time.Time, days int) []ccusage.ModelEntry {
	if usage == nil || days <= 0 {
		return nil
	}

	byModel := map[string]*ccusage.ModelEntry{}
	for _, day := range FilterLastDays(usage.Daily, today, days) {
		for _, model := range day.Models {
			total, ok := byModel[model.Model]
			if !ok {
				entry := ccusage.ModelEntry{Model: model.Model}
				total = &entry
				byModel[model.Model] = total
			}
			total.InputTokens += model.InputTokens
			total.OutputTokens += model.OutputTokens
			total.CacheTokens += model.CacheTokens
			total.TotalTokens += model.TotalTokens
			total.Cost += model.Cost
		}
	}

	out := make([]ccusage.ModelEntry, 0, len(byModel))
	for _, total := range byModel {
		out = append(out, *total)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalTokens == out[j].TotalTokens {
			return out[i].Model < out[j].Model
		}
		return out[i].TotalTokens > out[j].TotalTokens
	})
	return out
}

type modelSeriesValues struct {
	name        string
	tokenVals   []float64
	costVals    []float64
	totalTokens int64
}

func ModelSeries(data *ccusage.UsageData, today time.Time, days int) (tokenSeries, costSeries []Series) {
	if data == nil || days <= 0 {
		return nil, nil
	}

	slotByDate := make(map[string]int, days)
	for i := 0; i < days; i++ {
		date := today.AddDate(0, 0, -(days - 1 - i))
		slotByDate[date.Format("2006-01-02")] = i
	}

	byModel := map[string]*modelSeriesValues{}
	add := func(usage *ccusage.ToolUsage) {
		if usage == nil {
			return
		}
		for _, day := range usage.Daily {
			slot, ok := slotByDate[day.Date.Format("2006-01-02")]
			if !ok {
				continue
			}
			for _, model := range day.Models {
				values, ok := byModel[model.Model]
				if !ok {
					values = &modelSeriesValues{
						name:      model.Model,
						tokenVals: make([]float64, days),
						costVals:  make([]float64, days),
					}
					byModel[model.Model] = values
				}
				values.tokenVals[slot] += float64(model.TotalTokens)
				values.costVals[slot] += model.Cost
				values.totalTokens += model.TotalTokens
			}
		}
	}
	add(data.Claude)
	add(data.Codex)

	models := make([]*modelSeriesValues, 0, len(byModel))
	for _, values := range byModel {
		models = append(models, values)
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].totalTokens == models[j].totalTokens {
			return models[i].name < models[j].name
		}
		return models[i].totalTokens > models[j].totalTokens
	})

	tokenSeries = make([]Series, len(models))
	costSeries = make([]Series, len(models))
	for i, values := range models {
		col := Dim
		if len(ModelPalette) > 0 {
			col = ModelPalette[i%len(ModelPalette)]
		}
		tokenSeries[i] = Series{Name: values.name, Color: col, Values: values.tokenVals}
		costSeries[i] = Series{Name: values.name, Color: col, Values: values.costVals}
	}
	return tokenSeries, costSeries
}

func addEntry(total *ccusage.DailyEntry, entry ccusage.DailyEntry) {
	total.InputTokens += entry.InputTokens
	total.OutputTokens += entry.OutputTokens
	total.CacheTokens += entry.CacheTokens
	total.TotalTokens += entry.TotalTokens
	total.Cost += entry.Cost
}

func RuleColumn(width int) string {
	return strings.Repeat("─", width)
}

func PadRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
