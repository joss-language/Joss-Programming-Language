package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FormatDate converts a standard PHP/Joss format string to formatted date
func FormatDate(format string, t time.Time) string {
	if format == "" {
		format = "Y-m-d H:i:s"
	}

	var sb strings.Builder
	escaped := false

	for i := 0; i < len(format); i++ {
		c := format[i]
		if escaped {
			sb.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}

		switch c {
		// Year
		case 'Y':
			sb.WriteString(t.Format("2006"))
		case 'y':
			sb.WriteString(t.Format("06"))
		// Month
		case 'm':
			sb.WriteString(t.Format("01"))
		case 'n':
			sb.WriteString(strconv.Itoa(int(t.Month())))
		case 'F':
			sb.WriteString(t.Month().String())
		case 'M':
			sb.WriteString(t.Format("Jan"))
		// Day
		case 'd':
			sb.WriteString(t.Format("02"))
		case 'j':
			sb.WriteString(strconv.Itoa(t.Day()))
		case 'D':
			sb.WriteString(t.Format("Mon"))
		case 'l':
			sb.WriteString(t.Format("Monday"))
		// Time
		case 'H':
			sb.WriteString(t.Format("15"))
		case 'h':
			sb.WriteString(t.Format("03"))
		case 'G':
			sb.WriteString(strconv.Itoa(t.Hour()))
		case 'g':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			sb.WriteString(strconv.Itoa(h))
		case 'i':
			sb.WriteString(t.Format("04"))
		case 's':
			sb.WriteString(t.Format("05"))
		case 'a':
			if t.Hour() < 12 {
				sb.WriteString("am")
			} else {
				sb.WriteString("pm")
			}
		case 'A':
			if t.Hour() < 12 {
				sb.WriteString("AM")
			} else {
				sb.WriteString("PM")
			}
		// Full formats
		case 'c':
			sb.WriteString(t.Format("2006-01-02T15:04:05-07:00"))
		case 'r':
			sb.WriteString(t.Format(time.RFC1123Z))
		case 'U':
			sb.WriteString(strconv.FormatInt(t.Unix(), 10))
		case 'O':
			sb.WriteString(t.Format("-0700"))
		case 'P':
			sb.WriteString(t.Format("-07:00"))
		case 'T':
			sb.WriteString(t.Format("MST"))
		default:
			sb.WriteByte(c)
		}
	}

	return sb.String()
}

// ParseHumanTime parses human dates like "now", "+2 days", "-1 hour", "2026-08-16", etc.
func ParseHumanTime(str string, base time.Time) (time.Time, error) {
	raw := strings.TrimSpace(str)
	normalized := strings.ToLower(raw)
	if normalized == "" || normalized == "now" {
		return base, nil
	}

	// Direct layout parsing
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"02/01/2006 15:04:05",
		"02/01/2006",
		"15:04:05",
		"15:04",
		time.RFC3339,
		time.RFC1123,
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, base.Location()); err == nil {
			return t, nil
		}
	}
	str = normalized

	// Relative offsets: "+2 days", "-1 week", "+30 minutes", "next month", "yesterday", "tomorrow"
	switch str {
	case "yesterday":
		return base.AddDate(0, 0, -1), nil
	case "tomorrow":
		return base.AddDate(0, 0, 1), nil
	case "today":
		return time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()), nil
	}

	// Parsing "+/- N (seconds|minutes|hours|days|weeks|months|years)"
	parts := strings.Fields(str)
	if len(parts) >= 2 {
		valStr := parts[0]
		unit := parts[1]
		if parts[0] == "next" || parts[0] == "last" || parts[0] == "prev" || parts[0] == "previous" {
			multiplier := 1
			if parts[0] != "next" {
				multiplier = -1
			}
			return applyDateOffset(base, multiplier, unit)
		}

		sign := 1
		if strings.HasPrefix(valStr, "+") {
			valStr = strings.TrimPrefix(valStr, "+")
		} else if strings.HasPrefix(valStr, "-") {
			sign = -1
			valStr = strings.TrimPrefix(valStr, "-")
		}

		if n, err := strconv.Atoi(valStr); err == nil {
			return applyDateOffset(base, sign*n, unit)
		}
	}

	return base, fmt.Errorf("unable to parse date string: %s", str)
}

func applyDateOffset(base time.Time, n int, unit string) (time.Time, error) {
	unit = strings.TrimSuffix(strings.ToLower(unit), "s")
	switch unit {
	case "sec", "second":
		return base.Add(time.Duration(n) * time.Second), nil
	case "min", "minute":
		return base.Add(time.Duration(n) * time.Minute), nil
	case "hour", "hr":
		return base.Add(time.Duration(n) * time.Hour), nil
	case "day", "d":
		return base.AddDate(0, 0, n), nil
	case "week", "wk":
		return base.AddDate(0, 0, n*7), nil
	case "month", "mon":
		return base.AddDate(0, n, 0), nil
	case "year", "yr":
		return base.AddDate(n, 0, 0), nil
	default:
		return base, fmt.Errorf("unknown time unit: %s", unit)
	}
}
