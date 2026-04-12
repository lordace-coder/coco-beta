import { useQuery } from "@tanstack/react-query";
import { healthApi } from "@/api/client";

export default function OverviewPage() {
  const { data: health } = useQuery({
    queryKey: ["health"],
    queryFn: () => healthApi.check().then((r) => r.data),
    refetchInterval: 30_000,
  });

  const dbStatus = (health as { services?: { database?: { status?: string } } })?.services?.database?.status ?? "—";

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">System Health</h1>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          label="Status"
          value={(health as { status?: string })?.status ?? "—"}
          highlight={(health as { status?: string })?.status === "ok" ? "green" : "red"}
        />
        <StatCard
          label="Database"
          value={dbStatus}
          highlight={dbStatus === "ok" ? "green" : "red"}
        />
        <StatCard
          label="Memory"
          value={`${(health as { memory?: { alloc_mb?: number } })?.memory?.alloc_mb ?? 0} MB`}
        />
      </div>

      {health && (
        <div className="rounded-lg border bg-card p-4">
          <h2 className="mb-3 font-semibold">Details</h2>
          <div className="grid gap-2 text-sm sm:grid-cols-2">
            <Row label="Uptime" value={(health as { uptime?: string })?.uptime ?? "—"} />
            <Row label="Redis" value={(health as { services?: { redis?: { status?: string } } })?.services?.redis?.status ?? "—"} />
          </div>
        </div>
      )}
    </div>
  );
}

function StatCard({ label, value, highlight }: { label: string; value: string | number; highlight?: string }) {
  return (
    <div className="rounded-lg border bg-card p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p
        className={`mt-1 text-2xl font-bold ${highlight === "green" ? "text-green-600" : highlight === "red" ? "text-destructive" : ""}`}
      >
        {value}
      </p>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  );
}
