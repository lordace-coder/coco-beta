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

// DispatchAppUserHook runs hooks for the special "app_users" pseudo-collection.
//
// Unlike DispatchHook, the caller passes projectName directly (no DB lookup),
// and doc is mutated in-place by JS (via ctx.doc field assignments), letting
// before-hooks edit user fields before they are saved.
//
// For "before" hooks: synchronous — returns (cancelled, cancelMsg, mutatedDoc).
// For "after" hooks: fire-and-forget — returns (false, "", doc).
func DispatchAppUserHook(
	event models.HookEvent,
	projectID, projectName string,
	doc map[string]interface{},
	user *models.AppUser,
	broadcast func(string, interface{}),
) (cancelled bool, cancelMsg string, mutatedDoc map[string]interface{}) {
	const colName = "app_users"

	isBefore := event == models.HookBeforeCreate ||
		event == models.HookBeforeUpdate ||
		event == models.HookBeforeDelete

	rctx := &RunContext{
		ProjectID:   projectID,
		ProjectName: projectName,
		Doc:         doc,
		User:        user,
		Broadcast:   broadcast,
		ReqMethod:   string(event),
	}

	if isBefore {
		if err := dispatchHookSync(projectID, projectName, string(event), colName, rctx); err != nil {
			log.Printf("app_users hook %s error: %v", event, err)
		}
		return rctx.Cancelled, rctx.CancelMessage, rctx.Doc
	}

	// After hooks: fire-and-forget
	rctxCopy := &RunContext{
		ProjectID:   projectID,
		ProjectName: projectName,
		Doc:         copyMap(doc),
		User:        user,
		Broadcast:   broadcast,
		ReqMethod:   string(event),
	}
	go func() {
		if err := dispatchHookSync(projectID, projectName, string(event), colName, rctxCopy); err != nil {
			log.Printf("app_users after-hook %s error: %v", event, err)
		}
	}()

	return false, "", doc
}

// dispatchHookSync calls all matching handlers in the registry synchronously.
func dispatchHookSync(projectID, projectName, event, collection string, rctx *RunContext) error {
	return dispatchHookInner(projectID, projectName, event, collection, rctx)
}

// AppUserToHookDoc builds the mutable doc map passed to app_user hooks.
// The map is the source of truth during before-hooks; call ApplyHookDocToUser
// after the hook returns to write any JS mutations back to the user struct.
func AppUserToHookDoc(u *models.AppUser) map[string]interface{} {
	doc := map[string]interface{}{
		"id":             u.ID,
		"email":          u.Email,
		"roles":          []interface{}{},
		"email_verified": u.EmailVerified,
		"created_at":     u.CreatedAt,
	}
	// expose roles as []interface{} so JS can read/write them
	roles := make([]interface{}, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = r
	}
	doc["roles"] = roles

	// expose custom data fields at top level (same as userToMap)
	reserved := map[string]bool{"id": true, "email": true, "roles": true, "email_verified": true, "created_at": true}
	for k, v := range u.Data {
		if !reserved[k] {
			doc[k] = v
		}
	}
	return doc
}

// ApplyHookDocToUser writes hook-mutated doc fields back to the AppUser struct.
// Only writable fields (email, roles, custom data) are copied; id and
// email_verified are ignored so hooks cannot forge them.
func ApplyHookDocToUser(doc map[string]interface{}, u *models.AppUser) {
	if email, ok := doc["email"].(string); ok && email != "" {
		u.Email = email
	}
	if roles, ok := doc["roles"].([]interface{}); ok {
		u.Roles = models.StringArray{}
		for _, r := range roles {
			if s, ok := r.(string); ok {
				u.Roles = append(u.Roles, s)
			}
		}
	}
	// Everything else (non-reserved keys) goes into Data
	reserved := map[string]bool{"id": true, "email": true, "roles": true, "email_verified": true, "created_at": true}
	if u.Data == nil {
		u.Data = models.JSONMap{}
	}
	for k, v := range doc {
		if !reserved[k] {
			u.Data[k] = v
		}
	}
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
