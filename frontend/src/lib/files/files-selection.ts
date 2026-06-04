import type { MediaObject } from "@/lib/api/api-client";

export function isSelectableObject(
  obj: MediaObject,
  rootFolderId: string,
): boolean {
  return obj.public_id !== rootFolderId;
}

export function selectableItems(
  items: MediaObject[],
  rootFolderId: string,
): MediaObject[] {
  return items.filter((o) => isSelectableObject(o, rootFolderId));
}

export function selectedObjects(
  items: MediaObject[],
  selectedIds: ReadonlySet<string>,
): MediaObject[] {
  return items.filter((o) => selectedIds.has(o.public_id));
}

export function canBulkDelete(objects: MediaObject[]): boolean {
  return objects.length > 0 && objects.every((o) => o.capabilities.delete);
}

export function canBulkMove(objects: MediaObject[]): boolean {
  return objects.length > 0 && objects.every((o) => o.capabilities.update);
}

export function canBulkRename(objects: MediaObject[]): boolean {
  return (
    objects.length > 0 &&
    objects.length <= 20 &&
    objects.every((o) => o.capabilities.update)
  );
}

export function canBulkRestore(objects: MediaObject[]): boolean {
  return (
    objects.length > 0 &&
    objects.length <= 20 &&
    objects.every((o) => o.capabilities.update)
  );
}

export function canBulkPurge(objects: MediaObject[]): boolean {
  return (
    objects.length > 0 &&
    objects.length <= 20 &&
    objects.every((o) => o.capabilities.delete)
  );
}
