"use client";

import { useQuery } from "@tanstack/react-query";
import {
  fetchObjectPreviewURLs,
  OBJECTS_QUERY_KEY,
  type ObjectAccessURLs,
} from "@/lib/api-client";

export function useObjectPreviewURLs(publicIds: string[]) {
  const stableKey = [...publicIds].sort().join(",");
  return useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "preview-urls", stableKey],
    queryFn: async () => {
      if (publicIds.length === 0) return {} as Record<string, ObjectAccessURLs>;
      const res = await fetchObjectPreviewURLs(publicIds);
      return res.urls;
    },
    enabled: publicIds.length > 0,
    staleTime: 10 * 60 * 1000,
  });
}
