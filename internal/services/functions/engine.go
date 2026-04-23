package functions

// engine.go — multi-file functions engine
//
// Each .js file in ./functions/<projectID>/ is a function.
// Every file gets its own `app` object; registrations from all files are merged.
//
// Load phase  : scan all .js files, execute each with a registration `app`.
// Dispatch    : HTTP, cron, and hook events look up matching handlers and run them.
// Hot reload  : on each dispatch, compare per-file mtimes; reload any changed file.
// require()   : files can require('./other') to import siblings (no npm, local only).

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/patrick/cocobase/internal/models"
)

// ── Per-file registration ─────────────────────────────────────────────────────

type httpRoute struct {
	method   string // GET POST PUT PATCH DELETE ANY
	path     string // e.g. /hello
	handler  goja.Callable
	rt       *goja.Runtime
	fileName string
}

type cronJob struct {
	schedule string
	handler  goja.Callable
	rt       *goja.Runtime
	fileName string
}

type hookHandler struct {
	event      string // beforeCreate … start
	collection string // "" = all / app hook
	handler    goja.Callable
	rt         *goja.Runtime
	fileName   string
}

// fileReg holds the registrations extracted from one .js file.
type fileReg struct {
	routes []httpRoute
	crons  []cronJob
	hooks  []hookHandler
	rt     *goja.Runtime
	mtime  time.Time
}

// ── Per-project registry ──────────────────────────────────────────────────────

// projectRegistry is the merged view across all files.
type projectRegistry struct {
	mu    sync.Mutex
	files map[string]*fileReg // filename → fileReg
}

var (
	regMu      sync.RWMutex
	registries = map[string]*projectRegistry{} // projectID → registry
)

func getProjectReg(projectID string) *projectRegistry {
	regMu.RLock()
	pr, ok := registries[projectID]
	regMu.RUnlock()
	if ok {
		return pr
	}
	regMu.Lock()
	defer regMu.Unlock()
	if pr, ok = registries[projectID]; ok {
		return pr
	}
	pr = &projectRegistry{files: map[string]*fileReg{}}
	registries[projectID] = pr
	return pr
}

// InvalidateRegistry drops the cached registry for a project.
func InvalidateRegistry(projectID string) {
	regMu.Lock()
	delete(registries, projectID)
	regMu.Unlock()
	InvalidatePool(projectID)
}

// ── Load one file ─────────────────────────────────────────────────────────────

// loadFile executes a single .js function file in registration mode.
func loadFile(projectID, projectName, filePath string) (*fileReg, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	rt := goja.New()
	rt.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	reg := &fileReg{
		rt:    rt,
		mtime: info.ModTime(),
	}

	fileName := filepath.Base(filePath)
	dir := filepath.Dir(filePath)

	// ── require(path) — local-only, no npm ───────────────────────────────────
	_ = rt.Set("require", func(call goja.FunctionCall) goja.Value {
		reqPath := call.Argument(0).String()

		// Only allow relative paths
		if !strings.HasPrefix(reqPath, "./") && !strings.HasPrefix(reqPath, "../") {
			panic(rt.ToValue(fmt.Sprintf("require: only relative paths allowed (got %q)", reqPath)))
		}

		// Resolve and read
		target := filepath.Join(dir, reqPath)
		if !strings.HasSuffix(target, ".js") {
			target += ".js"
		}
		src, err := os.ReadFile(target)
		if err != nil {
			panic(rt.ToValue(fmt.Sprintf("require: cannot read %q: %v", target, err)))
		}

		// Execute in a module wrapper, return exports
		wrapped := fmt.Sprintf(`(function(module, exports){ %s; return module.exports; })`, src)
		prog, err := goja.Compile(filepath.Base(target), wrapped, false)
		if err != nil {
			panic(rt.ToValue(fmt.Sprintf("require: compile error in %q: %v", target, err)))
		}
		fn, err := rt.RunProgram(prog)
		if err != nil {
			panic(rt.ToValue(fmt.Sprintf("require: runtime error in %q: %v", target, err)))
		}
		callable, ok := goja.AssertFunction(fn)
		if !ok {
			return goja.Undefined()
		}
		modObj := rt.NewObject()
		exportsObj := rt.NewObject()
		_ = modObj.Set("exports", exportsObj)
		result, err := callable(goja.Undefined(), modObj, exportsObj)
		if err != nil {
			panic(rt.ToValue(fmt.Sprintf("require: exec error in %q: %v", target, err)))
		}
		return result
	})

	// ── app registration object ───────────────────────────────────────────────
	appObj := rt.NewObject()

	for _, method := range []string{"get", "post", "put", "patch", "delete", "all"} {
		m := strings.ToUpper(method)
		_ = appObj.Set(method, rt.ToValue(func(m string) func(goja.FunctionCall) goja.Value {
			return func(call goja.FunctionCall) goja.Value {
				path := call.Argument(0).String()
				handler, ok := goja.AssertFunction(call.Argument(1))
				if !ok {
					return goja.Undefined()
				}
				reg.routes = append(reg.routes, httpRoute{
					method: m, path: path, handler: handler,
					rt: rt, fileName: fileName,
				})
				return goja.Undefined()
			}
		}(m)))
	}

	_ = appObj.Set("cron", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		schedule := call.Argument(0).String()
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			return goja.Undefined()
		}
		reg.crons = append(reg.crons, cronJob{
			schedule: schedule, handler: handler,
			rt: rt, fileName: fileName,
		})
		return goja.Undefined()
	}))

	_ = appObj.Set("on", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		event := call.Argument(0).String()
		switch len(call.Arguments) {
		case 2:
			handler, ok := goja.AssertFunction(call.Argument(1))
			if !ok {
				return goja.Undefined()
			}
			reg.hooks = append(reg.hooks, hookHandler{
				event: event, collection: "", handler: handler,
				rt: rt, fileName: fileName,
			})
		case 3:
			collection := call.Argument(1).String()
			handler, ok := goja.AssertFunction(call.Argument(2))
			if !ok {
				return goja.Undefined()
			}
			reg.hooks = append(reg.hooks, hookHandler{
				event: event, collection: collection, handler: handler,
				rt: rt, fileName: fileName,
			})
		}
		return goja.Undefined()
	}))

	_ = rt.Set("app", appObj)

	// ── console object — available during registration and dispatch ───────────
	consoleObj := rt.NewObject()
	makeConsoleFn := func(level string) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			parts := make([]string, len(call.Arguments))
			for i, a := range call.Arguments {
				parts[i] = fmt.Sprintf("%v", a.Export())
			}
			log.Printf("[fn:%s/%s] %s: %s", safePrefix(projectID), fileName, level, strings.Join(parts, " "))
			return goja.Undefined()
		}
	}
	_ = consoleObj.Set("log", rt.ToValue(makeConsoleFn("log")))
	_ = consoleObj.Set("warn", rt.ToValue(makeConsoleFn("warn")))
	_ = consoleObj.Set("error", rt.ToValue(makeConsoleFn("error")))
	_ = consoleObj.Set("info", rt.ToValue(makeConsoleFn("info")))
	_ = consoleObj.Set("debug", rt.ToValue(makeConsoleFn("debug")))
	_ = rt.Set("console", consoleObj)

	// ── ctx stub — provides no-op versions of all ctx methods so top-level
	// calls in the file body don't crash the registration phase.
	// Real ctx (with live DB/auth/etc.) is injected per-dispatch in buildCtxForRuntime.
	ctxStub := rt.NewObject()
	noop := rt.ToValue(func(call goja.FunctionCall) goja.Value { return goja.Undefined() })
	noopObj := rt.NewObject()
	for _, method := range []string{"query", "findOne", "get", "create", "update", "delete", "queryUsers", "findUser"} {
		_ = noopObj.Set(method, noop)
	}
	authStub := rt.NewObject()
	for _, method := range []string{"createUser", "updateUser", "deleteUser", "getUser", "findUser", "queryUsers"} {
		_ = authStub.Set(method, noop)
	}
	kvStub := rt.NewObject()
	_ = kvStub.Set("set", noop)
	_ = kvStub.Set("get", rt.ToValue(func(call goja.FunctionCall) goja.Value { return goja.Null() }))
	_ = kvStub.Set("delete", noop)
	queueStub := rt.NewObject()
	_ = queueStub.Set("add", noop)
	_ = queueStub.Set("call", noop)
	reqStub := rt.NewObject()
	_ = reqStub.Set("method", "")
	_ = reqStub.Set("path", "")
	_ = reqStub.Set("headers", rt.NewObject())
	_ = reqStub.Set("body", "")
	_ = reqStub.Set("query", rt.NewObject())
	_ = reqStub.Set("json", rt.ToValue(func(call goja.FunctionCall) goja.Value { return goja.Null() }))
	projStub := rt.NewObject()
	_ = projStub.Set("id", projectID)
	_ = projStub.Set("name", projectName)
	_ = ctxStub.Set("req", reqStub)
	_ = ctxStub.Set("user", goja.Null())
	_ = ctxStub.Set("doc", goja.Null())
	_ = ctxStub.Set("project", projStub)
	_ = ctxStub.Set("db", noopObj)
	_ = ctxStub.Set("auth", authStub)
	_ = ctxStub.Set("kv", kvStub)
	_ = ctxStub.Set("queue", queueStub)
	_ = ctxStub.Set("env", rt.NewObject())
	_ = ctxStub.Set("respond", noop)
	_ = ctxStub.Set("next", noop)
	_ = ctxStub.Set("cancel", noop)
	_ = ctxStub.Set("fetch", noop)
	_ = ctxStub.Set("publish", noop)
	_ = ctxStub.Set("mail", noop)
	_ = ctxStub.Set("log", rt.ToValue(makeConsoleFn("log")))
	_ = rt.Set("ctx", ctxStub)

	if _, err := rt.RunString(string(data)); err != nil {
		return nil, fmt.Errorf("%s: %w", fileName, err)
	}

	log.Printf("[fn:%s] loaded %s — %d routes %d crons %d hooks",
		safePrefix(projectID), fileName, len(reg.routes), len(reg.crons), len(reg.hooks))
	return reg, nil
}

// ── Sync project registry with disk ──────────────────────────────────────────

// syncRegistry checks all .js files in the project dir, reloads any that
// are new or have changed mtime, and removes any that were deleted.
func syncRegistry(projectID, projectName string) *projectRegistry {
	pr := getProjectReg(projectID)
	pr.mu.Lock()
	defer pr.mu.Unlock()

	dir := ProjectFunctionsDir(projectID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return pr
	}

	seen := map[string]bool{}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		// Skip type-declaration helpers
		if e.Name() == "cocobase.d.ts" {
			continue
		}

		fullPath := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		seen[e.Name()] = true

		existing, ok := pr.files[e.Name()]
		if ok && existing.mtime.Equal(info.ModTime()) {
			continue // unchanged
		}

		// Load / reload
		fr, err := loadFile(projectID, projectName, fullPath)
		if err != nil {
			log.Printf("[fn:%s] error loading %s: %v", safePrefix(projectID), e.Name(), err)
			// Keep stale on error
			continue
		}
		pr.files[e.Name()] = fr
	}

	// Remove deleted files
	for name := range pr.files {
		if !seen[name] {
			delete(pr.files, name)
		}
	}

	return pr
}

// ── Aggregated views ──────────────────────────────────────────────────────────

func allRoutes(pr *projectRegistry) []httpRoute {
	var out []httpRoute
	for _, fr := range pr.files {
		out = append(out, fr.routes...)
	}
	return out
}

func allCrons(pr *projectRegistry) []cronJob {
	var out []cronJob
	for _, fr := range pr.files {
		out = append(out, fr.crons...)
	}
	return out
}

func allHooks(pr *projectRegistry) []hookHandler {
	var out []hookHandler
	for _, fr := range pr.files {
		out = append(out, fr.hooks...)
	}
	return out
}

// ── HTTP dispatch ─────────────────────────────────────────────────────────────

// DispatchHTTP finds and runs the matching HTTP route across all function files.
// Returns (responded, error). responded=false + err=nil means no route matched.
// After the handler returns, any queue.add tasks registered during the handler
// are drained synchronously on the same goroutine (goja is not goroutine-safe).
func DispatchHTTP(projectID, projectName string, rctx *RunContext) (bool, error) {
	pr := syncRegistry(projectID, projectName)

	method := strings.ToUpper(rctx.ReqMethod)
	path := rctx.ReqPath

	for _, route := range allRoutes(pr) {
		if !matchMethod(route.method, method) {
			continue
		}
		if !matchPath(route.path, path) {
			continue
		}
		ctx := buildCtxForRuntime(route.rt, projectID, projectName, rctx)
		if _, err := route.handler(goja.Undefined(), ctx); err != nil {
			return false, fmt.Errorf("[%s] %s %s: %w", route.fileName, route.method, route.path, err)
		}
		// Drain queue.add inline tasks on the same goroutine.
		drainInlineTasks(rctx)
		return rctx.Responded, nil
	}
	return false, nil
}

// drainInlineTasks runs all queue.add tasks accumulated in rctx.
// Must be called on the same goroutine as the JS handler (goja single-threaded).
func drainInlineTasks(rctx *RunContext) {
	tasks := rctx.inlineTasks
	rctx.inlineTasks = nil
	for _, t := range tasks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[queue:%s] inline task panic: %v", rctx.ProjectID[:8], r)
				}
			}()
			if _, err := t.handler(goja.Undefined(), t.ctx, t.rt.ToValue(t.data)); err != nil {
				log.Printf("[queue:%s] inline task error: %v", rctx.ProjectID[:8], err)
			}
			// Drain any nested queue.add calls too
			nested := t.rctx.inlineTasks
			t.rctx.inlineTasks = nil
			for _, nt := range nested {
				func() {
					defer func() { recover() }()
					nt.handler(goja.Undefined(), nt.ctx, nt.rt.ToValue(nt.data)) //nolint
				}()
			}
		}()
	}
}

// ── Hook dispatch ─────────────────────────────────────────────────────────────

// dispatchHookInner fires all matching hook handlers synchronously.
func dispatchHookInner(projectID, projectName, event, collection string, rctx *RunContext) error {
	pr := syncRegistry(projectID, projectName)

	for _, h := range allHooks(pr) {
		if h.event != event {
			continue
		}
		if h.collection != "" && h.collection != collection {
			continue
		}
		ctx := buildCtxForRuntime(h.rt, projectID, projectName, rctx)
		if _, err := h.handler(goja.Undefined(), ctx); err != nil {
			return fmt.Errorf("[%s] hook %s/%s: %w", h.fileName, event, collection, err)
		}
		if rctx.Cancelled {
			return nil
		}
	}
	return nil
}

// DispatchAppHook fires app-level hooks (e.g. "start") across all files.
func DispatchAppHook(projectID, projectName, event string) {
	pr := syncRegistry(projectID, projectName)
	rctx := &RunContext{ProjectID: projectID, ProjectName: projectName}

	for _, h := range allHooks(pr) {
		if h.event == event && h.collection == "" {
			ctx := buildCtxForRuntime(h.rt, projectID, projectName, rctx)
			if _, err := h.handler(goja.Undefined(), ctx); err != nil {
				log.Printf("[fn:%s] app hook %s [%s]: %v", safePrefix(projectID), event, h.fileName, err)
			}
		}
	}
}

// ── Cron wiring ───────────────────────────────────────────────────────────────

// GetCronJobs returns all cron registrations across all files for a project.
func GetCronJobs(projectID, projectName string) []cronJob {
	pr := syncRegistry(projectID, projectName)
	return allCrons(pr)
}

// RunCronJob runs a cron handler by its global index (across all files).
func RunCronJob(projectID, projectName string, idx int) {
	pr := syncRegistry(projectID, projectName)
	jobs := allCrons(pr)
	if idx >= len(jobs) {
		return
	}
	job := jobs[idx]
	rctx := &RunContext{ProjectID: projectID, ProjectName: projectName, ReqMethod: "CRON"}
	ctx := buildCtxForRuntime(job.rt, projectID, projectName, rctx)
	if _, err := job.handler(goja.Undefined(), ctx); err != nil {
		log.Printf("[fn:%s] cron[%d] [%s]: %v", safePrefix(projectID), idx, job.fileName, err)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// safePrefix returns up to the first 8 chars of s (avoids panic on short IDs).
func safePrefix(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// ── Route matching ────────────────────────────────────────────────────────────

func matchMethod(routeMethod, reqMethod string) bool {
	return routeMethod == "ANY" || routeMethod == reqMethod
}

func matchPath(routePath, reqPath string) bool {
	a := strings.TrimRight(routePath, "/")
	b := strings.TrimRight(reqPath, "/")
	if a == "" {
		a = "/"
	}
	if b == "" {
		b = "/"
	}
	return a == b
}

// ── JS context builder ────────────────────────────────────────────────────────

func buildCtxForRuntime(rt *goja.Runtime, projectID, projectName string, rctx *RunContext) goja.Value {
	stub := &models.Function{
		ID:        projectID,
		ProjectID: projectID,
		Name:      "fn",
	}
	rctx.ProjectName = projectName
	return buildJSContext(rt, stub, rctx)
}
