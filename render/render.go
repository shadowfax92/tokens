package render

import (
	"fmt"
	"math"
	"os"
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

// StackedVerticalBars draws the solid-block bar chart, but each day's bar is
// split top-to-bottom across the series (first series on the bottom). The bar
// height stays the column's combined total, so the silhouette matches a plain
// single-series chart — only the coloring carries the per-series breakdown.
// formatter labels the per-column total under each bar.
func StackedVerticalBars(series []Series, labels []string, formatter func(float64) string) {
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

	totals := make([]float64, cols)
	var maxTotal float64
	for i := 0; i < cols; i++ {
		var t float64
		for _, s := range series {
			if i < len(s.Values) {
				t += s.Values[i]
			}
		}
		totals[i] = t
		if t > maxTotal {
			maxTotal = t
		}
	}

	if maxTotal == 0 {
		fmt.Println("  (none)")
		return
	}

	chartHeight := 8
	colWidth := 7
	for _, l := range labels {
		if len(l)+1 > colWidth {
			colWidth = len(l) + 1
		}
	}
	for _, t := range totals {
		if w := len(formatter(t)) + 1; w > colWidth {
			colWidth = w
		}
	}

	splits := make([][]int, cols)
	for i := 0; i < cols; i++ {
		vals := make([]float64, len(series))
		for si, s := range series {
			if i < len(s.Values) {
				vals[si] = s.Values[i]
			}
		}
		splits[i] = stackHeights(vals, maxTotal, chartHeight)
	}

	for row := chartHeight; row >= 1; row-- {
		fmt.Print("  ")
		for i := 0; i < cols; i++ {
			lo, printed := 0, false
			for si := range series {
				hi := lo + splits[i][si]
				if splits[i][si] > 0 && row > lo && row <= hi {
					fmt.Printf("%s%s", series[si].Color.Sprint("██"), strings.Repeat(" ", colWidth-2))
					printed = true
					break
				}
				lo = hi
			}
			if !printed {
				fmt.Print(strings.Repeat(" ", colWidth))
			}
		}
		fmt.Println()
	}

	fmt.Print("  ")
	for _, l := range labels {
		fmt.Printf("%-*s", colWidth, l)
	}
	fmt.Println()

	fmt.Print("  ")
	for _, t := range totals {
		Dim.Printf("%-*s", colWidth, formatter(t))
	}
	fmt.Println()
}

// stackHeights splits one column's bar into per-series row counts (bottom-to-top
// in series order). The bar's height is round(total/maxTotal * height), and the
// rows are apportioned to the series by cumulative proportional rounding so they
// never overflow the bar. Any non-zero series keeps at least one row whenever the
// bar is tall enough to fit every non-zero series.
func stackHeights(values []float64, maxTotal float64, height int) []int {
	rows := make([]int, len(values))
	var total float64
	for _, v := range values {
		total += v
	}
	if total <= 0 || maxTotal <= 0 || height <= 0 {
		return rows
	}

	barTop := int(math.Round(total / maxTotal * float64(height)))
	if barTop < 1 {
		barTop = 1
	}
	if barTop > height {
		barTop = height
	}

	cum, prev := 0.0, 0
	for i, v := range values {
		cum += v
		cumRows := int(math.Round(cum / total * float64(barTop)))
		if cumRows > barTop {
			cumRows = barTop
		}
		rows[i] = cumRows - prev
		prev = cumRows
	}

	ensureVisible(rows, values, barTop)
	return rows
}

// ensureVisible gives every non-zero series at least one row when the bar can fit
// them all, borrowing from the tallest stack so the total stays put.
func ensureVisible(rows []int, values []float64, barTop int) {
	nonzero := 0
	for _, v := range values {
		if v > 0 {
			nonzero++
		}
	}
	if nonzero == 0 || barTop < nonzero {
		return
	}
	for {
		starved := -1
		for i := range rows {
			if values[i] > 0 && rows[i] == 0 {
				starved = i
				break
			}
		}
		if starved == -1 {
			return
		}
		donor := -1
		for i := range rows {
			if rows[i] >= 2 && (donor == -1 || rows[i] > rows[donor]) {
				donor = i
			}
		}
		if donor == -1 {
			return
		}
		rows[donor]--
		rows[starved]++
	}
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
