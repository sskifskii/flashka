import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { StudyResult } from "./StudyResult";

afterEach(() => vi.unstubAllGlobals());

it("shows totals, errors and a repeat link for the same folder", async () => {
  const sessionId = crypto.randomUUID();
  const folderId = crypto.randomUUID();
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          result: {
            id: sessionId,
            scopeType: "folder",
            scopeFolderId: folderId,
            actualCount: 3,
            correctCount: 2,
            incorrectCount: 1,
            correctPercent: 67,
            errors: [
              { cardId: crypto.randomUUID(), front: "Вопрос", back: "Ответ" },
            ],
          },
        }),
        { status: 200 },
      ),
    ),
  );
  render(
    <MemoryRouter initialEntries={[`/app/study/${sessionId}/result`]}>
      <Routes>
        <Route path="/app/study/:id/result" element={<StudyResult />} />
      </Routes>
    </MemoryRouter>,
  );

  expect(
    await screen.findByRole("heading", { name: "Результат" }),
  ).toBeVisible();
  expect(screen.getByText("67%")).toBeVisible();
  expect(screen.getByText("Вопрос")).toBeVisible();
  expect(
    screen.getByRole("link", { name: "Повторить эту область" }),
  ).toHaveAttribute("href", `/app/study/new?folderId=${folderId}`);
});
