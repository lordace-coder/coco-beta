package functions

import (
	"log"
	"sync"
	"time"

	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/robfig/cron/v3"
)

var (
	scheduler     *cron.Cron
	schedulerOnce sync.Once

	// cronEntries maps function ID → cron entry ID so we can remove/replace them.
	cronMu      sync.Mutex
	cronEntries = map[string]cron.EntryID{}
)

// StartScheduler initialises the global cron scheduler and loads all enabled cron
// functions from the database. Also registers built-in system jobs.
func StartScheduler() {
	schedulerOnce.Do(func() {
		scheduler = cron.New(cron.WithSeconds()) // allow optional seconds field
		registerSystemJobs()
		loadUserCronFunctions()
		scheduler.Start()
		log.Println("⏰ Cron scheduler started")
	})
}

// StopScheduler shuts down the cron scheduler gracefully.
func StopScheduler() {
	if scheduler != nil {
		ctx := scheduler.Stop()
		<-ctx.Done()
	}
}

// registerSystemJobs adds built-in maintenance jobs.
func registerSystemJobs() {
	// Every 5 minutes: evict expired KV entries
	scheduler.AddFunc("0 */5 * * * *", func() {
		n := EvictAllExpired()
		if n > 0 {
			log.Printf("[system-cron] evicted %d expired KV entries", n)
		}
	})

	// Every hour: clear GORM collection name→ID cache
	scheduler.AddFunc("0 0 * * * *", func() {
		database.ClearCollectionCache()
		log.Println("[system-cron] cleared collection cache")
	})

	// Every 24 hours: delete expired tokens from DB
	scheduler.AddFunc("0 0 3 * * *", func() { // 03:00 daily
		cleanExpiredTokens()
	})
}

func cleanExpiredTokens() {
	now := time.Now()
	db := database.DB
	db.Where("expires_at < ?", now).Delete(&models.PasswordResetToken{})
	db.Where("expires_at < ?", now).Delete(&models.EmailVerificationToken{})
	db.Where("expires_at < ?", now).Delete(&models.TwoFactorCode{})
	log.Println("[system-cron] cleaned expired tokens")
}

// loadUserCronFunctions loads all enabled cron functions from the DB and schedules them.
func loadUserCronFunctions() {
	var fns []models.Function
	database.DB.Where("trigger_type = ? AND enabled = true", models.TriggerCron).Find(&fns)
	for i := range fns {
		scheduleCronFunction(&fns[i])
	}
	log.Printf("[cron] loaded %d user cron function(s)", len(fns))
}

// scheduleCronFunction adds (or replaces) a cron entry for fn.
func scheduleCronFunction(fn *models.Function) {
	schedule := fn.TriggerConfig.Schedule
	if schedule == "" {
		return
	}

	cronMu.Lock()
	defer cronMu.Unlock()

	// Remove old entry if it exists
	if id, ok := cronEntries[fn.ID]; ok {
		scheduler.Remove(id)
	}

	fnCopy := *fn
	entryID, err := scheduler.AddFunc(schedule, func() {
		rctx := &RunContext{
			ProjectID: fnCopy.ProjectID,
			ReqMethod: "CRON",
		}
		if err := Execute(&fnCopy, rctx); err != nil {
			log.Printf("[cron] %s error: %v", fnCopy.Name, err)
		}
	})
	if err != nil {
		log.Printf("[cron] invalid schedule %q for %s: %v", schedule, fn.Name, err)
		return
	}
	cronEntries[fn.ID] = entryID
	log.Printf("[cron] scheduled %q (%s)", fn.Name, schedule)
}

// UnscheduleCronFunction removes a function's cron entry.
func UnscheduleCronFunction(functionID string) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if id, ok := cronEntries[functionID]; ok {
		scheduler.Remove(id)
		delete(cronEntries, functionID)
	}
}

// ReloadCronFunction re-schedules a function after it has been updated.
func ReloadCronFunction(fn *models.Function) {
	UnscheduleCronFunction(fn.ID)
	if fn.Enabled && fn.TriggerType == models.TriggerCron {
		scheduleCronFunction(fn)
	}
}

// CronEntries returns a snapshot of all scheduled cron entries (for the dashboard).
type CronEntry struct {
	FunctionID string    `json:"function_id"`
	Schedule   string    `json:"schedule"`
	Next       time.Time `json:"next_run"`
	Prev       time.Time `json:"prev_run"`
}

func GetCronEntries(projectID string) []CronEntry {
	if scheduler == nil {
		return nil
	}
	// Get all DB functions for this project
	var fns []models.Function
	database.DB.Where("project_id = ? AND trigger_type = ? AND enabled = true", projectID, models.TriggerCron).Find(&fns)

	fnMap := map[string]models.Function{}
	for _, f := range fns {
		fnMap[f.ID] = f
	}

	cronMu.Lock()
	defer cronMu.Unlock()

	var entries []CronEntry
	for _, e := range scheduler.Entries() {
		// Find which function this entry belongs to
		for fnID, entryID := range cronEntries {
			if entryID == e.ID {
				fn, ok := fnMap[fnID]
				if !ok {
					break
				}
				if fn.ProjectID != projectID {
					break
				}
				entries = append(entries, CronEntry{
					FunctionID: fnID,
					Schedule:   fn.TriggerConfig.Schedule,
					Next:       e.Next,
					Prev:       e.Prev,
				})
				break
			}
		}
	}
	return entries
}

// RunNow executes a cron function immediately (called from dashboard "Run now" button).
func RunNow(fn *models.Function) error {
	rctx := &RunContext{
		ProjectID: fn.ProjectID,
		ReqMethod: "MANUAL",
	}
	return Execute(fn, rctx)
}
