package core

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
)

func (r *Runtime) executeMemoryMethod(inst *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "rss":
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return int64(ms.Sys)
	case "arenaUsed":
		return int64(0)
	case "gcStats":
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return map[string]interface{}{
			"objects_live":    int64(ms.HeapObjects),
			"bytes_live":      int64(ms.HeapAlloc),
			"total_allocs":    int64(ms.Mallocs),
			"total_freed":     int64(ms.Frees),
			"collections":     int64(ms.NumGC),
			"threshold_bytes": int64(ms.NextGC),
		}
	case "gcCollect":
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		runtime.GC()
		runtime.ReadMemStats(&after)
		if before.HeapObjects > after.HeapObjects {
			return int64(before.HeapObjects - after.HeapObjects)
		}
		return int64(0)
	case "trim":
		debug.FreeOSMemory()
		return true
	case "sizeOf":
		if len(args) == 0 || args[0] == nil {
			return int64(0)
		}
		switch v := args[0].(type) {
		case string:
			return int64(len(v) + 1)
		case []interface{}:
			return int64(32 + len(v)*8)
		case map[string]interface{}:
			return int64(64 + len(v)*32)
		case *Instance:
			return int64(40 + len(v.Fields)*8)
		default:
			return int64(8)
		}
	case "snapshot":
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return map[string]interface{}{
			"rss_bytes":       int64(ms.Sys),
			"arena_bytes":     int64(0),
			"gc_bytes_live":   int64(ms.HeapAlloc),
			"gc_objects_live": int64(ms.HeapObjects),
			"gc_collections":  int64(ms.NumGC),
		}
	case "diff":
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		var bRss, bArena, bGcBytes, bGcObjs int64
		if len(args) > 0 {
			if m, ok := args[0].(map[string]interface{}); ok {
				bRss = toInt64Mem(m["rss_bytes"])
				bArena = toInt64Mem(m["arena_bytes"])
				bGcBytes = toInt64Mem(m["gc_bytes_live"])
				bGcObjs = toInt64Mem(m["gc_objects_live"])
			}
		}
		return map[string]interface{}{
			"rss_delta":        int64(ms.Sys) - bRss,
			"arena_delta":      int64(0) - bArena,
			"gc_bytes_delta":   int64(ms.HeapAlloc) - bGcBytes,
			"gc_objects_delta": int64(ms.HeapObjects) - bGcObjs,
		}
	case "report":
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return fmt.Sprintf("Memory[RSS: %.2f MB | Arena: 0 KB | GC live: %d objs (%.2f KB) | GC runs: %d]",
			float64(ms.Sys)/(1024*1024), ms.HeapObjects, float64(ms.HeapAlloc)/1024.0, ms.NumGC)
	case "checkGrowth":
		if len(args) < 2 {
			return true
		}
		maxDelta := toInt64Mem(args[1])
		label := "memory_check"
		if len(args) >= 3 && args[2] != nil {
			if s, ok := args[2].(string); ok && s != "" {
				label = s
			}
		}
		diffMap := r.executeMemoryMethod(inst, "diff", args[:1]).(map[string]interface{})
		rssDelta := toInt64Mem(diffMap["rss_delta"])
		gcDelta := toInt64Mem(diffMap["gc_bytes_delta"])
		if rssDelta > maxDelta || gcDelta > maxDelta {
			fmt.Fprintf(os.Stderr,
				"Advertencia ⚠️ se ha detectado falla en la memoria (%s) en [RSS: %+d KB | GC live: %+d KB | Limite: %d KB], se está detectando un crecimiento.\n",
				label, rssDelta/1024, gcDelta/1024, maxDelta/1024)
			return false
		}
		return true
	}
	return nil
}

func toInt64Mem(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}
