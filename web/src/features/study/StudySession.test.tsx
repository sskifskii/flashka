import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { StudySessionRun } from "./StudySession";

afterEach(() => vi.unstubAllGlobals());

it("hides the back until reveal and answers by keyboard", async () => {
  const sessionId = crypto.randomUUID();
  const cardId = crypto.randomUUID();
  const base = {
    id: sessionId,
    actualCount: 1,
    answeredCount: 0,
    status: "active",
  };
  const fetchMock = vi
    .fn()
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          session: {
            ...base,
            current: {
              position: 1,
              cardId,
              front: "Вопрос",
              back: null,
              revealed: false,
            },
          },
        }),
        { status: 200 },
      ),
    )
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          session: {
            ...base,
            current: {
              position: 1,
              cardId,
              front: "Вопрос",
              back: "Ответ",
              revealed: true,
            },
          },
        }),
        { status: 200 },
      ),
    )
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          session: {
            ...base,
            answeredCount: 1,
            status: "completed",
            current: null,
          },
        }),
        { status: 200 },
      ),
    );
  vi.stubGlobal("fetch", fetchMock);
  render(
    <MemoryRouter initialEntries={[`/app/study/${sessionId}`]}>
      <Routes>
        <Route path="/app/study/:id" element={<StudySessionRun />} />
      </Routes>
    </MemoryRouter>,
  );

  expect(await screen.findByText("Вопрос")).toBeVisible();
  expect(screen.queryByText("Ответ")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Вспомнил →" }),
  ).not.toBeInTheDocument();

  await userEvent.click(screen.getByRole("button", { name: "Показать ответ" }));
  expect(await screen.findByText("Ответ")).toBeVisible();
  await userEvent.keyboard("{ArrowRight}");
  expect(await screen.findByText("Раунд завершён")).toBeVisible();

  expect(fetchMock).toHaveBeenNthCalledWith(
    2,
    `/api/v1/study/sessions/${sessionId}/cards/1/reveal`,
    expect.objectContaining({ method: "POST" }),
  );
  expect(fetchMock).toHaveBeenNthCalledWith(
    3,
    `/api/v1/study/sessions/${sessionId}/cards/1/answer`,
    expect.objectContaining({
      method: "PUT",
      body: expect.stringContaining('"rating":"correct"'),
    }),
  );
});

it("keeps the revealed card when saving an answer fails", async () => {
  const sessionId = crypto.randomUUID();
  const fetchMock = vi
    .fn()
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          session: {
            id: sessionId,
            actualCount: 2,
            answeredCount: 0,
            status: "active",
            current: {
              position: 1,
              cardId: crypto.randomUUID(),
              front: "Лицевая",
              back: "Обратная",
              revealed: true,
            },
          },
        }),
        { status: 200 },
      ),
    )
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: { code: "internal_error", message: "Ошибка" },
        }),
        { status: 500 },
      ),
    );
  vi.stubGlobal("fetch", fetchMock);
  render(
    <MemoryRouter initialEntries={[`/app/study/${sessionId}`]}>
      <Routes>
        <Route path="/app/study/:id" element={<StudySessionRun />} />
      </Routes>
    </MemoryRouter>,
  );
  expect(await screen.findByText("Обратная")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "← Не вспомнил" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Карточка осталась на месте",
  );
  expect(screen.getByText("Лицевая")).toBeVisible();
  expect(screen.getByText("Обратная")).toBeVisible();
});
