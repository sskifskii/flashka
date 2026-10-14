import { test, expect } from "@playwright/test";
test("register reload cross-tab logout and login", async ({
  page,
  context,
}) => {
  const login = `User${crypto.randomUUID().replaceAll("-", "").slice(0, 20)}`;
  const password = "exact password";
  await page.goto("/register");
  await expect(page.getByText("восстановить забытый пароль")).toBeVisible();
  await page.getByLabel("Логин").fill(login);
  await page.getByLabel("Пароль").fill(password);
  await page.getByRole("button", { name: "Создать аккаунт" }).click();
  await expect(page).toHaveURL(/\/app$/);
  await page.reload();
  await expect(page.getByText(`Добро пожаловать, ${login}`)).toBeVisible();
  await expect(page.locator("body")).toHaveJSProperty(
    "scrollWidth",
    await page.locator("body").evaluate((el) => el.clientWidth),
  );
  const otherPage = await context.newPage();
  await otherPage.goto("/app");
  await expect(otherPage.getByText(`Добро пожаловать, ${login}`)).toBeVisible();
  await page.getByRole("button", { name: "Выйти" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await expect(otherPage).toHaveURL(/\/login$/);
  await page.getByLabel("Логин").fill(login.toLowerCase());
  await page.getByLabel("Пароль").fill(password);
  await page.getByRole("button", { name: "Войти" }).click();
  await expect(page).toHaveURL(/\/app$/);
});

test("focus fallback notices a logout when BroadcastChannel is unavailable", async ({
  page,
  context,
}) => {
  await context.addInitScript(() => {
    delete (window as { BroadcastChannel?: typeof BroadcastChannel })
      .BroadcastChannel;
  });
  const login = `User${crypto.randomUUID().replaceAll("-", "").slice(0, 20)}`;
  await page.goto("/register");
  await page.getByLabel("Логин").fill(login);
  await page.getByLabel("Пароль").fill("exact password");
  await page.getByRole("button", { name: "Создать аккаунт" }).click();
  await expect(page).toHaveURL(/\/app$/);

  const otherPage = await context.newPage();
  await otherPage.goto("/app");
  await expect(otherPage.getByText(`Добро пожаловать, ${login}`)).toBeVisible();
  await page.bringToFront();
  await page.getByRole("button", { name: "Выйти" }).click();
  await otherPage.bringToFront();
  await otherPage.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect(otherPage).toHaveURL(/\/login$/);
});
