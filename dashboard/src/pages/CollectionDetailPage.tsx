import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { collectionsApi } from "@/api/client";

export default function CollectionDetailPage() {
  const { id: projectId, colId } = useParams<{ id: string; colId: string }>();
  const qc = useQueryClient();
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const { data: col } = useQuery({
    queryKey: ["collection", projectId, colId],
    queryFn: () => collectionsApi.get(projectId!, colId!).then((r) => r.data),
    enabled: !!projectId && !!colId,
  });

  const { data: docsData, isLoading } = useQuery({
    queryKey: ["documents", projectId, colId, offset],
    queryFn: () =>
      collectionsApi
        .listDocuments(projectId!, colId!, { limit, offset })
        .then((r) => r.data),
    enabled: !!projectId && !!colId,
  });

  const deleteMutation = useMutation({
    mutationFn: (docId: string) => collectionsApi.deleteDocument(projectId!, colId!, docId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["documents", projectId, colId] }),
  });

  const docs = docsData?.data ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/projects" className="hover:underline">Projects</Link>
        <span>/</span>
        <Link to={`/projects/${projectId}`} className="hover:underline">{projectId}</Link>
        <span>/</span>
        <span className="text-foreground">{col?.name ?? colId}</span>
      </div>

      <div>
        <h1 className="text-2xl font-bold">{col?.name}</h1>
        <p className="text-sm text-muted-foreground">
          {docsData?.total ?? 0} document{docsData?.total !== 1 ? "s" : ""}
        </p>
      </div>

      <div className="rounded-lg border bg-card">
        {isLoading ? (
          <div className="p-6 text-center text-sm text-muted-foreground">Loading…</div>
        ) : docs.length === 0 ? (
          <div className="p-6 text-center text-sm text-muted-foreground">No documents</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="px-4 py-3">ID</th>
                <th className="px-4 py-3">Data preview</th>
                <th className="px-4 py-3">Created</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {docs.map((doc) => (
                <tr key={doc.id} className="border-b last:border-0">
                  <td className="px-4 py-3 font-mono text-xs">{doc.id.slice(0, 8)}…</td>
                  <td className="max-w-xs truncate px-4 py-3 font-mono text-xs text-muted-foreground">
                    {JSON.stringify(doc.data).slice(0, 80)}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(doc.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => {
                        if (confirm("Delete this document?")) deleteMutation.mutate(doc.id);
                      }}
                      className="text-xs text-destructive hover:underline"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="flex gap-2">
        <button
          disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - limit))}
          className="rounded border px-3 py-1 text-sm disabled:opacity-40"
        >
          Previous
        </button>
        <button
          disabled={!docsData?.has_more}
          onClick={() => setOffset(offset + limit)}
          className="rounded border px-3 py-1 text-sm disabled:opacity-40"
        >
          Next
        </button>
      </div>
    </div>
  );
}
