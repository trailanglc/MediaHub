import { test, expect } from "@playwright/test";

const email = process.env.E2E_EMAIL;
const password = process.env.E2E_PASSWORD;

test.describe("File Manager", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(!email || !password, "Cần E2E_EMAIL và E2E_PASSWORD");

    await page.goto("/login");
    await page.getByLabel("Email").fill(email!);
    await page.getByLabel("Mật khẩu").fill(password!);
    await page.getByRole("button", { name: "Đăng nhập" }).click();
    await page.waitForURL(/\/(dashboard|files)/);
  });

  test("mở /files và chuyển thùng rác", async ({ page }) => {
    await page.goto("/files");
    await expect(page.getByRole("heading", { name: "File Manager" })).toBeVisible();

    await page.getByRole("button", { name: "Thùng rác" }).click();
    await expect(page).toHaveURL(/trash=1/);
    await expect(page.getByRole("heading", { name: "Thùng rác" })).toBeVisible();

    await page.getByRole("button", { name: "Quay lại file" }).click();
    await expect(page.getByRole("heading", { name: "File Manager" })).toBeVisible();
  });

  test("tìm kiếm debounce cập nhật URL", async ({ page }) => {
    await page.goto("/files");
    const search = page.getByPlaceholder(/Tìm trong thư mục/);
    await search.fill("e2e-debounce-test");
    await expect(page).toHaveURL(/q=e2e-debounce-test/, { timeout: 5000 });
  });

  test("tạo folder mới", async ({ page }) => {
    await page.goto("/files");
    const folderName = `e2e-folder-${Date.now()}`;
    await page.getByRole("button", { name: "Folder mới" }).click();
    await page.getByRole("dialog").getByRole("textbox").fill(folderName);
    await page.getByRole("button", { name: "Tạo" }).click();
    await expect(page.getByText(folderName)).toBeVisible({ timeout: 10_000 });
  });
});
