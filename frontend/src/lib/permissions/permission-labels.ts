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
