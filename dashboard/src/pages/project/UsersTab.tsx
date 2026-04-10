import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { usersApi } from "@/api/client";
import { DynamicForm } from "@/components/DynamicForm";

const LIMIT = 20;

export function UsersTab({ projectId }: { projectId: string }) {
  const qc = useQueryClient();
  const [offset, setOffset] = useState(0);
  const [showCreate, setShowCreate] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["users", projectId, offset],
    queryFn: () => usersApi.list(projectId, { limit: LIMIT, offset }).then((r) => r.data),
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
        <button
          onClick={() => setShowCreate((v) => !v)}
          className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90"
        >
          {showCreate ? "Cancel" : "+ Create user"}
        </button>
      </div>

      {showCreate && (
        <CreateUserForm
          projectId={projectId}
          onDone={() => {
            setShowCreate(false);
            qc.invalidateQueries({ queryKey: ["users", projectId] });
          }}
        />
      )}

      <div className="rounded-lg border bg-card overflow-hidden">
        {isLoading ? (
          <p className="p-6 text-center text-sm text-muted-foreground">Loading…</p>
        ) : users.length === 0 ? (
          <p className="p-6 text-center text-sm text-muted-foreground">No users yet</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left text-muted-foreground text-xs">
                <th className="px-4 py-2">Email</th>
                <th className="px-4 py-2">Verified</th>
                <th className="px-4 py-2">Provider</th>
                <th className="px-4 py-2">Roles</th>
                <th className="px-4 py-2">Joined</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b last:border-0 hover:bg-muted/20">
                  <td className="px-4 py-2.5">
                    <p className="font-medium">{u.email}</p>
                    <p className="text-xs text-muted-foreground font-mono">{u.id.slice(0, 8)}…</p>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${u.email_verified ? "bg-green-100 text-green-700" : "bg-amber-100 text-amber-700"}`}>
                      {u.email_verified ? "Verified" : "Pending"}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-muted-foreground text-xs capitalize">{u.oauth_provider ?? "email"}</td>
                  <td className="px-4 py-2.5">
                    {u.roles?.length
                      ? u.roles.map((r: string) => <span key={r} className="mr-1 rounded bg-muted px-1.5 py-0.5 text-xs">{r}</span>)
                      : <span className="text-xs text-muted-foreground">—</span>}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">{new Date(u.created_at).toLocaleDateString()}</td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => { if (confirm(`Delete ${u.email}?`)) deleteMutation.mutate(u.id); }}
                      className="text-xs text-destructive hover:underline"
                    >Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <Pagination offset={offset} limit={LIMIT} hasMore={!!data?.has_more} onChange={setOffset} />
    </div>
  );
}

function CreateUserForm({ projectId, onDone }: { projectId: string; onDone: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [roles, setRoles] = useState("");
  const [extraData, setExtraData] = useState<Record<string, unknown>>({});
  const [jsonMode, setJsonMode] = useState(false);
  const [jsonRaw, setJsonRaw] = useState("{}");
  const [jsonError, setJsonError] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setJsonError("");
    let data = extraData;
    if (jsonMode) {
      try {
        data = JSON.parse(jsonRaw);
      } catch {
        setJsonError("Invalid JSON");
        return;
      }
    }
    setLoading(true);
    try {
      await usersApi.create(projectId, {
        email,
        password,
        roles: roles.split(",").map((r) => r.trim()).filter(Boolean),
        data,
      });
      onDone();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message;
      setError(msg ?? "Failed to create user");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="rounded-lg border bg-card p-4 space-y-4">
      <h3 className="font-medium text-sm">Create user</h3>
      <div className="grid gap-3 sm:grid-cols-2">
        <div>
          <label className="block text-xs font-medium mb-1">Email *</label>
          <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
        </div>
        <div>
          <label className="block text-xs font-medium mb-1">Password *</label>
          <input type="password" required minLength={6} value={password} onChange={(e) => setPassword(e.target.value)}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
        </div>
        <div className="sm:col-span-2">
          <label className="block text-xs font-medium mb-1">Roles <span className="text-muted-foreground">(comma-separated, optional)</span></label>
          <input type="text" placeholder="admin, editor" value={roles} onChange={(e) => setRoles(e.target.value)}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
        </div>
      </div>
      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="block text-xs font-medium">
            Extra data <span className="text-muted-foreground">— any fields you want stored on this user</span>
          </label>
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
              rows={5}
              value={jsonRaw}
              spellCheck={false}
              onChange={(e) => { setJsonRaw(e.target.value); setJsonError(""); }}
              className="w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-ring resize-y"
              placeholder='{"age": 30, "plan": "pro"}'
            />
            {jsonError && <p className="text-xs text-destructive mt-1">{jsonError}</p>}
          </div>
        ) : (
          <DynamicForm onChange={(data) => setExtraData(data)} />
        )}
      </div>
      {error && <p className="text-xs text-destructive">{error}</p>}
      <button type="submit" disabled={loading}
        className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50">
        {loading ? "Creating…" : "Create user"}
      </button>
    </form>
  );
}

function Pagination({ offset, limit, hasMore, onChange }: { offset: number; limit: number; hasMore: boolean; onChange: (o: number) => void }) {
  return (
    <div className="flex items-center gap-2">
      <button disabled={offset === 0} onClick={() => onChange(Math.max(0, offset - limit))}
        className="rounded border px-3 py-1 text-sm disabled:opacity-40 hover:bg-muted">← Prev</button>
      <span className="text-xs text-muted-foreground">Page {Math.floor(offset / limit) + 1}</span>
      <button disabled={!hasMore} onClick={() => onChange(offset + limit)}
        className="rounded border px-3 py-1 text-sm disabled:opacity-40 hover:bg-muted">Next →</button>
    </div>
  );
}
