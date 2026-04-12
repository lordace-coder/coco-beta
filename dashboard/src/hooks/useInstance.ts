import { useQuery } from "@tanstack/react-query";
import { instanceApi } from "@/api/client";

// Returns the single instance project ID.
// All components use this instead of receiving projectId as a prop.
export function useInstance() {
  const { data } = useQuery({
    queryKey: ["instance"],
    queryFn: () => instanceApi.get().then((r) => r.data),
    staleTime: Infinity, // instance ID never changes
  });
  return data?.id ?? "";
}
