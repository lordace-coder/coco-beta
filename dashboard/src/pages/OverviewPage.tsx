import { useQuery } from "@tanstack/react-query";
import { projectsApi, healthApi } from "@/api/client";
import { useAuthStore } from "@/hooks/useAuth";

export default function OverviewPage() {
  const admin = useAuthStore((s) => s.admin);

  const { data: projectsData } = useQuery({
    queryKey: ["projects"],
    queryFn: () => projectsApi.list().then((r) => r.data),
  });

  const { data: health } = useQuery({
    queryKey: ["health"],
    queryFn: () => healthApi.check().then((r) => r.data),
    refetchInterval: 30_000,
  });

  const projects = projectsData?.data ?? [];
  const activeProjects = projects.filter((p) => p.active).length;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Overview</h1>
        <p className="text-muted-foreground">
          Welcome back{admin?.email ? `, ${admin.email}` : ""}
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard label="Total Projects" value={projects.length} />
        <StatCard label="Active Projects" value={activeProjects} />
        <StatCard
          label="Database"
          value={(health as { services?: { database?: { status?: string } } })?.services?.database?.status ?? "—"}
          highlight={(health as { services?: { database?: { status?: string } } })?.services?.database?.status === "ok" ? "green" : "red"}
        />
      </div>

      {/* System health */}
      {health && (
        <div className="rounded-lg border bg-card p-4">
          <h2 className="mb-3 font-semibold">System Health</h2>
          <div className="grid gap-2 text-sm sm:grid-cols-2">
            <Row label="Status" value={(health as { status?: string })?.status ?? "—"} />
            <Row label="Uptime" value={(health as { uptime?: string })?.uptime ?? "—"} />
            <Row
              label="Redis"
              value={
                (health as { services?: { redis?: { status?: string } } })?.services?.redis?.status ?? "—"
              }
            />
            <Row
              label="Memory (alloc)"
              value={
                `${(health as { memory?: { alloc_mb?: number } })?.memory?.alloc_mb ?? 0} MB`
              }
            />
          </div>
        </div>
      )}

      {/* Recent projects */}
      <div className="rounded-lg border bg-card">
        <div className="border-b px-4 py-3 font-semibold">Recent Projects</div>
        {projects.length === 0 ? (
          <div className="p-6 text-center text-sm text-muted-foreground">No projects yet</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">Status</th>
                <th className="px-4 py-2">Created</th>
              </tr>
            </thead>
            <tbody>
              {projects.slice(0, 5).map((p) => (
                <tr key={p.id} className="border-b last:border-0">
                  <td className="px-4 py-2 font-medium">{p.name}</td>
                  <td className="px-4 py-2">
                    <span
                      className={`rounded px-2 py-0.5 text-xs font-medium ${p.active ? "bg-green-100 text-green-700" : "bg-muted text-muted-foreground"}`}
                    >
                      {p.active ? "active" : "inactive"}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">
                    {new Date(p.created_at).toLocaleDateString()}
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
