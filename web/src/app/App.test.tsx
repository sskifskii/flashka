import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../features/auth/AuthProvider";
import { App } from "./App";

function renderApp(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <AuthProvider>
          <App />
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => vi.unstubAllGlobals());

function errorResponse(status: number) {
  return new Response(
    JSON.stringify({
      error: {
        code: status === 401 ? "unauthenticated" : "internal_error",
        message: status === 401 ? "Требуется вход" : "Внутренняя ошибка",
        requestId: crypto.randomUUID(),
      },
    }),
    { status, headers: { "Content-Type": "application/json" } },
  );
}

function userResponse() {
  return new Response(
    JSON.stringify({ user: { id: crypto.randomUUID(), login: "Alex" } }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

describe("authentication routes", () => {
  it("shows the recovery warning and inline client validation", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(errorResponse(401)));
    renderApp("/register");
    expect(
      await screen.findByText(/восстановить забытый пароль/),
    ).toBeVisible();
    await userEvent.click(
      screen.getByRole("button", { name: "Создать аккаунт" }),
    );
    expect(await screen.findByText(/Логин должен содержать/)).toBeVisible();
    expect(screen.getByText(/Пароль должен содержать/)).toBeVisible();
  });

  it("does not reveal protected content while the session is loading", () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => undefined)),
    );
    renderApp("/app");
    expect(screen.getByText("Проверяем сессию…")).toBeVisible();
    expect(
      screen.queryByRole("heading", { name: "Карточки" }),
    ).not.toBeInTheDocument();
  });

  it("redirects an explicit guest to login", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(errorResponse(401)));
    renderApp("/app");
    expect(
      await screen.findByRole("heading", { name: "С возвращением" }),
    ).toBeVisible();
  });

  it.each([
    ["network", () => Promise.reject(new TypeError("offline"))],
    ["server", () => Promise.resolve(errorResponse(500))],
  ])(
    "shows a retry state for a %s bootstrap failure",
    async (_name, response) => {
      vi.stubGlobal("fetch", vi.fn(response));
      renderApp("/app");
      expect(
        await screen.findByText("Не удалось проверить сессию"),
      ).toBeVisible();
      expect(
        screen.queryByRole("heading", { name: "С возвращением" }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("heading", { name: "Карточки" }),
      ).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Повторить" })).toBeEnabled();
    },
  );

  it("retries bootstrap and redirects only after an explicit 401", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(errorResponse(500))
      .mockResolvedValueOnce(errorResponse(401));
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/app");
    await userEvent.click(
      await screen.findByRole("button", { name: "Повторить" }),
    );
    expect(
      await screen.findByRole("heading", { name: "С возвращением" }),
    ).toBeVisible();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("keeps the shell, blocks double click and does not broadcast when logout fails", async () => {
    let finishLogout!: (response: Response) => void;
    const logoutResponse = new Promise<Response>((resolve) => {
      finishLogout = resolve;
    });
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).endsWith("/auth/me"))
        return Promise.resolve(userResponse());
      if (String(input).endsWith("/cards") && !init?.method)
        return Promise.resolve(
          new Response(JSON.stringify({ items: [], nextCursor: null }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      if (String(input).endsWith("/auth/logout") && init?.method === "POST")
        return logoutResponse;
      throw new Error(`unexpected request ${String(input)}`);
    });
    const postMessage = vi.fn();
    class TestChannel {
      postMessage = postMessage;
      addEventListener() {}
      close() {}
    }
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("BroadcastChannel", TestChannel);
    renderApp("/app");
    expect(
      await screen.findByRole("heading", { name: "Карточки" }),
    ).toBeVisible();
    const button = screen.getByRole("button", { name: "Выйти" });
    const click = userEvent.click(button);
    expect(
      await screen.findByRole("button", { name: "Выходим…" }),
    ).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Выходим…" }));
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(screen.getByRole("heading", { name: "Карточки" })).toBeVisible();
    expect(postMessage).not.toHaveBeenCalled();
    finishLogout(errorResponse(500));
    await click;
    expect(await screen.findByText(/Не удалось выйти/)).toBeVisible();
    expect(screen.getByRole("heading", { name: "Карточки" })).toBeVisible();
    expect(postMessage).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("broadcasts and navigates only after logout returns 204", async () => {
    let finishLogout!: (response: Response) => void;
    const logoutResponse = new Promise<Response>((resolve) => {
      finishLogout = resolve;
    });
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).endsWith("/auth/me"))
        return Promise.resolve(userResponse());
      if (String(input).endsWith("/cards") && !init?.method)
        return Promise.resolve(
          new Response(JSON.stringify({ items: [], nextCursor: null }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      if (String(input).endsWith("/auth/logout") && init?.method === "POST")
        return logoutResponse;
      throw new Error(`unexpected request ${String(input)}`);
    });
    const postMessage = vi.fn();
    class TestChannel {
      postMessage = postMessage;
      addEventListener() {}
      close() {}
    }
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("BroadcastChannel", TestChannel);
    renderApp("/app");
    expect(
      await screen.findByRole("heading", { name: "Карточки" }),
    ).toBeVisible();
    const click = userEvent.click(
      screen.getByRole("button", { name: "Выйти" }),
    );
    expect(
      await screen.findByRole("button", { name: "Выходим…" }),
    ).toBeDisabled();
    expect(screen.getByRole("heading", { name: "Карточки" })).toBeVisible();
    expect(postMessage).not.toHaveBeenCalled();
    finishLogout(new Response(null, { status: 204 }));
    await click;
    expect(
      await screen.findByRole("heading", { name: "С возвращением" }),
    ).toBeVisible();
    expect(postMessage).toHaveBeenCalledOnce();
  });
});
