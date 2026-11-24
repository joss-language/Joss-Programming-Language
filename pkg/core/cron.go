package core

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

var (
	scheduledClosures = make(map[string]*CapturedFunction)
	cronAutoSeq       int64
)

// Cron Implementation (Daemon mode simulation)
func (r *Runtime) executeCronMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "schedule":
		var name, schedule string
		var target interface{}
		if len(args) >= 3 {
			name, _ = args[0].(string)
			schedule, _ = args[1].(string)
			target = args[2]
		} else if len(args) == 2 {
			schedule, _ = args[0].(string)
			target = args[1]
		} else {
			return nil
		}

		var block *parser.BlockStatement
		var closure *CapturedFunction
		switch cb := target.(type) {
		case *parser.BlockStatement:
			block = cb
		case *CapturedFunction:
			closure = cb
			if cb.Function != nil {
				block = cb.Function.Body
			}
		case *parser.FunctionLiteral:
			closure = r.captureFunction(cb)
			block = cb.Body
		default:
			return nil
		}

		scheduledTasksMu.Lock()
		if name == "" {
			cronAutoSeq++
			name = fmt.Sprintf("cron_%d", cronAutoSeq)
		}
		if block != nil {
			scheduledTasks[name] = block
		} else {
			delete(scheduledTasks, name)
		}
		if closure != nil {
			scheduledClosures[name] = closure
		} else {
			delete(scheduledClosures, name)
		}
		scheduledSchedules[name] = schedule
		scheduledTasksMu.Unlock()

		// 1. Register/Update Task in DB if connected
		if r.GetDB() != nil {
			r.EnsureCronTable()
			prefix := r.dbPrefix()
			tableName := prefix + "cron"

			var id int
			err := r.databaseExecutor().QueryRow(fmt.Sprintf("SELECT id FROM %s WHERE name = ?", tableName), name).Scan(&id)
			if err == sql.ErrNoRows {
				_, err = r.databaseExecutor().Exec(fmt.Sprintf("INSERT INTO %s (name, schedule, status) VALUES (?, ?, 'idle')", tableName), name, schedule)
			} else if err == nil {
				_, err = r.databaseExecutor().Exec(fmt.Sprintf("UPDATE %s SET schedule = ? WHERE id = ?", tableName), schedule, id)
			}
			if err != nil {
				fmt.Printf("[Cron] Error registrando tarea %s en DB: %v\n", name, err)
			}
		}

		cronTickerMutex.Lock()
		if !cronTickerStarted {
			cronTickerStarted = true
			go r.StartCronTicker()
		}
		cronTickerMutex.Unlock()
		return nil

	case "runDue":
		type dueEntry struct {
			name    string
			block   *parser.BlockStatement
			closure *CapturedFunction
		}
		now := time.Now()
		scheduledTasksMu.RLock()
		due := make([]dueEntry, 0, len(scheduledSchedules))
		for name, expr := range scheduledSchedules {
			if expr == "" || !MatchCron(expr, now) {
				continue
			}
			blk := scheduledTasks[name]
			cls := scheduledClosures[name]
			if cls != nil || blk != nil {
				due = append(due, dueEntry{name: name, block: blk, closure: cls})
			}
		}
		scheduledTasksMu.RUnlock()

		executed := int64(0)
		for _, entry := range due {
			if entry.closure != nil {
				r.callCapturedFunction(entry.closure, nil)
				executed++
			} else if entry.block != nil {
				r.executeBlock(entry.block)
				executed++
			}
		}
		return executed
	}
	return nil
}
