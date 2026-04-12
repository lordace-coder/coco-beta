import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { logsApi } from "@/api/client";
import { useInstance } from "@/hooks/useInstance";

const LIMIT = 100;

export function LogsTab() {
  const projectId = useInstance();
  const [page, setPage] = useState(0);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["logs", projectId, page],
    queryFn: () =>
      logsApi.list(projectId, { limit: LIMIT }).then((r) => r.data),
    refetchInterval: 10_000,
  });

  const lines: string[] = (data as { data?: string[]; total?: number })?.data ?? [];
  const total: number = (data as { data?: string[]; total?: number })?.total ?? 0;

  function levelClass(line: string) {
    if (line.includes("[ERROR]") || line.includes("[FATAL]")) return "text-red-500";
    if (line.includes("[WARN]")) return "text-amber-500";
    if (line.includes("[INFO]")) return "text-green-600 dark:text-green-400";
    return "text-foreground/80";
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Server log</p>
          <p className="text-xs text-muted-foreground">
            Showing last {lines.length} of {total} lines · auto-refreshes every 10s
          </p>
        </div>
        <button
          onClick={() => refetch()}
          className="rounded border px-3 py-1 text-xs hover:bg-muted"
        >
          ↻ Refresh
        </button>
      </div>

      <div className="rounded-lg border bg-zinc-950 dark:bg-black overflow-hidden">
        {isLoading ? (
          <p className="p-6 text-center text-sm text-muted-foreground">Loading…</p>
        ) : error ? (
          <p className="p-6 text-center text-sm text-red-500">
            Failed to load logs —{" "}
            {(error as { message?: string })?.message ?? "unknown error"}
          </p>
        ) : lines.length === 0 ? (
          <p className="p-6 text-center text-sm text-muted-foreground">
            No log entries yet. Logs are written to <code className="font-mono">cocobase.log</code>.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <pre className="p-4 text-[11px] font-mono leading-relaxed space-y-px">
              {lines.map((line, i) => (
                <div key={i} className={levelClass(line)}>
                  {line}
                </div>
              ))}
            </pre>
          </div>
        )}
      </div>
    </div>
  );
}
