import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { logsApi } from "@/api/client";

const ACTION_COLORS: Record<string, string> = {
  create_user: "bg-green-100 text-green-700",
  delete_user: "bg-red-100 text-red-700",
  create_collection: "bg-blue-100 text-blue-700",
  delete_collection: "bg-red-100 text-red-700",
  create_document: "bg-green-100 text-green-700",
  delete_document: "bg-red-100 text-red-700",
  update_document: "bg-amber-100 text-amber-700",
};

const LIMIT = 30;

export function LogsTab({ projectId }: { projectId: string }) {
  const [offset, setOffset] = useState(0);

  const { data, isLoading, error } = useQuery({
    queryKey: ["logs", projectId, offset],
    queryFn: () => logsApi.list(projectId, { limit: LIMIT, offset }).then((r) => r.data),
    refetchInterval: 10_000,
  });

  const logs = data?.data ?? [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">Activity log · auto-refreshes every 10s</p>
        <span className="text-xs text-muted-foreground">Total: {data?.total ?? 0}</span>
      </div>

      <div className="rounded-lg border bg-card overflow-hidden">
        {isLoading ? (
          <p className="p-6 text-center text-sm text-muted-foreground">Loading…</p>
        ) : error ? (
          <p className="p-6 text-center text-sm text-destructive">
            Failed to load logs — {(error as { message?: string })?.message ?? "unknown error"}
          </p>
        ) : logs.length === 0 ? (
          <p className="p-6 text-center text-sm text-muted-foreground">No activity yet</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left text-muted-foreground text-xs">
                <th className="px-4 py-2">Action</th>
                <th className="px-4 py-2">Resource</th>
                <th className="px-4 py-2">Detail</th>
                <th className="px-4 py-2">When</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <tr key={log.id} className="border-b last:border-0 hover:bg-muted/20">
                  <td className="px-4 py-2.5">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${ACTION_COLORS[log.action] ?? "bg-muted text-muted-foreground"}`}>
                      {log.action.replace(/_/g, " ")}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-xs capitalize text-muted-foreground">{log.resource}</td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground max-w-xs truncate">{log.detail}</td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground whitespace-nowrap">
                    {new Date(log.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="flex items-center gap-2">
        <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - LIMIT))}
          className="rounded border px-3 py-1 text-sm disabled:opacity-40 hover:bg-muted">← Prev</button>
        <span className="text-xs text-muted-foreground">Page {Math.floor(offset / LIMIT) + 1}</span>
        <button disabled={!data?.has_more} onClick={() => setOffset(offset + LIMIT)}
          className="rounded border px-3 py-1 text-sm disabled:opacity-40 hover:bg-muted">Next →</button>
      </div>
    </div>
  );
}
