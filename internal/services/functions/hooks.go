package functions

import (
	"log"

	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
)

// DispatchHook runs all hook handlers registered in functions.js for the given
// lifecycle event and collection.
//
// For "before" hooks: runs synchronously; if any handler calls ctx.cancel()
// returns (true, cancelMessage).
// For "after" hooks: runs in a background goroutine, always returns (false, "").
func DispatchHook(
	event models.HookEvent,
	projectID string,
	collection *models.Collection,
	doc map[string]interface{},
	user *models.AppUser,
	broadcast func(string, interface{}),
) (cancelled bool, cancelMsg string) {

	// Look up the project name for the registry (needed to load functions.js)
	var project models.Project
	if err := database.DB.Select("id, name").First(&project, "id = ?", projectID).Error; err != nil {
		return false, ""
	}

	colName := ""
	if collection != nil {
		colName = collection.Name
	}

	isBefore := event == models.HookBeforeCreate ||
		event == models.HookBeforeUpdate ||
		event == models.HookBeforeDelete

	rctx := &RunContext{
		ProjectID:   projectID,
		ProjectName: project.Name,
		Doc:         doc,
		User:        user,
		Broadcast:   broadcast,
		ReqMethod:   string(event),
	}

	if isBefore {
		if err := dispatchHookSync(projectID, project.Name, string(event), colName, rctx); err != nil {
			log.Printf("hook %s/%s error: %v", event, colName, err)
		}
		return rctx.Cancelled, rctx.CancelMessage
	}

	// After hooks: fire-and-forget
	rctxCopy := &RunContext{
		ProjectID:   projectID,
		ProjectName: project.Name,
		Doc:         copyMap(doc),
		User:        user,
		Broadcast:   broadcast,
		ReqMethod:   string(event),
	}
	go func() {
		if err := dispatchHookSync(projectID, project.Name, string(event), colName, rctxCopy); err != nil {
			log.Printf("after-hook %s/%s error: %v", event, colName, err)
		}
	}()

	return false, ""
}

// dispatchHookSync calls all matching handlers in the registry synchronously.
func dispatchHookSync(projectID, projectName, event, collection string, rctx *RunContext) error {
	return dispatchHookInner(projectID, projectName, event, collection, rctx)
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
