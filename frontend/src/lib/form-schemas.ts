import { z } from "zod";

export const emailSchema = z.string().email("Email không hợp lệ");

export const passwordSchema = z
  .string()
  .min(8, "Mật khẩu tối thiểu 8 ký tự")
  .regex(/[a-zA-Z]/, "Cần ít nhất một chữ cái")
  .regex(/[0-9]/, "Cần ít nhất một chữ số");

export const memberRoleSchema = z.enum(["manager", "viewer"]);
