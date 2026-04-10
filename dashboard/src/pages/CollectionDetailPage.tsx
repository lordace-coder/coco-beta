import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { collectionsApi, projectsApi } from "@/api/client";
import { DynamicForm } from "@/components/DynamicForm";

export default function CollectionDetailPage() {
  const { id: projectId, colId } = useParams<{ id: string; colId: string }>();
  const qc = useQueryClient();
  const [offset, setOffset] = useState(0);
  const limit = 20;

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

  const [showCreate, setShowCreate] = useState(false);
  const [newDocData, setNewDocData] = useState<Record<string, unknown>>({});
  const [newDocFiles, setNewDocFiles] = useState<Record<string, File>>({});
  const [creating, setCreating] = useState(false);
  const [jsonMode, setJsonMode] = useState(false);
  const [jsonRaw, setJsonRaw] = useState("{\n  \n}");
  const [jsonError, setJsonError] = useState("");

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

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setJsonError("");
    let data = newDocData;
    if (jsonMode) {
      try {
        data = JSON.parse(jsonRaw);
      } catch {
        setJsonError("Invalid JSON");
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
        await fetch(`${base}/api/v1/collections/${col?.name}/documents`, {
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
      setJsonRaw("{\n  \n}");
      setShowCreate(false);
    } finally {
      setCreating(false);
    }
  }

  const docs = docsData?.data ?? [];
  const [expandedId, setExpandedId] = useState<string | null>(null);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/projects" className="hover:underline">Projects</Link>
        <span>/</span>
        <Link to={`/projects/${projectId}`} className="hover:underline">{projectId}</Link>
        <span>/</span>
        <span className="text-foreground">{col?.name ?? colId}</span>
      </div>

      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">{col?.name}</h1>
          <p className="text-sm text-muted-foreground">{docsData?.total ?? 0} document{docsData?.total !== 1 ? "s" : ""}</p>
        </div>
        <button onClick={() => setShowCreate((v) => !v)}
          className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90">
          {showCreate ? "Cancel" : "+ New document"}
        </button>
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="rounded-lg border bg-card p-4 space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium">New document</p>
            <button
              type="button"
              onClick={() => { setJsonMode((v) => !v); setJsonError(""); }}
              className="text-xs text-muted-foreground hover:text-foreground underline"
            >
              {jsonMode ? "Switch to form" : "Switch to JSON"}
            </button>
          </div>

          {jsonMode ? (
            <div>
              <textarea
                rows={8}
                value={jsonRaw}
                spellCheck={false}
                onChange={(e) => { setJsonRaw(e.target.value); setJsonError(""); }}
                className="w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-ring resize-y"
                placeholder={'{\n  "title": "Hello",\n  "published": true\n}'}
              />
              {jsonError && <p className="text-xs text-destructive mt-1">{jsonError}</p>}
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
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            if (confirm("Delete this document?")) deleteMutation.mutate(doc.id);
                          }}
                          className="text-xs text-destructive hover:underline"
                        >
                          Delete
                        </button>
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
                                  {typeof v === "object"
                                    ? JSON.stringify(v, null, 2)
                                    : String(v)}
                                </p>
                              </div>
                            ))}
                            {dataEntries.length === 0 && (
                              <p className="text-xs text-muted-foreground col-span-full">No data fields</p>
                            )}
                          </div>
                          <p className="mt-2 text-xs text-muted-foreground font-mono">ID: {doc.id}</p>
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
    </div>
  );
}
