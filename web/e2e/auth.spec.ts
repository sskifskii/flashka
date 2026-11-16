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
  await expect(page.getByRole("heading", { name: "Карточки" })).toBeVisible();
  await page.getByRole("link", { name: "Добавить карточку" }).first().click();
  await page
    .getByRole("textbox", { name: "Front", exact: true })
    .fill("  <script>front</script>  ");
  await page.getByRole("button", { name: "Развернуть на Back" }).click();
  await page
    .getByRole("textbox", { name: "Back", exact: true })
    .fill("back\nline");
  await page.getByRole("button", { name: "Сохранить" }).click();
  await expect(page.getByText("Карточка сохранена")).toBeVisible();
  await page.getByRole("link", { name: "Открыть карточку" }).click();
  await expect(
    page.getByText("<script>front</script>", { exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(page.getByText("back\nline", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Редактировать" }).click();
  await page
    .getByRole("textbox", { name: "Front", exact: true })
    .fill("changed front");
  await page.getByRole("button", { name: "Сохранить" }).click();
  await expect(page.getByText("changed front", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Удалить" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: "Отмена" }).click();
  await expect(page.getByText("changed front", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Удалить" }).click();
  await page.getByRole("button", { name: "Удалить навсегда" }).click();
  await expect(page).toHaveURL(/\/app$/);
  await expect(page.getByText("Создайте первую карточку")).toBeVisible();
  await expect(page.locator("body")).toHaveJSProperty(
    "scrollWidth",
    await page.locator("body").evaluate((el) => el.clientWidth),
  );
  const otherPage = await context.newPage();
  await otherPage.goto("/app");
  await expect(
    otherPage.getByRole("heading", { name: "Карточки" }),
  ).toBeVisible();
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
  await expect(
    otherPage.getByRole("heading", { name: "Карточки" }),
  ).toBeVisible();
  await page.bringToFront();
  await page.getByRole("button", { name: "Выйти" }).click();
  await otherPage.bringToFront();
  await otherPage.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect(otherPage).toHaveURL(/\/login$/);
});

test("manages nested folders and moves a card", async ({ page }) => {
  const login = `User${crypto.randomUUID().replaceAll("-", "").slice(0, 20)}`;
  await page.goto("/register");
  await page.getByLabel("Логин").fill(login);
  await page.getByLabel("Пароль").fill("exact password");
  await page.getByRole("button", { name: "Создать аккаунт" }).click();
  await expect(page).toHaveURL(/\/app$/);
  await expect(page.getByRole("heading", { name: "Карточки" })).toBeVisible();
  const toggle = page.getByRole("button", { name: "Показать папки" });
  if (await toggle.isVisible()) await toggle.click();

  await page.getByRole("button", { name: "Создать внутри" }).click();
  await page.getByLabel("Название").fill("Topic");
  await page.getByRole("button", { name: "Сохранить" }).click();
  await expect(page).toHaveURL(/folderId=/);
  await page.getByRole("button", { name: "Создать внутри" }).click();
  await page.getByLabel("Название").fill("Child");
  await page.getByRole("button", { name: "Сохранить" }).click();
  await expect(page.getByLabel("Название")).toHaveCount(0);
  const closeFolders = page.getByRole("button", { name: "Скрыть папки" });
  if (await closeFolders.isVisible()) await closeFolders.click();

  await page.getByRole("link", { name: "Добавить карточку" }).first().click();
  await expect(page.getByLabel("Папка")).toHaveValue(/.+/);
  await page
    .getByRole("textbox", { name: "Front", exact: true })
    .fill("folder front");
  await page.getByRole("button", { name: "Развернуть на Back" }).click();
  await page
    .getByRole("textbox", { name: "Back", exact: true })
    .fill("folder back");
  await page.getByRole("button", { name: "Сохранить" }).click();
  await page.getByRole("link", { name: "Открыть карточку" }).click();
  await page.getByRole("button", { name: "Редактировать" }).click();
  await page.getByLabel("Папка").selectOption({ label: "Inbox" });
  await page.getByRole("button", { name: "Сохранить" }).click();
  await expect(
    page.getByRole("button", { name: "Редактировать" }),
  ).toBeVisible();
  await page.goto("/app");
  await expect(page.getByRole("heading", { name: "Карточки" })).toBeVisible();
  if (await toggle.isVisible()) await toggle.click();
  await page.getByRole("button", { name: /^Child \d+ \/ \d+$/ }).click();
  if (await toggle.isVisible()) await toggle.click();
  await page.getByRole("button", { name: "Удалить" }).click();
  await page.getByRole("button", { name: "Удалить папку" }).click();
  await expect(
    page.getByRole("button", { name: /^Child \d+ \/ \d+$/ }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: /^Topic \d+ \/ \d+$/ }).click();
  if (await toggle.isVisible()) await toggle.click();
  await page.getByRole("button", { name: "Удалить" }).click();
  await page.getByRole("button", { name: "Удалить папку" }).click();
  await expect(
    page.getByRole("button", { name: /^Topic \d+ \/ \d+$/ }),
  ).toHaveCount(0);
  await expect(page.locator("body")).toHaveJSProperty(
    "scrollWidth",
    await page.locator("body").evaluate((el) => el.clientWidth),
  );
});
