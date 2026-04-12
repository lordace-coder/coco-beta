package functions

import (
	"fmt"
	"log"
	"time"

	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
)

const maxStoredLogs = 20

// Execute runs a function, records its result in the DB log, and returns any error.
// It always reads code from disk first (./functions/<projectID>/<name>.js),
// falling back to the inline code stored in the database if the file is missing.
func Execute(fn *models.Function, rctx *RunContext) (err error) {
	if !fn.Enabled {
		return fmt.Errorf("function %q is disabled", fn.Name)
	}

	// Load code from file — disk is the source of truth; DB code is the fallback.
	// We also record the file's mod time so the compile cache busts automatically
	// whenever the user saves their editor — no server restart needed.
	fn.Code, fn.UpdatedAt = ReadFunctionCodeWithMtime(fn.ProjectID, fn.Name, fn.Code, fn.UpdatedAt)

	timeout := time.Duration(fn.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	start := time.Now()

	// Recover from panics so one bad function can't crash the server
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	runErr := RunFunction(fn, rctx, timeout)

	duration := time.Since(start).Milliseconds()
	success := runErr == nil
	errStr := ""
	if runErr != nil {
		errStr = runErr.Error()
		err = runErr
	}

	entry := models.FunctionLog{
		RunAt:    start,
		Duration: duration,
		Success:  success,
		Output:   rctx.LogOutput.String(),
		Error:    errStr,
	}

	// Append log entry, keep only last maxStoredLogs
	logs := append(fn.Logs, entry)
	if len(logs) > maxStoredLogs {
		logs = logs[len(logs)-maxStoredLogs:]
	}

	now := time.Now()
	updateData := map[string]interface{}{
		"logs":        logs,
		"last_run_at": &now,
		"last_error":  errStr,
	}
	if dbErr := database.DB.Model(fn).Updates(updateData).Error; dbErr != nil {
		log.Printf("functions: failed to save log for %s: %v", fn.Name, dbErr)
	}

	if runErr != nil {
		log.Printf("functions: [%s] error: %v", fn.Name, runErr)
	} else {
		log.Printf("functions: [%s] ok (%dms)", fn.Name, duration)
	}

	return err
}

// ExecuteByID loads a function from the DB and runs it.
func ExecuteByID(functionID, projectID string, rctx *RunContext) error {
	var fn models.Function
	if err := database.DB.
		Where("id = ? AND project_id = ?", functionID, projectID).
		First(&fn).Error; err != nil {
		return fmt.Errorf("function not found: %w", err)
	}
	return Execute(&fn, rctx)
}
