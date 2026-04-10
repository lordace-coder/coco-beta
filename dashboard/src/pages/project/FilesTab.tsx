import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { filesApi } from "@/api/client";

export function FilesTab({ projectId }: { projectId: string }) {
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
    <div className="rounded-lg border bg-card overflow-hidden">
      {isLoading ? (
        <p className="p-6 text-center text-sm text-muted-foreground">Loading…</p>
      ) : files.length === 0 ? (
        <p className="p-6 text-center text-sm text-muted-foreground">No files uploaded yet</p>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40 text-left text-muted-foreground text-xs">
              <th className="px-4 py-2">File</th>
              <th className="px-4 py-2">Size</th>
              <th className="px-4 py-2">Modified</th>
              <th className="px-4 py-2"></th>
            </tr>
          </thead>
          <tbody>
            {files.map((f) => (
              <tr key={f.key} className="border-b last:border-0 hover:bg-muted/20">
                <td className="px-4 py-2.5">
                  <a href={f.url} target="_blank" rel="noopener noreferrer"
                    className="font-mono text-xs text-primary hover:underline">
                    {f.key.split("/").pop()}
                  </a>
                  <p className="text-xs text-muted-foreground font-mono">{f.key}</p>
                </td>
                <td className="px-4 py-2.5 text-xs text-muted-foreground">{formatBytes(f.size)}</td>
                <td className="px-4 py-2.5 text-xs text-muted-foreground">{new Date(f.last_modified).toLocaleDateString()}</td>
                <td className="px-4 py-2.5 text-right">
                  <button
                    onClick={() => { if (confirm(`Delete "${f.key}"?`)) deleteMutation.mutate(f.key); }}
                    className="text-xs text-destructive hover:underline"
                  >Delete</button>
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
