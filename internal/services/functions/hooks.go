package functions

import (
	"log"

	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
)

// DispatchHook runs all enabled hook functions for the given event/collection.
// For "before" hooks: if any function calls ctx.cancel(), returns (true, message).
// For "after" hooks: runs in background goroutines, return value is always (false, "").
func DispatchHook(
	event models.HookEvent,
	projectID string,
	collection *models.Collection,
	doc map[string]interface{},
	user *models.AppUser,
	broadcast func(string, interface{}),
) (cancelled bool, cancelMsg string) {

	var fns []models.Function
	database.DB.Where(
		"project_id = ? AND trigger_type = ? AND enabled = true",
		projectID, models.TriggerHook,
	).Find(&fns)

	if len(fns) == 0 {
		return false, ""
	}

	colName := ""
	colID := ""
	if collection != nil {
		colName = collection.Name
		colID = collection.ID
	}

	isBefore := event == models.HookBeforeCreate ||
		event == models.HookBeforeUpdate ||
		event == models.HookBeforeDelete

	for i := range fns {
		fn := &fns[i]
		cfg := fn.TriggerConfig

		// Filter: must match event
		if cfg.Event != string(event) {
			continue
		}
		// Filter: must match collection (empty = all collections)
		if cfg.Collection != "" && cfg.Collection != colName && cfg.Collection != colID {
			continue
		}

		rctx := &RunContext{
			ProjectID: projectID,
			Doc:       doc,
			User:      user,
			Broadcast: broadcast,
			ReqMethod: string(event),
		}

		if isBefore {
			// Run synchronously so we can cancel
			if err := Execute(fn, rctx); err != nil {
				log.Printf("hook %s error: %v", fn.Name, err)
			}
			if rctx.Cancelled {
				return true, rctx.CancelMessage
			}
		} else {
			// After hooks: fire-and-forget
			fnCopy := *fn
			rctxCopy := &RunContext{
				ProjectID: projectID,
				Doc:       copyMap(doc),
				User:      user,
				Broadcast: broadcast,
				ReqMethod: string(event),
			}
			go func() {
				if err := Execute(&fnCopy, rctxCopy); err != nil {
					log.Printf("after-hook %s error: %v", fnCopy.Name, err)
				}
			}()
		}
	}

	return false, ""
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
