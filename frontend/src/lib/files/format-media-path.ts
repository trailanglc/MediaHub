import type { BreadcrumbItem } from "@/lib/api/api-client";

/** Folder path label for global search results (excludes the object itself). */
export function formatMediaPath(
  breadcrumbs: BreadcrumbItem[] | undefined,
  rootFolderId: string,
): string {
  if (!breadcrumbs?.length) return "—";
  const unique = breadcrumbs.filter((item, index, arr) => {
    const first = arr.findIndex((x) => x.public_id === item.public_id);
    return first === index;
  });
  const folders = unique.filter(
    (item) => item.type === "folder" && item.public_id !== rootFolderId,
  );
  if (folders.length === 0) return "/";
  return folders.map((f) => f.name).join(" / ");
}

export function parentFolderFromObject(
  obj: { parent_public_id: string | null },
  rootFolderId: string,
): string {
  return obj.parent_public_id ?? rootFolderId;
}
