import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { functionsApi } from "@/api/client";
import type { CloudFunction, CloudFunctionCreate, TriggerType, TriggerConfig, HttpRoute } from "@/api/client";

const HOOK_EVENTS = [
  "beforeCreate", "afterCreate",
  "beforeUpdate", "afterUpdate",
  "beforeDelete", "afterDelete",
];

const STARTER_CODE: Record<TriggerType, string> = {
  http: `// HTTP function — called when someone hits /fn<your-path>
// ctx.req: { method, path, headers, body, query, json() }
// ctx.user: authenticated user or null
ctx.respond(200, JSON.stringify({ message: "Hello from Cocobase!" }), {
  "Content-Type": "application/json"
});`,
  hook: `// Hook function — runs on collection lifecycle events
// ctx.doc: the document being created/updated/deleted
// ctx.user: the user performing the action (or null)
// ctx.next()   — allow the operation to proceed
// ctx.cancel("reason") — block the operation

ctx.log("doc:", JSON.stringify(ctx.doc));
ctx.next();`,
  cron: `// Cron function — runs on a schedule
// ctx.db, ctx.kv, ctx.mail, ctx.fetch are all available

ctx.log("Cron fired at", new Date().toISOString());`,
};

interface Props { projectId: string; }

export function FunctionsTab({ projectId }: Props) {
  const qc = useQueryClient();
  const [selected, setSelected] = useState<CloudFunction | null>(null);
  const [creating, setCreating] = useState(false);
  const [activeView, setActiveView] = useState<"functions" | "routes">("functions");
  const [runResult, setRunResult] = useState<{ output: string; error?: string; duration_ms: number } | null>(null);
  const [running, setRunning] = useState(false);

  const { data } = useQuery({
    queryKey: ["functions", projectId],
    queryFn: () => functionsApi.list(projectId).then(r => r.data),
  });

  const { data: routesData } = useQuery({
    queryKey: ["fn-routes", projectId],
    queryFn: () => functionsApi.listRoutes(projectId).then(r => r.data),
    enabled: activeView === "routes",
  });

  const deleteMutation = useMutation({
    mutationFn: (fnId: string) => functionsApi.delete(projectId, fnId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["functions", projectId] });
      setSelected(null);
    },
  });

  const fns = data?.data ?? [];
  const routes: HttpRoute[] = routesData?.data ?? [];

  return (
    <div className="space-y-4">
      {/* View switcher */}
      <div className="flex items-center justify-between">
        <div className="flex gap-1 rounded-lg border p-1 bg-muted">
          {(["functions", "routes"] as const).map(v => (
            <button key={v} onClick={() => setActiveView(v)}
              className={`px-3 py-1 rounded-md text-sm font-medium capitalize transition-colors ${activeView === v ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"}`}>
              {v === "routes" ? "HTTP Routes" : "Functions"}
            </button>
          ))}
        </div>
        {activeView === "functions" && (
          <button onClick={() => { setCreating(true); setSelected(null); setRunResult(null); }}
            className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90">
            + New function
          </button>
        )}
      </div>

      {/* Routes view */}
      {activeView === "routes" && (
        <div className="rounded-lg border overflow-hidden">
          {routes.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground text-center">No HTTP functions yet. Create a function with trigger type "http".</p>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-muted">
                <tr>
                  <th className="px-4 py-2 text-left font-medium">Method</th>
                  <th className="px-4 py-2 text-left font-medium">Path</th>
                  <th className="px-4 py-2 text-left font-medium">Name</th>
                  <th className="px-4 py-2 text-left font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {routes.map(r => (
                  <tr key={r.function_id} className="border-t">
                    <td className="px-4 py-2 font-mono text-xs font-bold text-blue-600">{r.method}</td>
                    <td className="px-4 py-2 font-mono text-xs">{r.path}</td>
                    <td className="px-4 py-2 text-xs">{r.name}</td>
                    <td className="px-4 py-2">
                      <span className={`text-xs rounded-full px-2 py-0.5 ${r.enabled ? "bg-green-100 text-green-700" : "bg-muted text-muted-foreground"}`}>
                        {r.enabled ? "active" : "disabled"}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Functions list + editor */}
      {activeView === "functions" && (
        <div className="grid grid-cols-3 gap-4" style={{ minHeight: 400 }}>
          {/* Left: list */}
          <div className="col-span-1 rounded-lg border overflow-hidden">
            {fns.length === 0 && !creating ? (
              <p className="p-4 text-sm text-muted-foreground">No functions yet.</p>
            ) : (
              <ul>
                {fns.map(f => (
                  <li key={f.id}>
                    <button onClick={() => { setSelected(f); setCreating(false); setRunResult(null); }}
                      className={`w-full text-left px-3 py-2.5 border-b hover:bg-accent transition-colors ${selected?.id === f.id ? "bg-accent" : ""}`}>
                      <div className="flex items-center justify-between gap-1">
                        <span className="text-sm font-medium truncate">{f.name}</span>
                        <TriggerBadge type={f.trigger_type} />
                      </div>
                      <div className="text-xs text-muted-foreground mt-0.5 truncate">
                        {triggerSummary(f)}
                      </div>
                      {f.last_error && (
                        <div className="text-xs text-destructive mt-0.5 truncate">⚠ {f.last_error}</div>
                      )}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* Right: editor */}
          <div className="col-span-2 rounded-lg border p-4 space-y-4">
            {creating && (
              <CreateForm projectId={projectId} onDone={() => { setCreating(false); qc.invalidateQueries({ queryKey: ["functions", projectId] }); }} onCancel={() => setCreating(false)} />
            )}
            {!creating && selected && (
              <EditForm
                key={selected.id}
                fn={selected}
                projectId={projectId}
                runResult={runResult}
                running={running}
                onSaved={(updated) => { setSelected(updated); qc.invalidateQueries({ queryKey: ["functions", projectId] }); }}
                onDelete={() => deleteMutation.mutate(selected.id)}
                onRun={async () => {
                  setRunning(true); setRunResult(null);
                  try {
                    const r = await functionsApi.run(projectId, selected.id);
                    setRunResult(r.data);
                    qc.invalidateQueries({ queryKey: ["functions", projectId] });
                  } finally { setRunning(false); }
                }}
              />
            )}
            {!creating && !selected && (
              <p className="text-sm text-muted-foreground">Select a function to edit, or create a new one.</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// ── Create form ───────────────────────────────────────────────────────────────

function CreateForm({ projectId, onDone, onCancel }: { projectId: string; onDone: () => void; onCancel: () => void }) {
  const [name, setName] = useState("");
  const [triggerType, setTriggerType] = useState<TriggerType>("http");
  const [config, setConfig] = useState<TriggerConfig>({ method: "ANY", path: "/hello" });
  const [code, setCode] = useState(STARTER_CODE.http);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  function handleTypeChange(t: TriggerType) {
    setTriggerType(t);
    setCode(STARTER_CODE[t]);
    if (t === "http") setConfig({ method: "ANY", path: "/hello" });
    else if (t === "hook") setConfig({ event: "beforeCreate", collection: "" });
    else setConfig({ schedule: "0 * * * *" });
  }

  async function handleCreate() {
    setError(""); setSaving(true);
    try {
      const payload: CloudFunctionCreate = { name, code, trigger_type: triggerType, trigger_config: config, enabled: true, timeout: 10 };
      await functionsApi.create(projectId, payload);
      onDone();
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } };
      setError(err?.response?.data?.message ?? "Failed to create");
    } finally { setSaving(false); }
  }

  return (
    <div className="space-y-3">
      <h3 className="font-semibold text-sm">New function</h3>
      <input value={name} onChange={e => setName(e.target.value)} placeholder="Function name"
        className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />

      <div className="flex gap-2">
        {(["http", "hook", "cron"] as TriggerType[]).map(t => (
          <button key={t} onClick={() => handleTypeChange(t)}
            className={`px-3 py-1 rounded-md text-xs font-medium border transition-colors ${triggerType === t ? "bg-primary text-primary-foreground border-primary" : "hover:bg-accent"}`}>
            {t}
          </button>
        ))}
      </div>

      <TriggerConfigEditor type={triggerType} config={config} onChange={setConfig} />

      <textarea value={code} onChange={e => setCode(e.target.value)} rows={12}
        spellCheck={false}
        className="w-full rounded-md border bg-muted px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-ring resize-y" />

      {error && <p className="text-xs text-destructive">{error}</p>}
      <div className="flex gap-2">
        <button onClick={handleCreate} disabled={saving || !name}
          className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50">
          {saving ? "Creating…" : "Create"}
        </button>
        <button onClick={onCancel} className="rounded-md border px-3 py-1.5 text-xs hover:bg-accent">Cancel</button>
      </div>
    </div>
  );
}

// ── Edit form ─────────────────────────────────────────────────────────────────

function EditForm({ fn, projectId, onSaved, onDelete, onRun, runResult, running }: {
  fn: CloudFunction;
  projectId: string;
  onSaved: (updated: CloudFunction) => void;
  onDelete: () => void;
  onRun: () => void;
  runResult: { output: string; error?: string; duration_ms: number } | null;
  running: boolean;
}) {
  const [name, setName] = useState(fn.name);
  const [code, setCode] = useState(fn.code);
  const [config, setConfig] = useState<TriggerConfig>(fn.trigger_config);
  const [enabled, setEnabled] = useState(fn.enabled);
  const [timeout, setTimeout_] = useState(fn.timeout);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);

  async function handleSave() {
    setError(""); setSaving(true);
    try {
      const r = await functionsApi.update(projectId, fn.id, { name, code, trigger_config: config, enabled, timeout });
      onSaved(r.data);
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } };
      setError(err?.response?.data?.message ?? "Failed to save");
    } finally { setSaving(false); }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <input value={name} onChange={e => setName(e.target.value)}
          className="flex-1 rounded-md border bg-background px-3 py-1.5 text-sm font-medium outline-none focus:ring-2 focus:ring-ring" />
        <TriggerBadge type={fn.trigger_type} />
        <label className="flex items-center gap-1.5 text-xs cursor-pointer">
          <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} className="rounded" />
          Enabled
        </label>
      </div>

      <TriggerConfigEditor type={fn.trigger_type} config={config} onChange={setConfig} />

      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span>Timeout:</span>
        <input type="number" value={timeout} onChange={e => setTimeout_(Number(e.target.value))} min={1} max={300}
          className="w-16 rounded border bg-background px-2 py-0.5 text-xs outline-none focus:ring-1 focus:ring-ring" />
        <span>seconds</span>
      </div>

      <textarea value={code} onChange={e => setCode(e.target.value)} rows={14}
        spellCheck={false}
        className="w-full rounded-md border bg-muted px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-ring resize-y" />

      {error && <p className="text-xs text-destructive">{error}</p>}

      <div className="flex items-center gap-2 flex-wrap">
        <button onClick={handleSave} disabled={saving}
          className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50">
          {saving ? "Saving…" : "Save"}
        </button>
        {(fn.trigger_type === "cron" || fn.trigger_type === "http") && (
          <button onClick={onRun} disabled={running}
            className="rounded-md border px-3 py-1.5 text-xs font-medium hover:bg-accent disabled:opacity-50">
            {running ? "Running…" : "▶ Run now"}
          </button>
        )}
        {!confirmDelete ? (
          <button onClick={() => setConfirmDelete(true)}
            className="ml-auto rounded-md border border-destructive px-3 py-1.5 text-xs font-medium text-destructive hover:bg-destructive/10">
            Delete
          </button>
        ) : (
          <div className="ml-auto flex gap-2 items-center">
            <span className="text-xs text-destructive">Delete this function?</span>
            <button onClick={onDelete} className="rounded-md bg-destructive px-3 py-1.5 text-xs text-white">Yes, delete</button>
            <button onClick={() => setConfirmDelete(false)} className="rounded-md border px-3 py-1.5 text-xs">Cancel</button>
          </div>
        )}
      </div>

      {/* Run result */}
      {runResult && (
        <div className={`rounded-md border p-3 space-y-1 text-xs ${runResult.error ? "border-destructive bg-destructive/5" : "border-green-500 bg-green-50 dark:bg-green-950/20"}`}>
          <div className="flex items-center justify-between">
            <span className={`font-semibold ${runResult.error ? "text-destructive" : "text-green-700 dark:text-green-400"}`}>
              {runResult.error ? "Error" : "Success"} — {runResult.duration_ms}ms
            </span>
          </div>
          {runResult.output && (
            <pre className="whitespace-pre-wrap font-mono text-[11px] text-foreground/80">{runResult.output}</pre>
          )}
          {runResult.error && (
            <pre className="whitespace-pre-wrap font-mono text-[11px] text-destructive">{runResult.error}</pre>
          )}
        </div>
      )}

      {/* Execution log */}
      {fn.logs && fn.logs.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">Recent runs</p>
          <div className="rounded-md border divide-y max-h-48 overflow-y-auto">
            {[...fn.logs].reverse().map((log, i) => (
              <div key={i} className="px-3 py-1.5 text-xs flex items-start gap-3">
                <span className={`shrink-0 ${log.success ? "text-green-600" : "text-destructive"}`}>{log.success ? "✓" : "✗"}</span>
                <span className="text-muted-foreground shrink-0">{new Date(log.run_at).toLocaleTimeString()}</span>
                <span className="text-muted-foreground shrink-0">{log.duration_ms}ms</span>
                {log.output && <span className="truncate font-mono">{log.output.trim()}</span>}
                {log.error && <span className="truncate text-destructive">{log.error}</span>}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ── Trigger config editor ─────────────────────────────────────────────────────

function TriggerConfigEditor({ type, config, onChange }: { type: TriggerType; config: TriggerConfig; onChange: (c: TriggerConfig) => void }) {
  if (type === "http") return (
    <div className="flex gap-2">
      <select value={config.method ?? "ANY"} onChange={e => onChange({ ...config, method: e.target.value })}
        className="rounded-md border bg-background px-2 py-1.5 text-xs outline-none focus:ring-2 focus:ring-ring">
        {["ANY", "GET", "POST", "PATCH", "PUT", "DELETE"].map(m => <option key={m}>{m}</option>)}
      </select>
      <input value={config.path ?? ""} onChange={e => onChange({ ...config, path: e.target.value })}
        placeholder="/your-path"
        className="flex-1 rounded-md border bg-background px-3 py-1.5 text-xs font-mono outline-none focus:ring-2 focus:ring-ring" />
    </div>
  );

  if (type === "hook") return (
    <div className="flex gap-2">
      <select value={config.event ?? "beforeCreate"} onChange={e => onChange({ ...config, event: e.target.value })}
        className="rounded-md border bg-background px-2 py-1.5 text-xs outline-none focus:ring-2 focus:ring-ring">
        {HOOK_EVENTS.map(e => <option key={e}>{e}</option>)}
      </select>
      <input value={config.collection ?? ""} onChange={e => onChange({ ...config, collection: e.target.value })}
        placeholder="collection name (blank = all)"
        className="flex-1 rounded-md border bg-background px-3 py-1.5 text-xs outline-none focus:ring-2 focus:ring-ring" />
    </div>
  );

  // cron
  return (
    <div className="space-y-1">
      <input value={config.schedule ?? ""} onChange={e => onChange({ ...config, schedule: e.target.value })}
        placeholder="cron expression e.g. 0 * * * * (every hour)"
        className="w-full rounded-md border bg-background px-3 py-1.5 text-xs font-mono outline-none focus:ring-2 focus:ring-ring" />
      <p className="text-[11px] text-muted-foreground">Format: second(optional) minute hour day month weekday — <a href="https://crontab.guru" target="_blank" rel="noreferrer" className="underline">crontab.guru</a></p>
    </div>
  );
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function TriggerBadge({ type }: { type: TriggerType }) {
  const colors: Record<TriggerType, string> = {
    http: "bg-blue-100 text-blue-700",
    hook: "bg-purple-100 text-purple-700",
    cron: "bg-orange-100 text-orange-700",
  };
  return <span className={`shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold ${colors[type]}`}>{type}</span>;
}

function triggerSummary(f: CloudFunction): string {
  const c = f.trigger_config;
  if (f.trigger_type === "http") return `${c.method ?? "ANY"} /fn${c.path ?? ""}`;
  if (f.trigger_type === "hook") return `${c.event ?? "?"}${c.collection ? ` → ${c.collection}` : " (all)"}`;
  if (f.trigger_type === "cron") return c.schedule ?? "no schedule";
  return "";
}
