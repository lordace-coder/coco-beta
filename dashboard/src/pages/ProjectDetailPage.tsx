import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projectsApi } from "@/api/client";
import { OverviewTab } from "./project/OverviewTab";
import { ConfigTab } from "./project/ConfigTab";
import { UsersTab } from "./project/UsersTab";
import { CollectionsTab } from "./project/CollectionsTab";
import { FilesTab } from "./project/FilesTab";
import { LogsTab } from "./project/LogsTab";
import { FunctionsTab } from "./project/FunctionsTab";

type Tab = "overview" | "settings" | "users" | "collections" | "files" | "logs" | "functions";

const TABS: { key: Tab; label: string }[] = [
  { key: "overview", label: "Overview" },
  { key: "settings", label: "Settings" },
  { key: "users", label: "Users" },
  { key: "collections", label: "Collections" },
  { key: "files", label: "Files" },
  { key: "functions", label: "Functions" },
  { key: "logs", label: "Logs" },
];

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

  return (
    <div className="space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/projects" className="hover:underline">Projects</Link>
        <span>/</span>
        <span className="text-foreground font-medium">{project.name}</span>
      </div>

      {/* Header */}
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-bold">{project.name}</h1>
        <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${project.active ? "bg-green-100 text-green-700" : "bg-muted text-muted-foreground"}`}>
          {project.active ? "active" : "inactive"}
        </span>
      </div>

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
      {tab === "users" && <UsersTab projectId={id!} />}
      {tab === "collections" && <CollectionsTab projectId={id!} />}
      {tab === "files" && <FilesTab projectId={id!} />}
      {tab === "functions" && <FunctionsTab projectId={id!} />}
      {tab === "logs" && <LogsTab projectId={id!} />}
    </div>
  );
}
