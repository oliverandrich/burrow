package humanize

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/oliverandrich/burrow/i18n"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func toTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	default:
		return time.Time{}, false
	}
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case *int:
		if n == nil {
			return 0, false
		}
		return *n, true
	default:
		return 0, false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case *int64:
		if n == nil {
			return 0, false
		}
		return *n, true
	case int:
		return int64(n), true
	case *int:
		if n == nil {
			return 0, false
		}
		return int64(*n), true
	default:
		return 0, false
	}
}

// Default date formats per locale for naturalday fallback.
var defaultDateFormats = map[string]string{
	"en": "Jan 2, 2006",
	"de": "02.01.2006",
}

func filesizeformat(ctx context.Context, v any) string {
	bytes, ok := toInt64(v)
	if !ok {
		return ""
	}
	locale := i18n.Locale(ctx)
	negative := bytes < 0
	if negative {
		bytes = -bytes
	}

	sign := ""
	if negative {
		sign = "-"
	}

	units := []struct {
		suffix    string
		threshold int64
	}{
		{"PB", 1 << 50},
		{"TB", 1 << 40},
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
	}

	for _, u := range units {
		if bytes >= u.threshold {
			value := float64(bytes) / float64(u.threshold)
			return sign + formatDecimal(locale, value, 1) + " " + u.suffix
		}
	}

	if bytes == 1 {
		return sign + "1 byte"
	}
	return sign + strconv.FormatInt(bytes, 10) + " bytes"
}

func formatDecimal(locale string, value float64, decimals int) string {
	p := message.NewPrinter(language.MustParse(locale))
	format := fmt.Sprintf("%%.%df", decimals)
	return p.Sprintf(format, value)
}

func intcomma(ctx context.Context, v any) string {
	n, ok := toInt(v)
	if !ok {
		return ""
	}
	p := message.NewPrinter(language.MustParse(i18n.Locale(ctx)))
	return p.Sprintf("%d", n)
}

func ordinal(ctx context.Context, v any) string {
	n, ok := toInt(v)
	if !ok {
		return ""
	}
	locale := i18n.Locale(ctx)
	data := map[string]any{"N": n}

	if locale == "de" {
		return i18n.TData(ctx, "humanize-ordinal", data)
	}

	// English ordinal rules
	abs := n
	if abs < 0 {
		abs = -abs
	}
	lastTwo := abs % 100
	lastOne := abs % 10

	var key string
	switch {
	case lastTwo >= 11 && lastTwo <= 13:
		key = "humanize-ordinal-th"
	case lastOne == 1:
		key = "humanize-ordinal-st"
	case lastOne == 2:
		key = "humanize-ordinal-nd"
	case lastOne == 3:
		key = "humanize-ordinal-rd"
	default:
		key = "humanize-ordinal-th"
	}
	return i18n.TData(ctx, key, data)
}

func apnumber(ctx context.Context, v any) string {
	n, ok := toInt(v)
	if !ok {
		return ""
	}
	if n < 1 || n > 9 {
		return strconv.Itoa(n)
	}
	return i18n.T(ctx, fmt.Sprintf("humanize-apnumber-%d", n))
}

func sameDay(a, b time.Time) bool {
	ya, ma, da := a.Date()
	yb, mb, db := b.Date()
	return ya == yb && ma == mb && da == db
}

func naturalday(ctx context.Context, dateFormat string, v any, now time.Time) string {
	t, ok := toTime(v)
	if !ok {
		return ""
	}
	switch {
	case sameDay(t, now):
		return i18n.T(ctx, "humanize-today")
	case sameDay(t, now.AddDate(0, 0, -1)):
		return i18n.T(ctx, "humanize-yesterday")
	case sameDay(t, now.AddDate(0, 0, 1)):
		return i18n.T(ctx, "humanize-tomorrow")
	}

	if dateFormat != "" {
		return t.Format(dateFormat)
	}
	if f, ok := defaultDateFormats[i18n.Locale(ctx)]; ok {
		return t.Format(f)
	}
	return t.Format(defaultDateFormats["en"])
}

func naturaltime(ctx context.Context, v any, now time.Time) string {
	t, ok := toTime(v)
	if !ok {
		return ""
	}
	diff := now.Sub(t)
	future := diff < 0
	if future {
		diff = -diff
	}

	seconds := int(math.Round(diff.Seconds()))
	if seconds < 5 {
		return i18n.T(ctx, "humanize-just-now")
	}

	type level struct {
		key       string
		threshold int
		divisor   int
	}

	levels := []level{
		{"years", 365 * 24 * 3600, 365 * 24 * 3600},
		{"months", 30 * 24 * 3600, 30 * 24 * 3600},
		{"weeks", 7 * 24 * 3600, 7 * 24 * 3600},
		{"days", 24 * 3600, 24 * 3600},
		{"hours", 3600, 3600},
		{"minutes", 60, 60},
		{"seconds", 0, 1},
	}

	for _, l := range levels {
		if seconds >= l.threshold {
			count := seconds / l.divisor
			suffix := "-ago"
			if future {
				suffix = "-from-now"
			}
			return i18n.TPlural(ctx, "humanize-"+l.key+suffix, count)
		}
	}

	return i18n.T(ctx, "humanize-just-now")
}
