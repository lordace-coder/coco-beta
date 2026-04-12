import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { collectionsApi } from "@/api/client";
import { CopyButton } from "@/components/CopyButton";
import { useInstance } from "@/hooks/useInstance";

export function CollectionsTab() {
  const projectId = useInstance();
  const qc = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["collections", projectId],
    queryFn: () => collectionsApi.list(projectId).then((r) => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (colId: string) => collectionsApi.delete(projectId, colId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["collections", projectId] }),
  });

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setCreating(true);
    try {
      await collectionsApi.create(projectId, name.trim());
      qc.invalidateQueries({ queryKey: ["collections", projectId] });
      setName("");
      setShowCreate(false);
    } finally {
      setCreating(false);
    }
  }

  const cols = data?.data ?? [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{cols.length} collection{cols.length !== 1 ? "s" : ""}</p>
        <button
          onClick={() => setShowCreate((v) => !v)}
          className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90"
        >
          {showCreate ? "Cancel" : "+ New collection"}
        </button>
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="flex gap-2 rounded-lg border bg-card p-4">
          <input
            type="text"
            placeholder="Collection name (e.g. posts)"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="flex-1 rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
          />
          <button type="submit" disabled={creating}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50">
            {creating ? "Creating…" : "Create"}
          </button>
        </form>
      )}

      <div className="rounded-lg border bg-card overflow-hidden">
        {isLoading ? (
          <p className="p-6 text-center text-sm text-muted-foreground">Loading…</p>
        ) : cols.length === 0 ? (
          <p className="p-6 text-center text-sm text-muted-foreground">No collections yet</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left text-muted-foreground text-xs">
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">ID</th>
                <th className="px-4 py-2">Created</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {cols.map((col) => (
                <tr key={col.id} className="border-b last:border-0 hover:bg-muted/20">
                  <td className="px-4 py-2.5">
                    <Link to={`/collections/${col.id}`} className="font-medium hover:underline">{col.name}</Link>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className="font-mono text-xs text-muted-foreground">{col.id.slice(0, 8)}…</span>
                    <CopyButton value={col.id} />
                  </td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">{new Date(col.created_at).toLocaleDateString()}</td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => { if (confirm(`Delete "${col.name}" and all its documents?`)) deleteMutation.mutate(col.id); }}
                      className="text-xs text-destructive hover:underline"
                    >Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
