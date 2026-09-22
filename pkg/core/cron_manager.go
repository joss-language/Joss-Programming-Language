package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

var (
	cronTickerStarted  bool
	cronTickerMutex    sync.Mutex
	scheduledTasks     = make(map[string]*parser.BlockStatement)
	scheduledSchedules = make(map[string]string)
	scheduledTasksMu   sync.RWMutex
	inMemoryRunning    = make(map[string]bool)
	inMemoryRunningMu  sync.Mutex
	cronDBResetOnce    sync.Once
)

// EnsureCronTable creates the cron table if it doesn't exist
func (r *Runtime) EnsureCronTable() {
	if r.GetDB() == nil {
		return
	}

	if err := r.ensureInternalSchemaTable("cron", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "name", definition: "string(255)|unique"},
		{name: "schedule", definition: "string(255)"},
		{name: "last_run_at", definition: "timestamp|nullable"},
		{name: "is_running", definition: "boolean|default(0)"},
		{name: "status", definition: "string(50)|nullable"},
	}); err != nil {
		fmt.Printf("[Cron] Error creando tabla interna: %v\n", err)
	}
}

// StartCronTicker starts the background evaluation loop
func (r *Runtime) StartCronTicker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	fmt.Println("[Cron] Ticker de tareas programadas iniciado.")

	for range ticker.C {
		r.TickCron()
	}
}

// TickCron checks scheduled tasks against the current time
func (r *Runtime) TickCron() {
	now := time.Now()

	// 1. If DB is available, sync and run using DB locking
	if r.GetDB() != nil {
		r.EnsureCronTable()
		prefix := r.dbPrefix()
		tableName := prefix + "cron"

		// Reset stale locks once on start
		cronDBResetOnce.Do(func() {
			_, _ = r.databaseExecutor().Exec(fmt.Sprintf("UPDATE %s SET is_running = 0 WHERE is_running = 1", tableName))
		})

		// Auto-sync in-memory registered tasks to DB
		scheduledTasksMu.RLock()
		for name, expr := range scheduledSchedules {
			var exists int
			err := r.databaseExecutor().QueryRow(fmt.Sprintf("SELECT 1 FROM %s WHERE name = ?", tableName), name).Scan(&exists)
			if err != nil {
				_, _ = r.databaseExecutor().Exec(fmt.Sprintf("INSERT INTO %s (name, schedule, status) VALUES (?, ?, 'idle')", tableName), name, expr)
			}
		}
		scheduledTasksMu.RUnlock()

		rows, err := r.databaseExecutor().Query(fmt.Sprintf("SELECT name, schedule, is_running FROM %s", tableName))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name, expr string
				var isRunning bool
				if err := rows.Scan(&name, &expr, &isRunning); err != nil {
					continue
				}

				if isRunning {
					continue
				}

				if MatchCron(expr, now) {
					scheduledTasksMu.RLock()
					block, exists := scheduledTasks[name]
					scheduledTasksMu.RUnlock()

					if exists {
						fmt.Printf("[Cron] Disparando tarea '%s' programada (%s)...\n", name, expr)
						r.RunCronTask(name, block)
					}
				}
			}
			return
		}
	}

	// 2. In-memory fallback (when DB is not connected)
	scheduledTasksMu.RLock()
	tasksCopy := make(map[string]*parser.BlockStatement, len(scheduledTasks))
	exprsCopy := make(map[string]string, len(scheduledSchedules))
	for k, v := range scheduledTasks {
		tasksCopy[k] = v
		exprsCopy[k] = scheduledSchedules[k]
	}
	scheduledTasksMu.RUnlock()

	for name, block := range tasksCopy {
		expr := exprsCopy[name]
		if expr == "" {
			continue
		}

		inMemoryRunningMu.Lock()
		running := inMemoryRunning[name]
		inMemoryRunningMu.Unlock()

		if running {
			continue
		}

		if MatchCron(expr, now) {
			fmt.Printf("[Cron] Disparando tarea in-memory '%s' (%s)...\n", name, expr)
			inMemoryRunningMu.Lock()
			inMemoryRunning[name] = true
			inMemoryRunningMu.Unlock()

			newR := r.Fork()
			go func(taskName string, blk *parser.BlockStatement) {
				defer func() {
					if rec := recover(); rec != nil {
						fmt.Printf("[Cron] Error en tarea in-memory %s: %v\n", taskName, rec)
					}
					newR.Free()
					inMemoryRunningMu.Lock()
					inMemoryRunning[taskName] = false
					inMemoryRunningMu.Unlock()
				}()
				newR.executeBlock(blk)
			}(name, block)
		}
	}
}

// RunCronTask handles execution safety and locking
func (r *Runtime) RunCronTask(name string, block *parser.BlockStatement) {
	prefix := r.dbPrefix()
	tableName := prefix + "cron"

	// Lock task
	_, err := r.databaseExecutor().Exec(fmt.Sprintf("UPDATE %s SET is_running = 1, status = 'running' WHERE name = ?", tableName), name)
	if err != nil {
		return
	}

	newR := r.Fork()
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Printf("[Cron] Error en tarea %s: %v\n", name, rec)
				if r.GetDB() != nil {
					_, _ = r.databaseExecutor().Exec(fmt.Sprintf("UPDATE %s SET is_running = 0, status = 'error', last_run_at = CURRENT_TIMESTAMP WHERE name = ?", tableName), name)
				}
			} else {
				if r.GetDB() != nil {
					_, _ = r.databaseExecutor().Exec(fmt.Sprintf("UPDATE %s SET is_running = 0, status = 'completed', last_run_at = CURRENT_TIMESTAMP WHERE name = ?", tableName), name)
				}
			}
			newR.Free()
		}()
		newR.executeBlock(block)
	}()
}

// MatchCron evaluates standard cron syntax
func MatchCron(expr string, t time.Time) bool {
	// Support friendly aliases
	expr = strings.TrimSpace(strings.ToLower(expr))
	if expr == "hourly" {
		expr = "0 * * * *"
	} else if expr == "daily" {
		expr = "0 0 * * *"
	} else if expr == "weekly" {
		expr = "0 0 * * 0"
	} else if expr == "monthly" {
		expr = "0 0 1 * *"
	}

	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return false
	}

	min := t.Minute()
	hour := t.Hour()
	dom := t.Day()
	month := int(t.Month())
	dow := int(t.Weekday()) // 0 = Sunday, 1 = Monday, ...

	return matchCronField(parts[0], min, 0, 59) &&
		matchCronField(parts[1], hour, 0, 23) &&
		matchCronField(parts[2], dom, 1, 31) &&
		matchCronField(parts[3], month, 1, 12) &&
		matchCronField(parts[4], dow, 0, 6)
}

func matchCronField(field string, val int, minVal, maxVal int) bool {
	if field == "*" {
		return true
	}

	// Step (*/X)
	if strings.HasPrefix(field, "*/") {
		var step int
		_, err := fmt.Sscanf(field, "*/%d", &step)
		if err == nil && step > 0 {
			return val%step == 0
		}
	}

	// List (e.g. 0,12)
	parts := strings.Split(field, ",")
	if len(parts) > 1 {
		for _, p := range parts {
			var item int
			if _, err := fmt.Sscanf(p, "%d", &item); err == nil {
				if item == val {
					return true
				}
			}
		}
		return false
	}

	// Exact number
	var item int
	if _, err := fmt.Sscanf(field, "%d", &item); err == nil {
		return item == val
	}

	return false
}
