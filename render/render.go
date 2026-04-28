package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nickhudkins/tokens/ccusage"
)

var (
	Bold      = color.New(color.Bold)
	Dim       = color.New(color.Faint)
	Cyan      = color.New(color.FgCyan)
	CyanBold  = color.New(color.FgCyan, color.Bold)
	Green     = color.New(color.FgGreen)
	GreenBold = color.New(color.FgGreen, color.Bold)
	Magenta   = color.New(color.FgMagenta)
	Yellow    = color.New(color.FgYellow)
	Red       = color.New(color.FgRed, color.Bold)
)

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

func VerticalBars(values []float64, labels []string, formatter func(float64) string, c *color.Color) {
	var maxVal float64
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal == 0 {
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
	for _, v := range values {
		if w := len(formatter(v)) + 1; w > colWidth {
			colWidth = w
		}
	}

	for row := chartHeight; row >= 1; row-- {
		threshold := float64(row) / float64(chartHeight) * maxVal
		fmt.Print("  ")
		for _, v := range values {
			if v >= threshold && v > 0 {
				bar := c.Sprint("██")
				pad := colWidth - 2
				fmt.Printf("%s%s", bar, strings.Repeat(" ", pad))
			} else {
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
	for _, v := range values {
		Dim.Printf("%-*s", colWidth, formatter(v))
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

func RuleColumn(width int) string {
	return strings.Repeat("─", width)
}

func PadRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
