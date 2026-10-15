import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { StudySetup } from "./StudySetup";

afterEach(() => vi.unstubAllGlobals());

it("shows preview, caps actual count and starts the fixed session", async () => {
  const root = {
    id: crypto.randomUUID(),
    parentId: null,
    name: "Inbox",
    isInbox: true,
    directCardCount: 3,
    totalCardCount: 3,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    children: [],
  };
  const sessionId = crypto.randomUUID();
  const fetchMock = vi
    .fn()
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ root }), { status: 200 }),
    )
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          preview: { available: 3, due: 1, new: 2, mastered: 0 },
        }),
        { status: 200 },
      ),
    )
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ session: { id: sessionId } }), {
        status: 201,
      }),
    );
  vi.stubGlobal("fetch", fetchMock);
  render(
    <MemoryRouter initialEntries={["/app/study/new"]}>
      <Routes>
        <Route path="/app/study/new" element={<StudySetup />} />
        <Route path="/app/study/:id" element={<div>Раунд открыт</div>} />
      </Routes>
    </MemoryRouter>,
  );
  expect(await screen.findByText("Доступно: 3")).toBeVisible();
  expect(screen.getByText("В раунде будет: 3")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Начать 3" }));
  expect(await screen.findByText("Раунд открыт")).toBeVisible();
  expect(fetchMock).toHaveBeenLastCalledWith(
    "/api/v1/study/sessions",
    expect.objectContaining({
      method: "POST",
      body: JSON.stringify({ scopeType: "all", count: 10 }),
    }),
  );
});
