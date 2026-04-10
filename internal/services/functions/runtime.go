package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
)

// httpFetchClient is shared across all runtimes with SSRF protection.
var httpFetchClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, _ := net.SplitHostPort(addr)
			if isPrivateHost(host) {
				return nil, fmt.Errorf("fetch: blocked private/loopback address %s", host)
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	},
}

func isPrivateHost(host string) bool {
	// Block SSRF targets: loopback, link-local, private ranges
	blocked := []string{"localhost", "::1", "0.0.0.0"}
	h := strings.ToLower(host)
	for _, b := range blocked {
		if h == b {
			return true
		}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	private := []string{
		"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12",
		"192.168.0.0/16", "169.254.0.0/16", "::1/128", "fc00::/7",
	}
	for _, cidr := range private {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// RunContext is passed to the JS sandbox for each invocation.
type RunContext struct {
	// HTTP request info (set for http-triggered functions)
	ReqMethod  string
	ReqPath    string
	ReqHeaders map[string]string
	ReqBody    string
	ReqQuery   map[string]string

	// Auth user (may be nil)
	User *models.AppUser

	// Document data (set for hook-triggered functions)
	Doc map[string]interface{}

	// Project
	ProjectID   string
	ProjectName string

	// Pub/sub broadcaster (injected by caller)
	Broadcast func(channel string, data interface{})

	// HTTP response output (for http-triggered functions)
	ResponseStatus  int
	ResponseBody    string
	ResponseHeaders map[string]string
	Responded       bool

	// Hook control
	Cancelled      bool
	CancelMessage  string
	Proceeded      bool

	// Log output collected during run
	LogOutput strings.Builder
}

// VM wraps a Goja runtime with its compiled program cache.
type VM struct {
	rt      *goja.Runtime
	progMu  sync.Mutex
	progs   map[string]*goja.Program // cached compiled programs keyed by function ID
}

func newVM() *VM {
	rt := goja.New()
	rt.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	return &VM{rt: rt, progs: make(map[string]*goja.Program)}
}

// vmPool holds a pool of VMs per project.
type vmPool struct {
	mu      sync.Mutex
	pool    []*VM
	maxSize int
}

var (
	poolsMu sync.RWMutex
	pools   = map[string]*vmPool{} // keyed by projectID
)

func getPool(projectID string) *vmPool {
	poolsMu.RLock()
	p, ok := pools[projectID]
	poolsMu.RUnlock()
	if ok {
		return p
	}
	poolsMu.Lock()
	defer poolsMu.Unlock()
	if p, ok = pools[projectID]; ok {
		return p
	}
	p = &vmPool{maxSize: 3}
	pools[projectID] = p
	return p
}

// InvalidatePool drops all cached VMs and compiled programs for a project.
// Call this whenever a function is created, updated, or deleted.
func InvalidatePool(projectID string) {
	poolsMu.Lock()
	delete(pools, projectID)
	poolsMu.Unlock()
}

func (p *vmPool) acquire() *VM {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pool) == 0 {
		return newVM()
	}
	vm := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
	return vm
}

func (p *vmPool) release(vm *VM) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pool) < p.maxSize {
		p.pool = append(p.pool, vm)
	}
}

// RunFunction executes fn's JS code with the given RunContext.
// Returns the (possibly mutated) RunContext and any execution error.
func RunFunction(fn *models.Function, rctx *RunContext, timeout time.Duration) error {
	pool := getPool(fn.ProjectID)
	vm := pool.acquire()
	defer pool.release(vm)

	// Compile (cached per function ID+UpdatedAt to bust on edits)
	cacheKey := fn.ID + "|" + fn.UpdatedAt.String()
	vm.progMu.Lock()
	prog, ok := vm.progs[cacheKey]
	if !ok {
		// Wrap user code so they can export a function or just write top-level code
		wrapped := fmt.Sprintf(`(function(ctx){
%s
})`, fn.Code)
		var err error
		prog, err = goja.Compile(fn.Name, wrapped, false)
		if err != nil {
			vm.progMu.Unlock()
			return fmt.Errorf("compile error: %w", err)
		}
		// Clear old cached programs to avoid unbounded growth
		vm.progs = map[string]*goja.Program{cacheKey: prog}
	}
	vm.progMu.Unlock()

	// Timeout via interrupt
	done := make(chan struct{})
	timer := time.AfterFunc(timeout, func() {
		vm.rt.Interrupt("execution timed out")
	})
	defer func() {
		timer.Stop()
		close(done)
	}()

	// Build the ctx object exposed to JS
	ctx := buildJSContext(vm.rt, fn, rctx)

	// Run
	val, err := vm.rt.RunProgram(prog)
	if err != nil {
		return fmt.Errorf("runtime error: %w", err)
	}

	// If the exported value is callable, call it with ctx
	if callable, ok := goja.AssertFunction(val); ok {
		_, err = callable(goja.Undefined(), ctx)
		if err != nil {
			return fmt.Errorf("runtime error: %w", err)
		}
	}

	return nil
}

// buildJSContext constructs the `ctx` object visible inside user JS code.
func buildJSContext(rt *goja.Runtime, fn *models.Function, rctx *RunContext) goja.Value {
	kv := GetProjectKV(fn.ProjectID)
	obj := rt.NewObject()

	// ── ctx.req ──────────────────────────────────────────────────────────────
	req := rt.NewObject()
	_ = req.Set("method", rctx.ReqMethod)
	_ = req.Set("path", rctx.ReqPath)
	_ = req.Set("headers", rctx.ReqHeaders)
	_ = req.Set("body", rctx.ReqBody)
	_ = req.Set("query", rctx.ReqQuery)
	// Convenience: auto-parse JSON body
	_ = req.Set("json", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		var v interface{}
		if err := json.Unmarshal([]byte(rctx.ReqBody), &v); err != nil {
			return goja.Null()
		}
		return rt.ToValue(v)
	}))
	_ = obj.Set("req", req)

	// ── ctx.user ─────────────────────────────────────────────────────────────
	if rctx.User != nil {
		u := rt.NewObject()
		_ = u.Set("id", rctx.User.ID)
		_ = u.Set("email", rctx.User.Email)
		_ = u.Set("roles", rctx.User.Roles)
		_ = u.Set("verified", rctx.User.EmailVerified)
		_ = obj.Set("user", u)
	} else {
		_ = obj.Set("user", goja.Null())
	}

	// ── ctx.doc ──────────────────────────────────────────────────────────────
	if rctx.Doc != nil {
		_ = obj.Set("doc", rt.ToValue(rctx.Doc))
	} else {
		_ = obj.Set("doc", goja.Null())
	}

	// ── ctx.project ──────────────────────────────────────────────────────────
	proj := rt.NewObject()
	_ = proj.Set("id", rctx.ProjectID)
	_ = proj.Set("name", rctx.ProjectName)
	_ = obj.Set("project", proj)

	// ── ctx.respond(status, body, headers?) ──────────────────────────────────
	_ = obj.Set("respond", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		status := int(call.Argument(0).ToInteger())
		body := call.Argument(1).String()
		rctx.ResponseStatus = status
		rctx.ResponseBody = body
		rctx.Responded = true
		if len(call.Arguments) > 2 {
			hdrs := call.Argument(2).Export()
			if m, ok := hdrs.(map[string]interface{}); ok {
				rctx.ResponseHeaders = make(map[string]string)
				for k, v := range m {
					rctx.ResponseHeaders[k] = fmt.Sprintf("%v", v)
				}
			}
		}
		return goja.Undefined()
	}))

	// ── ctx.next() — hook: proceed with original operation ───────────────────
	_ = obj.Set("next", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		rctx.Proceeded = true
		return goja.Undefined()
	}))

	// ── ctx.cancel(msg?) — hook: abort operation ──────────────────────────────
	_ = obj.Set("cancel", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		rctx.Cancelled = true
		if len(call.Arguments) > 0 {
			rctx.CancelMessage = call.Argument(0).String()
		} else {
			rctx.CancelMessage = "cancelled by function"
		}
		return goja.Undefined()
	}))

	// ── ctx.fetch(url, options?) ─────────────────────────────────────────────
	_ = obj.Set("fetch", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		rawURL := call.Argument(0).String()

		method := "GET"
		var bodyReader io.Reader
		reqHeaders := map[string]string{}

		if len(call.Arguments) > 1 {
			opts := call.Argument(1).Export()
			if m, ok := opts.(map[string]interface{}); ok {
				if v, ok := m["method"].(string); ok {
					method = strings.ToUpper(v)
				}
				if v, ok := m["body"].(string); ok {
					bodyReader = strings.NewReader(v)
				}
				if v, ok := m["headers"].(map[string]interface{}); ok {
					for k, hv := range v {
						reqHeaders[k] = fmt.Sprintf("%v", hv)
					}
				}
			}
		}

		// Validate URL / SSRF guard
		parsed, err := url.Parse(rawURL)
		if err != nil || isPrivateHost(parsed.Hostname()) {
			panic(rt.ToValue(fmt.Sprintf("fetch: blocked URL %s", rawURL)))
		}

		httpReq, err := http.NewRequest(method, rawURL, bodyReader)
		if err != nil {
			panic(rt.ToValue(err.Error()))
		}
		for k, v := range reqHeaders {
			httpReq.Header.Set(k, v)
		}

		resp, err := httpFetchClient.Do(httpReq)
		if err != nil {
			panic(rt.ToValue(err.Error()))
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		result := rt.NewObject()
		_ = result.Set("status", resp.StatusCode)
		_ = result.Set("body", string(respBody))
		_ = result.Set("json", rt.ToValue(func(call goja.FunctionCall) goja.Value {
			var v interface{}
			if err := json.Unmarshal(respBody, &v); err != nil {
				return goja.Null()
			}
			return rt.ToValue(v)
		}))
		return result
	}))

	// ── ctx.publish(channel, data) ───────────────────────────────────────────
	_ = obj.Set("publish", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		if rctx.Broadcast == nil {
			return goja.Undefined()
		}
		channel := call.Argument(0).String()
		data := call.Argument(1).Export()
		// Scope channel to project
		rctx.Broadcast(fn.ProjectID+":"+channel, data)
		return goja.Undefined()
	}))

	// ── ctx.db — project-scoped database access ───────────────────────────────
	db := rt.NewObject()

	_ = db.Set("list", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		colName := call.Argument(0).String()
		col, err := findCollection(fn.ProjectID, colName)
		if err != nil {
			panic(rt.ToValue(err.Error()))
		}
		var docs []models.Document
		database.DB.Where("collection_id = ?", col.ID).
			Order("created_at desc").Limit(500).Find(&docs)
		result := make([]interface{}, len(docs))
		for i, d := range docs {
			result[i] = docToMap(&d)
		}
		return rt.ToValue(result)
	}))

	_ = db.Set("get", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		colName := call.Argument(0).String()
		docID := call.Argument(1).String()
		col, err := findCollection(fn.ProjectID, colName)
		if err != nil {
			panic(rt.ToValue(err.Error()))
		}
		var doc models.Document
		if err := database.DB.Where("id = ? AND collection_id = ?", docID, col.ID).First(&doc).Error; err != nil {
			return goja.Null()
		}
		return rt.ToValue(docToMap(&doc))
	}))

	_ = db.Set("create", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		colName := call.Argument(0).String()
		data := call.Argument(1).Export()
		col, err := findCollection(fn.ProjectID, colName)
		if err != nil {
			panic(rt.ToValue(err.Error()))
		}
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			panic(rt.ToValue("db.create: data must be an object"))
		}
		doc := models.Document{CollectionID: col.ID, Data: dataMap}
		if err := database.DB.Create(&doc).Error; err != nil {
			panic(rt.ToValue(err.Error()))
		}
		return rt.ToValue(docToMap(&doc))
	}))

	_ = db.Set("update", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		colName := call.Argument(0).String()
		docID := call.Argument(1).String()
		data := call.Argument(2).Export()
		col, err := findCollection(fn.ProjectID, colName)
		if err != nil {
			panic(rt.ToValue(err.Error()))
		}
		var doc models.Document
		if err := database.DB.Where("id = ? AND collection_id = ?", docID, col.ID).First(&doc).Error; err != nil {
			panic(rt.ToValue("document not found"))
		}
		if m, ok := data.(map[string]interface{}); ok {
			for k, v := range m {
				doc.Data[k] = v
			}
		}
		database.DB.Save(&doc)
		return rt.ToValue(docToMap(&doc))
	}))

	_ = db.Set("delete", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		colName := call.Argument(0).String()
		docID := call.Argument(1).String()
		col, err := findCollection(fn.ProjectID, colName)
		if err != nil {
			panic(rt.ToValue(err.Error()))
		}
		database.DB.Where("id = ? AND collection_id = ?", docID, col.ID).Delete(&models.Document{})
		return goja.Undefined()
	}))

	_ = obj.Set("db", db)

	// ── ctx.kv ───────────────────────────────────────────────────────────────
	kvObj := rt.NewObject()
	_ = kvObj.Set("set", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		val := call.Argument(1).Export()
		ttl := 0
		if len(call.Arguments) > 2 {
			ttl = int(call.Argument(2).ToInteger())
		}
		kv.Set(key, val, ttl)
		return goja.Undefined()
	}))
	_ = kvObj.Set("get", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		val, ok := kv.Get(call.Argument(0).String())
		if !ok {
			return goja.Null()
		}
		return rt.ToValue(val)
	}))
	_ = kvObj.Set("delete", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		kv.Delete(call.Argument(0).String())
		return goja.Undefined()
	}))
	_ = obj.Set("kv", kvObj)

	// ── ctx.log(...) ─────────────────────────────────────────────────────────
	_ = obj.Set("log", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			parts[i] = fmt.Sprintf("%v", a.Export())
		}
		line := strings.Join(parts, " ")
		rctx.LogOutput.WriteString(line + "\n")
		return goja.Undefined()
	}))

	// ── ctx.mail(options) — send email using the project's mailer config ─────
	// options: { to, subject, html?, text? }
	_ = obj.Set("mail", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		opts := call.Argument(0).Export()
		m, ok := opts.(map[string]interface{})
		if !ok {
			panic(rt.ToValue("mail: argument must be an object {to, subject, html?, text?}"))
		}
		toString := func(key string) string {
			if v, ok := m[key]; ok {
				return fmt.Sprintf("%v", v)
			}
			return ""
		}
		msg := services.EmailMessage{
			To:      toString("to"),
			Subject: toString("subject"),
			HTML:    toString("html"),
			Text:    toString("text"),
		}
		if msg.To == "" || msg.Subject == "" {
			panic(rt.ToValue("mail: 'to' and 'subject' are required"))
		}
		// Load project for mailer config
		var project models.Project
		if err := database.DB.First(&project, "id = ?", fn.ProjectID).Error; err != nil {
			panic(rt.ToValue("mail: could not load project"))
		}
		if err := services.SendEmailForProject(&project, msg); err != nil {
			panic(rt.ToValue("mail: " + err.Error()))
		}
		rctx.LogOutput.WriteString("[mail] sent to " + msg.To + "\n")
		return goja.Undefined()
	}))

	// ── ctx.env — placeholder; project env vars could be stored in DB later ──
	_ = obj.Set("env", rt.NewObject())

	return obj
}

// ── helpers ───────────────────────────────────────────────────────────────────

func findCollection(projectID, nameOrID string) (*models.Collection, error) {
	var col models.Collection
	err := database.DB.
		Where("project_id = ? AND (id = ? OR name = ?)", projectID, nameOrID, nameOrID).
		First(&col).Error
	if err != nil {
		return nil, fmt.Errorf("collection %q not found", nameOrID)
	}
	return &col, nil
}

func docToMap(d *models.Document) map[string]interface{} {
	m := map[string]interface{}{
		"id":            d.ID,
		"collection_id": d.CollectionID,
		"created_at":    d.CreatedAt,
	}
	for k, v := range d.Data {
		m[k] = v
	}
	return m
}

