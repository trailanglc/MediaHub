export const PERMISSION_ACTIONS = [
  "read",
  "upload",
  "update",
  "delete",
  "convert",
  "share",
  "stream",
  "download",
  "manage",
] as const;

export type PermissionAction = (typeof PERMISSION_ACTIONS)[number];

const PERMISSION_LABELS: Record<string, string> = {
  read: "Xem",
  upload: "Tải lên",
  update: "Sửa",
  delete: "Xóa",
  convert: "Convert",
  share: "Chia sẻ",
  stream: "Stream",
  download: "Tải xuống",
  manage: "Quản lý",
};

export function permissionLabel(permission: string): string {
  return PERMISSION_LABELS[permission] ?? permission;
}
