import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projectsApi } from "@/api/client";
import { useInstance } from "@/hooks/useInstance";
import { OverviewTab } from "./project/OverviewTab";
import { ConfigTab } from "./project/ConfigTab";
import { UsersTab } from "./project/UsersTab";
import { CollectionsTab } from "./project/CollectionsTab";
import { FilesTab } from "./project/FilesTab";
import { LogsTab } from "./project/LogsTab";
import { FunctionsTab } from "./project/FunctionsTab";

type Tab = "overview" | "settings" | "users" | "collections" | "files" | "logs" | "functions";

const TABS: { key: Tab; label: string }[] = [
  { key: "overview", label: "API" },
  { key: "collections", label: "Collections" },
  { key: "users", label: "Users" },
  { key: "functions", label: "Functions" },
  { key: "files", label: "Files" },
  { key: "logs", label: "Logs" },
  { key: "settings", label: "Settings" },
];

export default function ProjectDetailPage() {
  const projectId = useInstance();
  const [tab, setTab] = useState<Tab>("overview");
  const qc = useQueryClient();

  const { data: project, isLoading } = useQuery({
    queryKey: ["project", projectId],
    queryFn: () => projectsApi.get(projectId).then((r) => r.data),
    enabled: !!projectId,
  });

  const regenMutation = useMutation({
    mutationFn: () => projectsApi.regenKey(projectId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["project", projectId] }),
  });

  if (isLoading || !projectId) return <div className="p-6 text-sm text-muted-foreground">Loading…</div>;
  if (!project) return <div className="p-6 text-sm text-muted-foreground">Loading instance…</div>;

  return (
    <div className="space-y-6">
      {/* Tab bar */}
      <div className="flex gap-1 border-b overflow-x-auto">
        {TABS.map((t) => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`shrink-0 px-4 py-2 text-sm font-medium transition-colors ${tab === t.key ? "border-b-2 border-primary text-primary" : "text-muted-foreground hover:text-foreground"}`}>
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {tab === "overview" && (
        <OverviewTab project={project} onRegen={() => {
          if (confirm("Regenerate API key? The current key will stop working immediately.")) {
            regenMutation.mutate();
          }
        }} />
      )}
      {tab === "settings" && <ConfigTab project={project} />}
      {tab === "users" && <UsersTab />}
      {tab === "collections" && <CollectionsTab />}
      {tab === "files" && <FilesTab />}
      {tab === "functions" && <FunctionsTab />}
      {tab === "logs" && <LogsTab />}
    </div>
  );
}
