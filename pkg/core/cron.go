package core

import (
	"database/sql"
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
)

// Cron Implementation (Daemon mode simulation)
func (r *Runtime) executeCronMethod(instance *Instance, method string, args []interface{}) interface{} {
	if method == "schedule" {
		if len(args) >= 3 {
			name := args[0].(string)
			schedule := args[1].(string)

			if block, ok := args[2].(*parser.BlockStatement); ok {
				scheduledTasksMu.Lock()
				scheduledTasks[name] = block
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
			}
		}
	}
	return nil
}
