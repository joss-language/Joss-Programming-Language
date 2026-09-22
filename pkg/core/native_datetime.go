package core

import (
	"fmt"
	"strings"
	"time"
)

func convertDateFormat(phpFormat string) string {
	replacements := []struct {
		php string
		goL string
	}{
		{"Y", "2006"},
		{"y", "06"},
		{"m", "01"},
		{"d", "02"},
		{"H", "15"},
		{"h", "03"},
		{"i", "04"},
		{"s", "05"},
		{"a", "pm"},
		{"A", "PM"},
	}
	res := phpFormat
	for _, r := range replacements {
		res = strings.ReplaceAll(res, r.php, r.goL)
	}
	return res
}

// executeDateTimeMethod handles methods on DateTime instances and static calls
func (r *Runtime) executeDateTimeMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "now":
		inst := &Instance{Class: r.Classes["DateTime"], Fields: make(map[string]interface{})}
		inst.Fields["_time"] = time.Now()
		return inst

	case "create":
		y, m, d := 2000, 1, 1
		h, min, sec := 0, 0, 0
		if len(args) > 0 {
			y = toIntVal(args[0])
		}
		if len(args) > 1 {
			m = toIntVal(args[1])
		}
		if len(args) > 2 {
			d = toIntVal(args[2])
		}
		if len(args) > 3 {
			h = toIntVal(args[3])
		}
		if len(args) > 4 {
			min = toIntVal(args[4])
		}
		if len(args) > 5 {
			sec = toIntVal(args[5])
		}
		t := time.Date(y, time.Month(m), d, h, min, sec, 0, time.Local)
		inst := &Instance{Class: r.Classes["DateTime"], Fields: make(map[string]interface{})}
		inst.Fields["_time"] = t
		return inst

	case "parse":
		str := ""
		if len(args) > 0 {
			str = fmt.Sprintf("%v", args[0])
		}
		layout := "2006-01-02 15:04:05"
		if len(args) > 1 {
			layout = convertDateFormat(fmt.Sprintf("%v", args[1]))
		}
		var parsed time.Time
		var err error
		layouts := []string{
			layout,
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02",
			"15:04:05",
		}
		for _, l := range layouts {
			parsed, err = time.ParseInLocation(l, str, time.Local)
			if err == nil {
				break
			}
		}
		if err != nil {
			panic(&JossError{Type: "DateTimeError", Message: fmt.Sprintf("DateTime::parse failed: %v", err), File: r.CurrentFile})
		}
		inst := &Instance{Class: r.Classes["DateTime"], Fields: make(map[string]interface{})}
		inst.Fields["_time"] = parsed
		return inst

	case "format":
		t := getDateTimeVal(instance)
		layout := "2006-01-02 15:04:05"
		if len(args) > 0 {
			layout = convertDateFormat(fmt.Sprintf("%v", args[0]))
		}
		return t.Format(layout)

	case "timestamp":
		t := getDateTimeVal(instance)
		return t.Unix()

	case "iso":
		t := getDateTimeVal(instance)
		return t.Format(time.RFC3339)

	case "year":
		return int64(getDateTimeVal(instance).Year())

	case "month":
		return int64(getDateTimeVal(instance).Month())

	case "day":
		return int64(getDateTimeVal(instance).Day())

	case "hour":
		return int64(getDateTimeVal(instance).Hour())

	case "minute":
		return int64(getDateTimeVal(instance).Minute())

	case "second":
		return int64(getDateTimeVal(instance).Second())

	case "add":
		t := getDateTimeVal(instance)
		dur := getDateIntervalVal(args)
		newInst := &Instance{Class: r.Classes["DateTime"], Fields: make(map[string]interface{})}
		newInst.Fields["_time"] = t.Add(dur)
		return newInst

	case "sub":
		t := getDateTimeVal(instance)
		dur := getDateIntervalVal(args)
		newInst := &Instance{Class: r.Classes["DateTime"], Fields: make(map[string]interface{})}
		newInst.Fields["_time"] = t.Add(-dur)
		return newInst

	case "diff":
		t1 := getDateTimeVal(instance)
		var t2 time.Time
		if len(args) > 0 {
			if other, ok := args[0].(*Instance); ok {
				if ot, ok := other.Fields["_time"].(time.Time); ok {
					t2 = ot
				}
			}
		}
		diff := t1.Sub(t2)
		if diff < 0 {
			diff = -diff
		}
		interval := &Instance{Class: r.Classes["DateInterval"], Fields: make(map[string]interface{})}
		interval.Fields["_duration"] = diff
		return interval
	}
	return nil
}

func getDateTimeVal(inst *Instance) time.Time {
	if inst != nil {
		if t, ok := inst.Fields["_time"].(time.Time); ok {
			return t
		}
	}
	return time.Now()
}

func getDateIntervalVal(args []interface{}) time.Duration {
	if len(args) > 0 {
		if intervalInst, ok := args[0].(*Instance); ok {
			if dur, ok := intervalInst.Fields["_duration"].(time.Duration); ok {
				return dur
			}
		} else if sec, ok := args[0].(int64); ok {
			return time.Duration(sec) * time.Second
		} else if sec, ok := args[0].(int); ok {
			return time.Duration(sec) * time.Second
		}
	}
	return 0
}

func toIntVal(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// executeDateIntervalMethod handles methods on DateInterval
func (r *Runtime) executeDateIntervalMethod(instance *Instance, method string, args []interface{}) interface{} {
	makeInterval := func(dur time.Duration) *Instance {
		inst := &Instance{Class: r.Classes["DateInterval"], Fields: make(map[string]interface{})}
		inst.Fields["_duration"] = dur
		return inst
	}

	switch method {
	case "days":
		n := 1
		if len(args) > 0 {
			n = toIntVal(args[0])
		}
		return makeInterval(time.Duration(n) * 24 * time.Hour)

	case "hours":
		n := 1
		if len(args) > 0 {
			n = toIntVal(args[0])
		}
		return makeInterval(time.Duration(n) * time.Hour)

	case "minutes":
		n := 1
		if len(args) > 0 {
			n = toIntVal(args[0])
		}
		return makeInterval(time.Duration(n) * time.Minute)

	case "seconds":
		n := 1
		if len(args) > 0 {
			n = toIntVal(args[0])
		}
		return makeInterval(time.Duration(n) * time.Second)

	case "totalSeconds":
		var dur time.Duration
		if instance != nil {
			if d, ok := instance.Fields["_duration"].(time.Duration); ok {
				dur = d
			}
		}
		return int64(dur.Seconds())
	}
	return nil
}
