import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projectsApi, usersApi, collectionsApi, filesApi } from "@/api/client";

type Tab = "overview" | "users" | "collections" | "files";

export default function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [tab, setTab] = useState<Tab>("overview");
  const qc = useQueryClient();

  const { data: project, isLoading } = useQuery({
    queryKey: ["project", id],
    queryFn: () => projectsApi.get(id!).then((r) => r.data),
    enabled: !!id,
  });

  const regenMutation = useMutation({
    mutationFn: () => projectsApi.regenKey(id!),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["project", id] }),
  });

  if (isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading…</div>;
  if (!project) return <div className="p-6 text-sm text-muted-foreground">Project not found</div>;

  const tabs: { key: Tab; label: string }[] = [
    { key: "overview", label: "Overview" },
    { key: "users", label: "Users" },
    { key: "collections", label: "Collections" },
    { key: "files", label: "Files" },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/projects" className="hover:underline">Projects</Link>
        <span>/</span>
        <span className="text-foreground">{project.name}</span>
      </div>

      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">{project.name}</h1>
          <span
            className={`mt-1 inline-block rounded px-2 py-0.5 text-xs font-medium ${project.active ? "bg-green-100 text-green-700" : "bg-muted text-muted-foreground"}`}
          >
            {project.active ? "active" : "inactive"}
          </span>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${tab === t.key ? "border-b-2 border-primary text-primary" : "text-muted-foreground hover:text-foreground"}`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "overview" && (
        <div className="space-y-4">
          <div className="rounded-lg border bg-card p-4">
            <h2 className="mb-3 font-semibold">Project Details</h2>
            <div className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">ID</span>
                <span className="font-mono">{project.id}</span>
              </div>
              <div className="flex items-center justify-between gap-4">
                <span className="text-muted-foreground">API Key</span>
                <div className="flex items-center gap-2">
                  <span className="max-w-xs truncate font-mono text-xs">{project.api_key}</span>
                  <button
                    onClick={() => {
                      if (confirm("Regenerate API key? This will invalidate the current key.")) {
                        regenMutation.mutate();
                      }
                    }}
                    className="text-xs text-primary hover:underline"
                  >
                    Regenerate
                  </button>
                </div>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Created</span>
                <span>{new Date(project.created_at).toLocaleString()}</span>
              </div>
            </div>
          </div>

          {project.allowed_origins && project.allowed_origins.length > 0 && (
            <div className="rounded-lg border bg-card p-4">
              <h2 className="mb-2 font-semibold">Allowed Origins</h2>
              <div className="flex flex-wrap gap-2">
                {project.allowed_origins.map((o) => (
                  <span key={o} className="rounded bg-muted px-2 py-0.5 font-mono text-xs">{o}</span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {tab === "users" && <UsersTab projectId={id!} />}
      {tab === "collections" && <CollectionsTab projectId={id!} />}
      {tab === "files" && <FilesTab projectId={id!} />}
    </div>
  );
}

function UsersTab({ projectId }: { projectId: string }) {
  const qc = useQueryClient();
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const { data, isLoading } = useQuery({
    queryKey: ["users", projectId, offset],
    queryFn: () => usersApi.list(projectId, { limit, offset }).then((r) => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (userId: string) => usersApi.delete(projectId, userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users", projectId] }),
  });

  const users = data?.data ?? [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">Total: {data?.total ?? 0}</p>
      </div>
      <div className="rounded-lg border bg-card">
        {isLoading ? (
          <div className="p-6 text-center text-sm text-muted-foreground">Loading…</div>
        ) : users.length === 0 ? (
          <div className="p-6 text-center text-sm text-muted-foreground">No users yet</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="px-4 py-3">Email</th>
                <th className="px-4 py-3">Verified</th>
                <th className="px-4 py-3">Provider</th>
                <th className="px-4 py-3">Joined</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b last:border-0">
                  <td className="px-4 py-3">{u.email}</td>
                  <td className="px-4 py-3">
                    <span className={`rounded px-2 py-0.5 text-xs ${u.email_verified ? "bg-green-100 text-green-700" : "bg-muted text-muted-foreground"}`}>
                      {u.email_verified ? "yes" : "no"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">{u.oauth_provider ?? "email"}</td>
                  <td className="px-4 py-3 text-muted-foreground">{new Date(u.created_at).toLocaleDateString()}</td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => {
                        if (confirm(`Delete user ${u.email}?`)) deleteMutation.mutate(u.id);
                      }}
                      className="text-xs text-destructive hover:underline"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
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
          disabled={!data?.has_more}
          onClick={() => setOffset(offset + limit)}
          className="rounded border px-3 py-1 text-sm disabled:opacity-40"
        >
          Next
        </button>
      </div>
    </div>
  );
}

function CollectionsTab({ projectId }: { projectId: string }) {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["collections", projectId],
    queryFn: () => collectionsApi.list(projectId).then((r) => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (colId: string) => collectionsApi.delete(projectId, colId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["collections", projectId] }),
  });

  const cols = data?.data ?? [];

  return (
    <div className="rounded-lg border bg-card">
      {isLoading ? (
        <div className="p-6 text-center text-sm text-muted-foreground">Loading…</div>
      ) : cols.length === 0 ? (
        <div className="p-6 text-center text-sm text-muted-foreground">No collections</div>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b text-left text-muted-foreground">
              <th className="px-4 py-3">Name</th>
              <th className="px-4 py-3">Created</th>
              <th className="px-4 py-3"></th>
            </tr>
          </thead>
          <tbody>
            {cols.map((col) => (
              <tr key={col.id} className="border-b last:border-0 hover:bg-muted/30">
                <td className="px-4 py-3">
                  <Link
                    to={`/projects/${projectId}/collections/${col.id}`}
                    className="font-medium hover:underline"
                  >
                    {col.name}
                  </Link>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  {new Date(col.created_at).toLocaleDateString()}
                </td>
                <td className="px-4 py-3 text-right">
                  <button
                    onClick={() => {
                      if (confirm(`Delete collection "${col.name}" and all its documents?`)) {
                        deleteMutation.mutate(col.id);
                      }
                    }}
                    className="text-xs text-destructive hover:underline"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function FilesTab({ projectId }: { projectId: string }) {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["files", projectId],
    queryFn: () => filesApi.list(projectId).then((r) => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (key: string) => filesApi.delete(projectId, key),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["files", projectId] }),
  });

  const files = data?.data ?? [];

  return (
    <div className="rounded-lg border bg-card">
      {isLoading ? (
        <div className="p-6 text-center text-sm text-muted-foreground">Loading…</div>
      ) : files.length === 0 ? (
        <div className="p-6 text-center text-sm text-muted-foreground">No files</div>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b text-left text-muted-foreground">
              <th className="px-4 py-3">File</th>
              <th className="px-4 py-3">Size</th>
              <th className="px-4 py-3">Modified</th>
              <th className="px-4 py-3"></th>
            </tr>
          </thead>
          <tbody>
            {files.map((f) => (
              <tr key={f.key} className="border-b last:border-0">
                <td className="px-4 py-3">
                  <a href={f.url} target="_blank" rel="noopener noreferrer" className="font-mono text-xs hover:underline">
                    {f.key.split("/").pop()}
                  </a>
                </td>
                <td className="px-4 py-3 text-muted-foreground">{formatBytes(f.size)}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  {new Date(f.last_modified).toLocaleDateString()}
                </td>
                <td className="px-4 py-3 text-right">
                  <button
                    onClick={() => {
                      if (confirm(`Delete file "${f.key}"?`)) deleteMutation.mutate(f.key);
                    }}
                    className="text-xs text-destructive hover:underline"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
