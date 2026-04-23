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

	// projectCronEntries maps "projectID:idx" → cron entry ID.
	// When functions.js is reloaded we remove old entries and add new ones.
	cronMu             sync.Mutex
	projectCronEntries = map[string][]cron.EntryID{} // projectID → entry IDs
	projectCronMtimes  = map[string]time.Time{}       // projectID → last loaded mtime
)

// StartScheduler initialises the global cron scheduler, registers system jobs,
// and loads user cron jobs from each project's functions.js.
func StartScheduler() {
	schedulerOnce.Do(func() {
		scheduler = cron.New(cron.WithSeconds())
		registerSystemJobs()
		loadAllProjectCronFunctions()
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
	scheduler.AddFunc("0 0 3 * * *", func() {
		cleanExpiredTokens()
	})

	// Every minute: reload cron jobs from functions.js if the file changed
	scheduler.AddFunc("0 * * * * *", func() {
		reloadChangedProjectCrons()
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

// loadAllProjectCronFunctions loads cron jobs for every project at startup.
func loadAllProjectCronFunctions() {
	var projects []models.Project
	database.DB.Find(&projects)
	for _, p := range projects {
		loadProjectCronFunctions(p.ID, p.Name)
	}
}

// reloadChangedProjectCrons checks each project's functions.js mtime and
// re-registers cron jobs if the file has changed since last load.
func reloadChangedProjectCrons() {
	var projects []models.Project
	database.DB.Find(&projects)
	for _, p := range projects {
		_, mtime := ReadProjectCode(p.ID)
		cronMu.Lock()
		last := projectCronMtimes[p.ID]
		cronMu.Unlock()
		if !mtime.Equal(last) {
			loadProjectCronFunctions(p.ID, p.Name)
		}
	}
}

// loadProjectCronFunctions reads the cron jobs from a project's functions.js
// and registers them in the scheduler, replacing any previous entries.
func loadProjectCronFunctions(projectID, projectName string) {
	jobs := GetCronJobs(projectID, projectName)

	cronMu.Lock()
	defer cronMu.Unlock()

	// Remove old entries for this project
	for _, entryID := range projectCronEntries[projectID] {
		scheduler.Remove(entryID)
	}
	projectCronEntries[projectID] = nil

	_, mtime := ReadProjectCode(projectID)
	projectCronMtimes[projectID] = mtime

	for i, job := range jobs {
		idx := i // capture for closure
		pid := projectID
		pname := projectName
		entryID, err := scheduler.AddFunc(job.schedule, func() {
			RunCronJob(pid, pname, idx)
		})
		if err != nil {
			log.Printf("[cron:%s] invalid schedule %q at index %d: %v", safePrefix(projectID), job.schedule, idx, err)
			continue
		}
		projectCronEntries[projectID] = append(projectCronEntries[projectID], entryID)
	}

	if len(jobs) > 0 {
		log.Printf("[cron:%s] registered %d cron job(s)", safePrefix(projectID), len(jobs))
	}
}

// ReloadProjectCrons forces a reload of cron jobs for a project.
// Call this after functions.js is written programmatically.
func ReloadProjectCrons(projectID, projectName string) {
	InvalidateRegistry(projectID)
	loadProjectCronFunctions(projectID, projectName)
}

// ── Legacy API — kept so dashboard handlers compile ──────────────────────────
// These are no-ops or thin wrappers now that crons come from functions.js.

// CronEntry describes one scheduled cron entry (for dashboard display).
type CronEntry struct {
	FunctionID string    `json:"function_id"`
	Schedule   string    `json:"schedule"`
	Next       time.Time `json:"next_run"`
	Prev       time.Time `json:"prev_run"`
}

// GetCronEntries returns scheduled cron entries for a project (for the dashboard).
func GetCronEntries(projectID string) []CronEntry {
	if scheduler == nil {
		return nil
	}
	cronMu.Lock()
	entryIDs := projectCronEntries[projectID]
	cronMu.Unlock()

	idSet := map[cron.EntryID]int{}
	for i, id := range entryIDs {
		idSet[id] = i
	}

	// Get schedule strings from the live registry
	jobs := GetCronJobs(projectID, "")

	var entries []CronEntry
	for _, e := range scheduler.Entries() {
		if idx, ok := idSet[e.ID]; ok {
			schedule := ""
			if idx < len(jobs) {
				schedule = jobs[idx].schedule
			}
			entries = append(entries, CronEntry{
				FunctionID: projectID,
				Schedule:   schedule,
				Next:       e.Next,
				Prev:       e.Prev,
			})
		}
	}
	return entries
}

// ReloadCronFunction is a no-op — crons are now loaded from functions.js.
func ReloadCronFunction(_ *models.Function) {}

// UnscheduleCronFunction is a no-op — use ReloadProjectCrons instead.
func UnscheduleCronFunction(_ string) {}
