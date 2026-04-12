package functions

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FunctionsBaseDir is the single functions directory for this instance.
// Single-instance mode: all .js files live directly in ./functions/.
const FunctionsBaseDir = "functions"

// ProjectFunctionsDir returns the functions directory.
// Single-instance: always ./functions/ regardless of projectID.
func ProjectFunctionsDir(_ string) string {
	return FunctionsBaseDir
}

// FunctionFilePath returns the canonical path for a named function file.
// Layout: ./functions/<name>.js
func FunctionFilePath(_, name string) string {
	return filepath.Join(FunctionsBaseDir, sanitizeName(name)+".js")
}

// ProjectFunctionsFile is kept for compatibility — returns functions.js path.
func ProjectFunctionsFile(projectID string) string {
	return FunctionFilePath(projectID, "functions")
}

// ReadProjectCode reads a named function file (or "functions" for the default).
func ReadProjectCode(projectID string) (code string, mtime time.Time) {
	return ReadFunctionFile(projectID, "functions")
}

// ReadFunctionFile reads a specific .js function file by name.
func ReadFunctionFile(projectID, name string) (code string, mtime time.Time) {
	path := FunctionFilePath(projectID, name)
	info, err := os.Stat(path)
	if err != nil {
		return "", time.Time{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}
	}
	return string(data), info.ModTime()
}

// ReadFunctionCode reads a function's JS code (legacy compat shim).
func ReadFunctionCode(projectID, name, inlineCode string) string {
	code, _ := ReadFunctionFile(projectID, name)
	if code == "" {
		return inlineCode
	}
	return code
}

// ReadFunctionCodeWithMtime reads a function file and returns code + mtime (legacy compat).
func ReadFunctionCodeWithMtime(projectID, name string, inlineCode string, fallbackMtime time.Time) (string, time.Time) {
	code, mtime := ReadFunctionFile(projectID, name)
	if code == "" {
		return inlineCode, fallbackMtime
	}
	return code, mtime
}

// WriteFunctionCode writes JS code to a named function file.
func WriteFunctionCode(projectID, name, code string) error {
	dir := ProjectFunctionsDir(projectID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create functions dir: %w", err)
	}
	path := FunctionFilePath(projectID, name)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write function file: %w", err)
	}
	return nil
}

// DeleteFunctionFile removes a function's .js file from disk.
func DeleteFunctionFile(projectID, name string) {
	path := FunctionFilePath(projectID, name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("functions: could not delete %s: %v", path, err)
	}
}

// ListFunctionFiles returns the names (without .js) of all function files in a project dir.
func ListFunctionFiles(projectID string) []string {
	dir := ProjectFunctionsDir(projectID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			names = append(names, strings.TrimSuffix(e.Name(), ".js"))
		}
	}
	return names
}

// EnsureProjectFunctionsDir creates the project dir, writes tooling files,
// and drops a hello_world.js sample if the dir is empty.
func EnsureProjectFunctionsDir(projectID, projectName string) {
	dir := ProjectFunctionsDir(projectID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("functions: could not create dir %s: %v", dir, err)
		return
	}

	WriteTypeDeclaration(dir)
	WriteVSCodeSettings(dir)

	// Write sample only if there are no .js files yet
	if len(ListFunctionFiles(projectID)) == 0 {
		WriteSampleFunction(projectID, projectName)
		log.Printf("functions: created sample hello_world.js")
	}
}

// WriteSampleFunction writes a hello_world.js starter file.
func WriteSampleFunction(projectID, _ string) {
	code := `// hello_world.js — starter HTTP function
// URL: GET /functions/func/hello_world
//
// Add more files to this folder — each .js file is a separate function.
// Use require('./utils') to share code between files.
// See FUNCTIONS.md in the project root for full docs.

app.get("/hello_world", (ctx) => {
  ctx.respond(200, JSON.stringify({
    message: "Hello from Cocobase!",
    project: ctx.project.name,
    time: new Date().toISOString(),
  }), { "Content-Type": "application/json" });
});
`

	if err := WriteFunctionCode(projectID, "hello_world", code); err != nil {
		log.Printf("functions: could not write sample: %v", err)
	}
}

// sanitizeName converts a function name to a safe filename.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		if r == ' ' {
			return '_'
		}
		return -1
	}, name)
	return name
}

// WriteTypeDeclaration writes cocobase.d.ts into the project functions directory.
func WriteTypeDeclaration(dir string) {
	dts := `// Cocobase cloud functions — type declarations
// Auto-generated. Do not edit.

interface FetchOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: string;
  headers?: Record<string, string>;
}

interface FetchResponse {
  status: number;
  body: string;
  json(): unknown;
}

interface QueryResult {
  data: Record<string, unknown>[];
  total: number;
  has_more: boolean;
}

interface DbApi {
  /**
   * Query a collection with filters, sorting and pagination.
   *
   * Filter operators (append to field name):
   *   field          exact match
   *   field_ne       not equal
   *   field_gt/gte   greater than / greater or equal
   *   field_lt/lte   less than / less or equal
   *   field_contains case-insensitive substring
   *   field_startswith / field_endswith
   *   field_in       comma-separated list  e.g. "published,draft"
   *   field_notin    not in list
   *   field_isnull   "true" or "false"
   *
   * OR logic: prefix key with [or] or [or:groupName]
   *   { "[or]status": "published", "[or]status_2": "draft" }
   *
   * @example
   *   ctx.db.query("posts", { status: "published", views_gte: "100", limit: 20, sort: "created_at", order: "desc" })
   */
  query(collection: string, options?: QueryOptions): QueryResult;

  /** Return the first matching document, or null. */
  findOne(collection: string, options?: QueryOptions): Record<string, unknown> | null;

  /** Get a single document by ID. Returns null if not found. */
  get(collection: string, id: string): Record<string, unknown> | null;

  /** Create a new document and return it. */
  create(collection: string, data: Record<string, unknown>): Record<string, unknown>;

  /** Merge fields into an existing document and return it. */
  update(collection: string, id: string, data: Record<string, unknown>): Record<string, unknown>;

  /** Delete a document by ID. */
  delete(collection: string, id: string): void;

  /** Query app users with the same filter syntax as db.query(). */
  queryUsers(options?: QueryOptions): QueryResult;

  /** Return the first matching user, or null. */
  findUser(options?: QueryOptions): Record<string, unknown> | null;
}

interface QueryOptions {
  limit?: number;
  offset?: number;
  sort?: string;
  order?: "asc" | "desc";
  /** Fields to return (not yet enforced server-side, reserved). */
  select?: string[];
  [filter: string]: unknown;
}

interface KvApi {
  set(key: string, value: unknown, ttl?: number): void;
  get(key: string): unknown | null;
  delete(key: string): void;
}

interface MailOptions {
  to: string;
  subject: string;
  html?: string;
  text?: string;
}

interface CocobaseUser {
  id: string;
  email: string;
  roles: string[];
  verified: boolean;
}

interface CocobaseRequest {
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  query: Record<string, string>;
  json(): unknown;
}

interface CocobaseProject {
  id: string;
  name: string;
}

interface AuthApi {
  /**
   * Create a new app user for this project.
   * @example ctx.auth.createUser({ email: "alice@x.com", password: "secret", data: { plan: "free" } })
   */
  createUser(options: { email: string; password: string; data?: Record<string, unknown>; roles?: string[] }): Record<string, unknown>;
  /** Get a user by ID, or null if not found. */
  getUser(id: string): Record<string, unknown> | null;
  /** Find the first user matching filters (same syntax as db.queryUsers). */
  findUser(options?: QueryOptions): Record<string, unknown> | null;
  /** Query users with filters, sorting and pagination. */
  queryUsers(options?: QueryOptions): QueryResult;
  /** Merge fields into an existing user. Pass emailVerified to verify/unverify. */
  updateUser(id: string, options: { email?: string; password?: string; data?: Record<string, unknown>; roles?: string[]; emailVerified?: boolean }): Record<string, unknown>;
  /** Delete a user by ID. */
  deleteUser(id: string): void;
}

interface QueueApi {
  /**
   * Run a function in the background after the response is sent.
   * Max 100 queued tasks. 20-second timeout per task. Not persisted across restarts.
   * @example
   *   queue.add((data) => { ctx.mail({ to: data.email, subject: "Welcome!" }); }, { email: user.email })
   */
  add(handler: (data: unknown) => void, data?: unknown): void;
  /**
   * Dispatch an app.on("queue", fileName, handler) event in the background.
   * @example queue.call("send_welcome", { userId: user.id })
   */
  call(fileName: string, data?: unknown): void;
}

interface CocobaseContext {
  req: CocobaseRequest;
  user: CocobaseUser | null;
  /** Document being operated on (hook functions). */
  doc: Record<string, unknown> | null;
  project: CocobaseProject;
  db: DbApi;
  auth: AuthApi;
  queue: QueueApi;
  kv: KvApi;
  env: Record<string, string>;
  respond(status: number, body: string, headers?: Record<string, string>): void;
  next(): void;
  cancel(message?: string): void;
  fetch(url: string, options?: FetchOptions): FetchResponse;
  publish(channel: string, data: unknown): void;
  mail(options: MailOptions): void;
  log(...args: unknown[]): void;
}

type HttpHandler = (ctx: CocobaseContext) => void;
type HookHandler = (ctx: CocobaseContext) => void;
type CronHandler = (ctx: CocobaseContext) => void;
type HookEvent =
  | "beforeCreate" | "afterCreate"
  | "beforeUpdate" | "afterUpdate"
  | "beforeDelete" | "afterDelete";
type AppEvent = "start" | "queue";

interface CocobaseApp {
  get(path: string, handler: HttpHandler): void;
  post(path: string, handler: HttpHandler): void;
  put(path: string, handler: HttpHandler): void;
  patch(path: string, handler: HttpHandler): void;
  delete(path: string, handler: HttpHandler): void;
  all(path: string, handler: HttpHandler): void;
  cron(schedule: string, handler: CronHandler): void;
  on(event: HookEvent, collection: string, handler: HookHandler): void;
  on(event: AppEvent, handler: HookHandler): void;
}

declare const app: CocobaseApp;
declare const ctx: CocobaseContext;
`
	path := filepath.Join(dir, "cocobase.d.ts")
	if err := os.WriteFile(path, []byte(dts), 0644); err != nil {
		log.Printf("functions: could not write cocobase.d.ts: %v", err)
	}
}

// WriteVSCodeSettings writes jsconfig.json + .vscode/settings.json.
func WriteVSCodeSettings(dir string) {
	jsconfig := `{
  "compilerOptions": {
    "checkJs": false,
    "target": "ES2020",
    "lib": ["ES2020"]
  },
  "include": ["*.js", "cocobase.d.ts"]
}
`
	os.WriteFile(filepath.Join(dir, "jsconfig.json"), []byte(jsconfig), 0644)

	vscodeDir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		return
	}
	settings := `{
  "editor.quickSuggestions": { "strings": true, "other": true, "comments": false }
}
`
	settingsPath := filepath.Join(vscodeDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		os.WriteFile(settingsPath, []byte(settings), 0644)
	}
}
