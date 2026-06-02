import { z } from "zod";

export const mediaObjectTypeSchema = z.enum([
  "folder",
  "file",
  "image",
  "video",
]);

export const createFolderSchema = z.object({
  name: z.string().trim().min(1, "Tên folder không được trống").max(255),
});

export const renameObjectSchema = z.object({
  name: z.string().trim().min(1, "Tên không được trống").max(255),
});

export const moveObjectSchema = z.object({
  parent_id: z.string().uuid("Chọn thư mục đích"),
});

export type CreateFolderForm = z.infer<typeof createFolderSchema>;
export type RenameObjectForm = z.infer<typeof renameObjectSchema>;
export type MoveObjectForm = z.infer<typeof moveObjectSchema>;
