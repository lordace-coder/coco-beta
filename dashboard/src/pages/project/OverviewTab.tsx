import { useState } from "react";
import { CopyButton } from "@/components/CopyButton";
import type { Project } from "@/api/client";

interface Props {
  project: Project;
  onRegen: () => void;
}

export function OverviewTab({ project, onRegen }: Props) {
  return (
    <div className="space-y-4">
      <div className="rounded-lg border bg-card divide-y">
        <Row label="Project ID">
          <span className="font-mono text-sm">{project.id}</span>
          <CopyButton value={project.id} />
        </Row>
        <Row label="API Key">
          <span className="font-mono text-sm truncate max-w-xs">{project.api_key}</span>
          <CopyButton value={project.api_key} />
          <button
            onClick={onRegen}
            className="rounded px-2 py-0.5 text-xs border border-destructive text-destructive hover:bg-destructive/10 transition-colors shrink-0"
          >
            Regenerate
          </button>
        </Row>
        <Row label="Status">
          <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${project.active ? "bg-green-100 text-green-700" : "bg-muted text-muted-foreground"}`}>
            {project.active ? "Active" : "Inactive"}
          </span>
        </Row>
        <Row label="Created">
          <span className="text-sm">{new Date(project.created_at).toLocaleString()}</span>
        </Row>
      </div>

      <SnippetCard project={project} />
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between px-4 py-3 gap-4">
      <span className="text-sm text-muted-foreground shrink-0 w-24">{label}</span>
      <div className="flex items-center gap-2 flex-1 justify-end flex-wrap">{children}</div>
    </div>
  );
}

function SnippetCard({ project }: { project: Project }) {
  const [lang, setLang] = useState<"js" | "python">("js");

  const jsSnippet = `import { Cocobase } from "cocobase";

const db = new Cocobase({
  apiKey: "${project.api_key}",
  baseURL: "${window.location.origin}"
});

await db.createDocument("users", {
  name: "John"
});`;

  const pySnippet = `from cocobase import Cocobase

db = Cocobase(
    api_key="${project.api_key}",
    base_url="${window.location.origin}"
)

db.create_document("users", {"name": "John"})`;

  const snippet = lang === "js" ? jsSnippet : pySnippet;

  return (
    <div className="rounded-lg border bg-card p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium">SDK Setup</p>
        <div className="flex gap-1">
          {(["js", "python"] as const).map((l) => (
            <button
              key={l}
              onClick={() => setLang(l)}
              className={`rounded px-2 py-0.5 text-xs border transition-colors ${lang === l ? "bg-primary text-primary-foreground border-primary" : "hover:bg-muted"}`}
            >
              {l === "js" ? "JavaScript" : "Python"}
            </button>
          ))}
        </div>
      </div>
      <pre className="rounded bg-muted px-4 py-3 text-xs font-mono overflow-x-auto leading-relaxed">{snippet}</pre>
      <CopyButton value={snippet} label="Copy snippet" />
    </div>
  );
}
