import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { collectionsApi, projectsApi } from "@/api/client";
import type { Document, Collection, CollectionPermissions, CollectionWebhooks, CollectionSentinels } from "@/api/client";
import { DynamicForm } from "@/components/DynamicForm";
import { SentinelInput } from "@/components/SentinelInput";

export default function CollectionDetailPage() {
  const { id: projectId, colId } = useParams<{ id: string; colId: string }>();
  const qc = useQueryClient();
  const [offset, setOffset] = useState(0);
  const limit = 20;
  const [activeTab, setActiveTab] = useState<"documents" | "settings">("documents");

  const { data: col } = useQuery({
    queryKey: ["collection", projectId, colId],
    queryFn: () => collectionsApi.get(projectId!, colId!).then((r) => r.data),
    enabled: !!projectId && !!colId,
  });

  const { data: project } = useQuery({
    queryKey: ["project", projectId],
    queryFn: () => projectsApi.get(projectId!).then((r) => r.data),
    enabled: !!projectId,
  });

  // ── Create state ──────────────────────────────────────────────────────────
  const [showCreate, setShowCreate] = useState(false);
  const [newDocData, setNewDocData] = useState<Record<string, unknown>>({});
  const [newDocFiles, setNewDocFiles] = useState<Record<string, File>>({});
  const [creating, setCreating] = useState(false);
  const [createJsonMode, setCreateJsonMode] = useState(false);
  const [createJsonRaw, setCreateJsonRaw] = useState("{\n  \n}");
  const [createJsonError, setCreateJsonError] = useState("");

  // ── Edit state ────────────────────────────────────────────────────────────
  const [editingDoc, setEditingDoc] = useState<Document | null>(null);
  const [editJsonRaw, setEditJsonRaw] = useState("");
  const [editJsonMode, setEditJsonMode] = useState(false);
  const [editJsonError, setEditJsonError] = useState("");
  const [editFormData, setEditFormData] = useState<Record<string, unknown>>({});

  const { data: docsData, isLoading } = useQuery({
    queryKey: ["documents", projectId, colId, offset],
    queryFn: () =>
      collectionsApi
        .listDocuments(projectId!, colId!, { limit, offset })
        .then((r) => r.data),
    enabled: !!projectId && !!colId,
  });

  const deleteMutation = useMutation({
    mutationFn: (docId: string) => collectionsApi.deleteDocument(projectId!, colId!, docId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["documents", projectId, colId] }),
  });

  const updateMutation = useMutation({
    mutationFn: ({ docId, data }: { docId: string; data: Record<string, unknown> }) =>
      collectionsApi.updateDocument(projectId!, colId!, docId, data, true),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["documents", projectId, colId] });
      setEditingDoc(null);
    },
  });

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreateJsonError("");
    let data = newDocData;
    if (createJsonMode) {
      try {
        data = JSON.parse(createJsonRaw);
      } catch {
        setCreateJsonError("Invalid JSON");
        return;
      }
    }
    setCreating(true);
    try {
      const hasFiles = Object.keys(newDocFiles).length > 0;
      if (hasFiles && project) {
        const form = new FormData();
        form.append("data", JSON.stringify(data));
        for (const [key, file] of Object.entries(newDocFiles)) {
          form.append(key, file);
        }
        const base = window.location.origin;
        await fetch(`${base}/collections/${colId}/documents`, {
          method: "POST",
          headers: { "x-api-key": project.api_key },
          body: form,
        });
      } else {
        await collectionsApi.createDocument(projectId!, colId!, data);
      }
      qc.invalidateQueries({ queryKey: ["documents", projectId, colId] });
      setNewDocData({});
      setNewDocFiles({});
      setCreateJsonRaw("{\n  \n}");
      setShowCreate(false);
    } finally {
      setCreating(false);
    }
  }

  function openEdit(doc: Document) {
    setEditingDoc(doc);
    setEditFormData({ ...doc.data });
    setEditJsonRaw(JSON.stringify(doc.data, null, 2));
    setEditJsonMode(false);
    setEditJsonError("");
  }

  function handleEditSave(e: React.FormEvent) {
    e.preventDefault();
    if (!editingDoc) return;
    setEditJsonError("");
    let data: Record<string, unknown>;
    if (editJsonMode) {
      try {
        data = JSON.parse(editJsonRaw);
      } catch {
        setEditJsonError("Invalid JSON");
        return;
      }
    } else {
      data = editFormData;
    }
    updateMutation.mutate({ docId: editingDoc.id, data });
  }

  const docs = docsData?.data ?? [];
  const [expandedId, setExpandedId] = useState<string | null>(null);

  return (
    <div className="space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/projects" className="hover:underline">Projects</Link>
        <span>/</span>
        <Link to={`/projects/${projectId}`} className="hover:underline">{projectId}</Link>
        <span>/</span>
        <span className="text-foreground">{col?.name ?? colId}</span>
      </div>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">{col?.name}</h1>
          <p className="text-sm text-muted-foreground">{docsData?.total ?? 0} document{docsData?.total !== 1 ? "s" : ""}</p>
        </div>
        {activeTab === "documents" && (
          <button onClick={() => setShowCreate((v) => !v)}
            className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90">
            {showCreate ? "Cancel" : "+ New document"}
          </button>
        )}
      </div>

      {/* Tab bar */}
      <div className="flex gap-1 border-b">
        {(["documents", "settings"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setActiveTab(t)}
            className={`px-4 py-2 text-sm font-medium capitalize transition-colors ${
              activeTab === t
                ? "border-b-2 border-primary text-foreground"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {/* Collection Settings tab */}
      {activeTab === "settings" && col && (
        <CollectionSettings projectId={projectId!} colId={colId!} col={col} />
      )}

      {/* Documents tab */}
      {activeTab === "documents" && (
        <>
          {showCreate && (
            <form onSubmit={handleCreate} className="rounded-lg border bg-card p-4 space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">New document</p>
                <button
                  type="button"
                  onClick={() => { setCreateJsonMode((v) => !v); setCreateJsonError(""); }}
                  className="text-xs text-muted-foreground hover:text-foreground underline"
                >
                  {createJsonMode ? "Switch to form" : "Switch to JSON"}
                </button>
              </div>

              {createJsonMode ? (
                <div>
                  <textarea
                    rows={8}
                    value={createJsonRaw}
                    spellCheck={false}
                    onChange={(e) => { setCreateJsonRaw(e.target.value); setCreateJsonError(""); }}
                    className="w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-ring resize-y"
                    placeholder={'{\n  "title": "Hello",\n  "published": true\n}'}
                  />
                  {createJsonError && <p className="text-xs text-destructive mt-1">{createJsonError}</p>}
                </div>
              ) : (
                <DynamicForm
                  onChange={(data, files) => { setNewDocData(data); setNewDocFiles(files); }}
                />
              )}

              <button type="submit" disabled={creating}
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50">
                {creating ? "Creating…" : "Create document"}
              </button>
            </form>
          )}

          {/* Edit modal */}
          {editingDoc && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
              <div className="w-full max-w-2xl rounded-xl border bg-card shadow-xl max-h-[90vh] flex flex-col">
                <div className="flex items-center justify-between px-5 py-4 border-b">
                  <div>
                    <h2 className="font-semibold text-sm">Edit document</h2>
                    <p className="text-xs text-muted-foreground font-mono">{editingDoc.id}</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <button
                      type="button"
                      onClick={() => { setEditJsonMode((v) => !v); setEditJsonError(""); }}
                      className="text-xs text-muted-foreground hover:text-foreground underline"
                    >
                      {editJsonMode ? "Switch to form" : "Switch to JSON"}
                    </button>
                    <button type="button" onClick={() => setEditingDoc(null)}
                      className="text-muted-foreground hover:text-foreground text-lg leading-none">×</button>
                  </div>
                </div>

                <form onSubmit={handleEditSave} className="flex-1 overflow-y-auto p-5 space-y-4">
                  {editJsonMode ? (
                    <div>
                      <textarea
                        rows={14}
                        value={editJsonRaw}
                        spellCheck={false}
                        onChange={(e) => { setEditJsonRaw(e.target.value); setEditJsonError(""); }}
                        className="w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-ring resize-y"
                      />
                      {editJsonError && <p className="text-xs text-destructive mt-1">{editJsonError}</p>}
                    </div>
                  ) : (
                    <EditForm
                      initialData={editingDoc.data}
                      onChange={(data) => setEditFormData(data)}
                      projectApiKey={project?.api_key}
                      collectionName={col?.name}
                    />
                  )}

                  <div className="flex items-center gap-3 pt-2 border-t">
                    <button type="submit" disabled={updateMutation.isPending}
                      className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50">
                      {updateMutation.isPending ? "Saving…" : "Save changes"}
                    </button>
                    <button type="button" onClick={() => setEditingDoc(null)}
                      className="rounded-md border px-4 py-2 text-sm hover:bg-muted">
                      Cancel
                    </button>
                    {updateMutation.isError && (
                      <p className="text-xs text-destructive">Failed to save.</p>
                    )}
                  </div>
                </form>
              </div>
            </div>
          )}

          {/* Documents table */}
          <div className="rounded-lg border bg-card">
            {isLoading ? (
              <div className="p-6 text-center text-sm text-muted-foreground">Loading…</div>
            ) : docs.length === 0 ? (
              <div className="p-6 text-center text-sm text-muted-foreground">No documents</div>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="px-4 py-3">ID</th>
                    <th className="px-4 py-3">Data preview</th>
                    <th className="px-4 py-3">Created</th>
                    <th className="px-4 py-3"></th>
                  </tr>
                </thead>
                <tbody>
                  {docs.map((doc) => {
                    const isExpanded = expandedId === doc.id;
                    const dataEntries = doc.data && typeof doc.data === "object"
                      ? Object.entries(doc.data as Record<string, unknown>)
                      : [];
                    return (
                      <>
                        <tr
                          key={doc.id}
                          className="border-b last:border-0 cursor-pointer hover:bg-muted/20 transition-colors"
                          onClick={() => setExpandedId(isExpanded ? null : doc.id)}
                        >
                          <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{doc.id.slice(0, 8)}…</td>
                          <td className="px-4 py-3">
                            <div className="flex flex-wrap gap-1.5">
                              {dataEntries.slice(0, 4).map(([k, v]) => (
                                <span key={k} className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-xs">
                                  <span className="font-medium text-foreground">{k}:</span>
                                  <span className="text-muted-foreground truncate max-w-[100px]">
                                    {typeof v === "object" ? (Array.isArray(v) ? `[${(v as unknown[]).length}]` : "{…}") : String(v)}
                                  </span>
                                </span>
                              ))}
                              {dataEntries.length > 4 && (
                                <span className="text-xs text-muted-foreground">+{dataEntries.length - 4} more</span>
                              )}
                              {dataEntries.length === 0 && (
                                <span className="text-xs text-muted-foreground italic">empty</span>
                              )}
                            </div>
                          </td>
                          <td className="px-4 py-3 text-xs text-muted-foreground whitespace-nowrap">
                            {new Date(doc.created_at).toLocaleDateString()}
                          </td>
                          <td className="px-4 py-3 text-right">
                            <div className="flex justify-end gap-2">
                              <button
                                onClick={(e) => { e.stopPropagation(); openEdit(doc); }}
                                className="text-xs text-primary hover:underline"
                              >
                                Edit
                              </button>
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  if (confirm("Delete this document?")) deleteMutation.mutate(doc.id);
                                }}
                                className="text-xs text-destructive hover:underline"
                              >
                                Delete
                              </button>
                            </div>
                          </td>
                        </tr>
                        {isExpanded && (
                          <tr key={`${doc.id}-expanded`} className="border-b bg-muted/10">
                            <td colSpan={4} className="px-4 py-3">
                              <div className="grid gap-1.5 sm:grid-cols-2 lg:grid-cols-3">
                                {dataEntries.map(([k, v]) => (
                                  <div key={k} className="rounded-md border bg-card px-3 py-2">
                                    <p className="text-xs font-medium text-muted-foreground mb-0.5">{k}</p>
                                    <p className="text-sm font-mono break-all">
                                      {typeof v === "string" && (v.startsWith("http://") || v.startsWith("https://"))
                                        ? <a href={v} target="_blank" rel="noopener noreferrer" className="text-primary underline">{v}</a>
                                        : typeof v === "object"
                                          ? JSON.stringify(v, null, 2)
                                          : String(v)}
                                    </p>
                                  </div>
                                ))}
                                {dataEntries.length === 0 && (
                                  <p className="text-xs text-muted-foreground col-span-full">No data fields</p>
                                )}
                              </div>
                              <div className="mt-3 flex items-center gap-4">
                                <p className="text-xs text-muted-foreground font-mono">ID: {doc.id}</p>
                                <button
                                  onClick={(e) => { e.stopPropagation(); openEdit(doc); }}
                                  className="text-xs text-primary hover:underline"
                                >
                                  Edit this document →
                                </button>
                              </div>
                            </td>
                          </tr>
                        )}
                      </>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>

          {/* Pagination */}
          <div className="flex gap-2">
            <button
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - limit))}
              className="rounded border px-3 py-1 text-sm disabled:opacity-40"
            >
              Previous
            </button>
            <button
              disabled={!docsData?.has_more}
              onClick={() => setOffset(offset + limit)}
              className="rounded border px-3 py-1 text-sm disabled:opacity-40"
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  );
}

// ── CollectionSettings ────────────────────────────────────────────────────────

interface CollectionSettingsProps {
  projectId: string;
  colId: string;
  col: Collection & { document_count?: number };
}

const ROLES = ["*", "admin", "user", "editor", "viewer", "guest"];

function CollectionSettings({ projectId, colId, col }: CollectionSettingsProps) {
  const qc = useQueryClient();
  const [saved, setSaved] = useState(false);

  const perms = col.permissions ?? { create: [], read: [], update: [], delete: [] };
  const wh = col.webhooks ?? {};

  const [permissions, setPermissions] = useState<CollectionPermissions>({
    create: perms.create ?? [],
    read: perms.read ?? [],
    update: perms.update ?? [],
    delete: perms.delete ?? [],
  });

  const [webhooks, setWebhooks] = useState<CollectionWebhooks>({
    pre_save: wh.pre_save ?? "",
    post_save: wh.post_save ?? "",
    pre_delete: wh.pre_delete ?? "",
    post_delete: wh.post_delete ?? "",
  });

  const snt = col.sentinels ?? {};
  const [sentinels, setSentinels] = useState<CollectionSentinels>({
    list: snt.list ?? "",
    view: snt.view ?? "",
    create: snt.create ?? "",
    update: snt.update ?? "",
    delete: snt.delete ?? "",
  });

  const updateMutation = useMutation({
    mutationFn: () =>
      collectionsApi.update(projectId, colId, { permissions, webhooks, sentinels }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["collection", projectId, colId] });
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    },
  });

  function toggleRole(op: keyof CollectionPermissions, role: string) {
    setPermissions((prev) => {
      const arr = prev[op];
      const next = arr.includes(role) ? arr.filter((r) => r !== role) : [...arr, role];
      return { ...prev, [op]: next };
    });
  }

  function addCustomRole(op: keyof CollectionPermissions) {
    const role = prompt("Enter role name (e.g. moderator):");
    if (!role) return;
    setPermissions((prev) => ({
      ...prev,
      [op]: [...prev[op], role],
    }));
  }

  return (
    <form onSubmit={(e) => { e.preventDefault(); updateMutation.mutate(); }} className="space-y-6">
      {/* Access rules */}
      <div className="rounded-lg border bg-card p-5 space-y-4">
        <div>
          <h3 className="font-semibold">Access rules</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Specify which roles can perform each operation. An empty list means <strong>anyone</strong> (including unauthenticated users) can access. Use <code className="bg-muted px-1 rounded text-xs">*</code> to allow all authenticated users.
          </p>
        </div>
        <div className="grid gap-5 sm:grid-cols-2">
          {(["create", "read", "update", "delete"] as const).map((op) => (
            <div key={op}>
              <p className="text-sm font-medium capitalize mb-2">{op}</p>
              <div className="flex flex-wrap gap-1.5">
                {ROLES.map((role) => {
                  const active = permissions[op].includes(role);
                  return (
                    <button
                      key={role}
                      type="button"
                      onClick={() => toggleRole(op, role)}
                      className={`rounded-full px-2.5 py-0.5 text-xs font-mono transition-colors ${
                        active
                          ? "bg-primary text-primary-foreground"
                          : "bg-muted text-muted-foreground hover:bg-muted/80"
                      }`}
                    >
                      {role}
                    </button>
                  );
                })}
                {/* Show any custom roles not in ROLES */}
                {permissions[op]
                  .filter((r) => !ROLES.includes(r))
                  .map((role) => (
                    <button
                      key={role}
                      type="button"
                      onClick={() => toggleRole(op, role)}
                      className="rounded-full px-2.5 py-0.5 text-xs font-mono bg-primary text-primary-foreground"
                    >
                      {role}
                    </button>
                  ))}
                <button
                  type="button"
                  onClick={() => addCustomRole(op)}
                  className="rounded-full px-2.5 py-0.5 text-xs border border-dashed text-muted-foreground hover:text-foreground"
                >
                  + custom
                </button>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {permissions[op].length === 0
                  ? "Open — no auth required"
                  : `Allowed: ${permissions[op].join(", ")}`}
              </p>
            </div>
          ))}
        </div>
      </div>

      {/* Webhooks */}
      <div className="rounded-lg border bg-card p-5 space-y-4">
        <div>
          <h3 className="font-semibold">Webhooks</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Cocobase will POST a JSON payload to these URLs when documents change. Leave empty to disable.
          </p>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {([
            ["pre_save", "Pre-save", "Fired before a document is created or updated."],
            ["post_save", "Post-save", "Fired after a document is created or updated."],
            ["pre_delete", "Pre-delete", "Fired before a document is deleted."],
            ["post_delete", "Post-delete", "Fired after a document is deleted."],
          ] as const).map(([key, label, desc]) => (
            <div key={key}>
              <label className="block text-sm font-medium mb-1">{label}</label>
              <p className="text-xs text-muted-foreground mb-1.5">{desc}</p>
              <input
                type="url"
                placeholder="https://yourserver.com/webhook"
                value={webhooks[key] ?? ""}
                onChange={(e) => setWebhooks((w) => ({ ...w, [key]: e.target.value }))}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
              />
            </div>
          ))}
        </div>
      </div>

      {/* Sentinels */}
      <div className="rounded-lg border bg-card p-5 space-y-4">
        <div>
          <h3 className="font-semibold">Sentinels</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Write dynamic security expressions evaluated per-request. Use <code className="bg-muted px-1 rounded text-xs">$req.user.*</code> for the authenticated user and <code className="bg-muted px-1 rounded text-xs">$doc.*</code> for the document. Empty = no restriction.
          </p>
          <div className="mt-2 rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground space-y-1">
            <p><strong>Examples:</strong></p>
            <p><code>$req.user.id == $doc.owner_id</code> — only owner can access</p>
            <p><code>$req.user.roles contains "admin"</code> — admins only</p>
            <p><code>$doc.published == true || $req.user.id == $doc.author_id</code> — published or own</p>
          </div>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {([
            ["list", "List sentinel", "Filter documents in list queries. Non-matching docs are silently excluded."],
            ["view", "View sentinel", "Restrict single-document GET. Returns 403 if denied."],
            ["create", "Create sentinel", "Gate document creation. $doc.* refers to the new data being submitted."],
            ["update", "Update sentinel", "Restrict updates. $doc.* refers to the current stored data."],
            ["delete", "Delete sentinel", "Restrict deletes. $doc.* refers to the document being deleted."],
          ] as const).map(([key, label, desc]) => (
            <SentinelInput
              key={key}
              label={label}
              desc={desc}
              value={sentinels[key] ?? ""}
              onChange={(v) => setSentinels((s) => ({ ...s, [key]: v }))}
            />
          ))}
        </div>
      </div>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={updateMutation.isPending}
          className="rounded-md bg-primary px-5 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50 hover:opacity-90">
          {updateMutation.isPending ? "Saving…" : "Save settings"}
        </button>
        {saved && <span className="text-sm text-green-600">Saved!</span>}
        {updateMutation.isError && <span className="text-sm text-destructive">Failed to save.</span>}
      </div>
    </form>
  );
}

// ── EditForm ──────────────────────────────────────────────────────────────────

interface EditFormProps {
  initialData: Record<string, unknown>;
  onChange: (data: Record<string, unknown>) => void;
  projectApiKey?: string;
  collectionName?: string;
}

function EditForm({ initialData, onChange }: EditFormProps) {
  const [data, setData] = useState<Record<string, unknown>>(initialData);

  function update(key: string, value: unknown) {
    const next = { ...data, [key]: value };
    setData(next);
    onChange(next);
  }

  function addField() {
    const key = prompt("New field name:");
    if (!key || key in data) return;
    update(key, "");
  }

  function removeField(key: string) {
    const next = { ...data };
    delete next[key];
    setData(next);
    onChange(next);
  }

  return (
    <div className="space-y-3">
      {Object.entries(data).map(([key, value]) => {
        const isLong = typeof value === "string" && value.length > 80;
        const isUrl = typeof value === "string" && (value.startsWith("http://") || value.startsWith("https://"));
        return (
          <div key={key} className="group">
            <div className="flex items-center justify-between mb-1">
              <label className="text-xs font-medium text-muted-foreground">{key}</label>
              <button
                type="button"
                onClick={() => removeField(key)}
                className="text-xs text-destructive opacity-0 group-hover:opacity-100 transition-opacity"
              >
                Remove
              </button>
            </div>
            {typeof value === "boolean" ? (
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={value}
                  onChange={(e) => update(key, e.target.checked)}
                  className="h-4 w-4 rounded border"
                />
                <span className="text-sm text-muted-foreground">{value ? "true" : "false"}</span>
              </div>
            ) : typeof value === "number" ? (
              <input
                type="number"
                value={value}
                onChange={(e) => update(key, parseFloat(e.target.value) || 0)}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
              />
            ) : isUrl ? (
              <div className="space-y-1.5">
                <input
                  type="url"
                  value={value as string}
                  onChange={(e) => update(key, e.target.value)}
                  className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
                />
                <p className="text-xs text-muted-foreground">This field contains a URL (possibly a file). Edit the URL directly or upload a file from the create form to generate a new URL.</p>
              </div>
            ) : typeof value === "object" ? (
              <textarea
                rows={3}
                value={JSON.stringify(value, null, 2)}
                onChange={(e) => {
                  try { update(key, JSON.parse(e.target.value)); } catch { update(key, e.target.value); }
                }}
                className="w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-ring resize-y"
              />
            ) : isLong ? (
              <textarea
                rows={4}
                value={value as string}
                onChange={(e) => update(key, e.target.value)}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring resize-y"
              />
            ) : (
              <input
                type="text"
                value={value as string}
                onChange={(e) => update(key, e.target.value)}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
              />
            )}
          </div>
        );
      })}
      <button
        type="button"
        onClick={addField}
        className="text-xs text-muted-foreground hover:text-foreground underline"
      >
        + Add field
      </button>
    </div>
  );
}
