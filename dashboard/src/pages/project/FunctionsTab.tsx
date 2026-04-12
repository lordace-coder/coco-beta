import { useState, useEffect, useRef } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { functionsApi } from "@/api/client";
import type { CronEntry, FunctionFile, RunResult } from "@/api/client";
import { useInstance } from "@/hooks/useInstance";

export function FunctionsTab() {
  const projectId = useInstance();
  const qc = useQueryClient();
  const [selected, setSelected] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [createError, setCreateError] = useState("");

  const { data: listData, isLoading: listLoading } = useQuery({
    queryKey: ["fn-list", projectId],
    queryFn: () => functionsApi.list(projectId).then(r => r.data.data),
  });

  const { data: cronData } = useQuery({
    queryKey: ["fn-crons", projectId],
    queryFn: () => functionsApi.getCrons(projectId).then(r => r.data.data),
    refetchInterval: 30_000,
  });

  const files: FunctionFile[] = listData ?? [];
  const crons: CronEntry[] = cronData ?? [];

  // Auto-select first file
  useEffect(() => {
    if (!selected && files.length > 0) setSelected(files[0].name);
  }, [files, selected]);

  const createMutation = useMutation({
    mutationFn: (name: string) => functionsApi.create(projectId, name),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["fn-list", projectId] });
      setSelected(res.data.name);
      setCreating(false);
      setNewName("");
      setCreateError("");
    },
    onError: (e: unknown) => {
      const err = e as { response?: { data?: { message?: string } } };
      setCreateError(err?.response?.data?.message ?? "Failed to create");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => functionsApi.delete(projectId, name),
    onSuccess: (_, name) => {
      qc.invalidateQueries({ queryKey: ["fn-list", projectId] });
      if (selected === name) setSelected(null);
    },
  });

  function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const n = newName.trim().replace(/\.js$/, "").replace(/[^a-z0-9_-]/gi, "_").toLowerCase();
    if (!n) return;
    createMutation.mutate(n);
  }

  return (
    <div className="flex gap-4 min-h-[520px]">
      {/* Sidebar */}
      <div className="w-44 shrink-0 flex flex-col gap-1">
        <div className="flex items-center justify-between mb-1">
          <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Files</span>
          <button
            onClick={() => { setCreating(v => !v); setCreateError(""); }}
            className="rounded px-1.5 py-0.5 text-xs hover:bg-accent"
            title="New function file"
          >＋</button>
        </div>

        {creating && (
          <form onSubmit={handleCreate} className="space-y-1 mb-1">
            <input
              autoFocus
              value={newName}
              onChange={e => setNewName(e.target.value)}
              placeholder="name"
              className="w-full rounded border bg-background px-2 py-1 text-xs outline-none focus:ring-1 focus:ring-ring"
            />
            {createError && <p className="text-[10px] text-destructive">{createError}</p>}
            <div className="flex gap-1">
              <button type="submit" disabled={!newName.trim()}
                className="flex-1 rounded bg-primary px-2 py-0.5 text-[11px] text-primary-foreground disabled:opacity-40">
                Create
              </button>
              <button type="button" onClick={() => setCreating(false)}
                className="rounded border px-2 py-0.5 text-[11px] hover:bg-accent">
                ✕
              </button>
            </div>
          </form>
        )}

        {listLoading && <p className="text-xs text-muted-foreground">Loading…</p>}

        {files.map(f => (
          <div key={f.name}
            className={`group flex items-center justify-between rounded px-2 py-1.5 text-xs cursor-pointer ${selected === f.name ? "bg-accent font-medium" : "hover:bg-accent/50"}`}
            onClick={() => setSelected(f.name)}
          >
            <span className="truncate font-mono">{f.name}.js</span>
            <button
              onClick={e => { e.stopPropagation(); if (confirm(`Delete ${f.name}.js?`)) deleteMutation.mutate(f.name); }}
              className="opacity-0 group-hover:opacity-100 text-destructive/70 hover:text-destructive px-0.5"
            >✕</button>
          </div>
        ))}

        {!listLoading && files.length === 0 && (
          <p className="text-[11px] text-muted-foreground">No functions yet. Click ＋ to create one.</p>
        )}

        {/* Cron status */}
        {crons.length > 0 && (
          <div className="mt-3 pt-3 border-t space-y-1">
            <span className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide">Cron jobs</span>
            {crons.map((c, i) => (
              <div key={i} className="text-[10px] text-muted-foreground leading-tight">
                <code className="text-orange-500">{c.schedule}</code>
                {c.next_run && c.next_run !== "0001-01-01T00:00:00Z" && (
                  <div>next {new Date(c.next_run).toLocaleTimeString()}</div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Editor pane */}
      <div className="flex-1 min-w-0">
        {selected
          ? <FileEditor name={selected} key={selected} />
          : <div className="flex items-center justify-center h-full rounded-lg border bg-muted/30 text-sm text-muted-foreground">
              Select a file or create one
            </div>
        }
      </div>
    </div>
  );
}

// ── Per-file editor ───────────────────────────────────────────────────────────

function FileEditor({ name }: { name: string }) {
  const projectId = useInstance();
  const [code, setCode] = useState("");
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [runPath, setRunPath] = useState("/" + name);
  const [runMethod, setRunMethod] = useState("GET");
  const [runBody, setRunBody] = useState("");
  const [running, setRunning] = useState(false);
  const [runResult, setRunResult] = useState<RunResult | null>(null);
  const [showRun, setShowRun] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["fn-file", projectId, name],
    queryFn: () => functionsApi.get(projectId, name).then(r => r.data),
  });

  useEffect(() => {
    if (data?.code !== undefined && !dirty) setCode(data.code);
  }, [data?.code, dirty]);

  // Ctrl/Cmd+S
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key === "s") {
        e.preventDefault();
        if (dirty && !saving) save(code);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [dirty, saving, code]);

  function save(c: string) {
    setSaving(true);
    setMsg(null);
    functionsApi.save(projectId, name, c).then(() => {
      setDirty(false);
      setMsg({ ok: true, text: "Saved" });
      setTimeout(() => setMsg(null), 2500);
    }).catch((e: unknown) => {
      const err = e as { response?: { data?: { message?: string } } };
      setMsg({ ok: false, text: err?.response?.data?.message ?? "Save failed" });
    }).finally(() => setSaving(false));
  }

  async function runFn() {
    setRunning(true);
    setRunResult(null);
    try {
      const res = await functionsApi.run(projectId, name, {
        method: runMethod,
        path: runPath,
        body: runBody || undefined,
      });
      setRunResult(res.data);
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } };
      setRunResult({ success: false, responded: false, output: "", duration_ms: 0, error: err?.response?.data?.message ?? "Failed" });
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="flex flex-col gap-3 h-full">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <span className="font-mono text-sm font-semibold">{name}.js</span>
          {data?.path && <span className="ml-2 text-xs text-muted-foreground">{data.path}</span>}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {msg && <span className={`text-xs ${msg.ok ? "text-green-600" : "text-destructive"}`}>{msg.text}</span>}
          {dirty && !saving && <span className="text-xs text-amber-500">Unsaved</span>}
          <button onClick={() => save(code)} disabled={!dirty || saving}
            className="rounded bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground hover:opacity-90 disabled:opacity-40">
            {saving ? "Saving…" : "Save"}
          </button>
          <button onClick={() => setShowRun(v => !v)}
            className={`rounded border px-2.5 py-1 text-xs font-medium hover:bg-accent ${showRun ? "bg-accent" : ""}`}>
            ▶ Test
          </button>
        </div>
      </div>

      {/* Hint bar */}
      <div className="rounded border border-blue-200 bg-blue-50 dark:bg-blue-950/30 dark:border-blue-800 px-3 py-1.5 text-[11px] text-blue-800 dark:text-blue-300 flex flex-wrap gap-x-4 gap-y-0.5">
        <span><code className="font-mono">app.get("/path", ctx =&gt; {"{ }"})</code></span>
        <span><code className="font-mono">app.cron("0 * * * *", ctx =&gt; {"{ }"})</code></span>
        <span><code className="font-mono">app.on("beforeCreate", "col", ctx =&gt; {"{ }"})</code></span>
        <span><code className="font-mono">require("./utils")</code> · <kbd className="rounded bg-blue-100 dark:bg-blue-900 px-1">Ctrl+S</kbd> to save</span>
      </div>

      {/* Test runner */}
      {showRun && (
        <div className="rounded-lg border bg-card p-3 space-y-2">
          <div className="flex gap-2">
            <select value={runMethod} onChange={e => setRunMethod(e.target.value)}
              className="rounded border bg-background px-2 py-1 text-xs outline-none focus:ring-1 focus:ring-ring">
              {["GET","POST","PUT","PATCH","DELETE"].map(m => <option key={m}>{m}</option>)}
            </select>
            <input value={runPath} onChange={e => setRunPath(e.target.value)}
              className="flex-1 rounded border bg-background px-2 py-1 text-xs font-mono outline-none focus:ring-1 focus:ring-ring" />
            <button onClick={runFn} disabled={running}
              className="rounded bg-primary px-3 py-1 text-xs text-primary-foreground hover:opacity-90 disabled:opacity-50">
              {running ? "…" : "Run"}
            </button>
          </div>
          {(runMethod === "POST" || runMethod === "PUT" || runMethod === "PATCH") && (
            <textarea value={runBody} onChange={e => setRunBody(e.target.value)}
              placeholder='{"key": "value"}' rows={2}
              className="w-full rounded border bg-background px-2 py-1.5 text-xs font-mono outline-none focus:ring-1 focus:ring-ring resize-none" />
          )}
          {runResult && (
            <div className={`rounded border p-2 text-xs space-y-1 ${runResult.success ? "border-green-500 bg-green-50 dark:bg-green-950/20" : "border-destructive bg-destructive/5"}`}>
              <div className="flex gap-3">
                <span className={`font-semibold ${runResult.success ? "text-green-700 dark:text-green-400" : "text-destructive"}`}>
                  {runResult.success ? "OK" : "Error"} — {runResult.duration_ms}ms
                </span>
                {runResult.status != null && <span className="text-muted-foreground">HTTP {runResult.status}</span>}
              </div>
              {runResult.output && <pre className="whitespace-pre-wrap font-mono text-[11px] opacity-80">{runResult.output.trim()}</pre>}
              {runResult.body  && <pre className="whitespace-pre-wrap font-mono text-[11px] opacity-80">{runResult.body}</pre>}
              {runResult.error && <pre className="whitespace-pre-wrap font-mono text-[11px] text-destructive">{runResult.error}</pre>}
            </div>
          )}
        </div>
      )}

      {/* Code editor */}
      {isLoading
        ? <div className="flex-1 rounded-lg border bg-muted animate-pulse min-h-[380px]" />
        : <CodeEditor value={code} onChange={v => { setCode(v); setDirty(true); }} />
      }
    </div>
  );
}

// ── Code editor (textarea) ────────────────────────────────────────────────────

function CodeEditor({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const ref = useRef<HTMLTextAreaElement>(null);

  function onKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Tab") {
      e.preventDefault();
      const ta = e.currentTarget;
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      const next = value.substring(0, start) + "  " + value.substring(end);
      onChange(next);
      requestAnimationFrame(() => { ta.selectionStart = ta.selectionEnd = start + 2; });
    }
  }

  return (
    <textarea
      ref={ref}
      value={value}
      onChange={e => onChange(e.target.value)}
      onKeyDown={onKeyDown}
      spellCheck={false}
      className="flex-1 min-h-[380px] w-full rounded-lg border bg-[#1e1e1e] text-[#d4d4d4] font-mono text-xs leading-relaxed p-4 resize-y outline-none focus:ring-2 focus:ring-ring"
    />
  );
}
