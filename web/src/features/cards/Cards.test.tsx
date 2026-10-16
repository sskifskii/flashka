import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CardDetail, CardList, NewCard } from "./Cards";

afterEach(() => vi.unstubAllGlobals());

describe("cards UI", () => {
  const summary = (id: string, front: string) => ({
    id,
    front,
    back: `${front} back`,
    status: "new" as const,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  });
  const listResponse = (
    items: ReturnType<typeof summary>[],
    nextCursor: string | null,
  ) =>
    new Response(JSON.stringify({ items, nextCursor }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  const cardResponse = () =>
    new Response(
      JSON.stringify({
        card: {
          ...summary("00000000-0000-4000-8000-000000000001", "front"),
          folderId: "00000000-0000-4000-8000-000000000002",
          correctCount: 0,
          incorrectCount: 0,
          correctStreak: 0,
          intervalDays: null,
          lastReviewedAt: null,
          dueAt: null,
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  const treeResponse = () =>
    new Response(
      JSON.stringify({
        root: {
          id: "00000000-0000-4000-8000-000000000002",
          parentId: null,
          name: "Inbox",
          isInbox: true,
          directCardCount: 0,
          totalCardCount: 0,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
          children: [],
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  const withTree = (
    handler: (
      input: RequestInfo | URL,
      init?: RequestInit,
    ) => Promise<Response>,
  ) =>
    vi.fn((input: RequestInfo | URL, init?: RequestInit) =>
      String(input).endsWith("/folders/tree")
        ? Promise.resolve(treeResponse())
        : handler(input, init),
    );

  it("moves from the list skeleton to the empty state", async () => {
    let finish!: (response: Response) => void;
    vi.stubGlobal(
      "fetch",
      withTree(
        () =>
          new Promise<Response>((resolve) => {
            finish = resolve;
          }),
      ),
    );
    render(
      <MemoryRouter>
        <CardList />
      </MemoryRouter>,
    );
    expect(screen.getByLabelText("Загрузка карточек")).toBeVisible();
    await act(async () => finish(listResponse([], null)));
    expect(await screen.findByText("Создайте первую карточку")).toBeVisible();
    expect(
      screen.queryByLabelText("Загрузка карточек"),
    ).not.toBeInTheDocument();
  });

  it("shows the selected folder path and both card sides", async () => {
    const rootId = "00000000-0000-4000-8000-000000000002";
    const childId = "00000000-0000-4000-8000-000000000003";
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        if (String(input).endsWith("/folders/tree")) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                root: {
                  id: rootId,
                  parentId: null,
                  name: "Inbox",
                  isInbox: true,
                  directCardCount: 0,
                  totalCardCount: 1,
                  createdAt: new Date().toISOString(),
                  updatedAt: new Date().toISOString(),
                  children: [
                    {
                      id: childId,
                      parentId: rootId,
                      name: "Языки",
                      isInbox: false,
                      directCardCount: 1,
                      totalCardCount: 1,
                      createdAt: new Date().toISOString(),
                      updatedAt: new Date().toISOString(),
                      children: [],
                    },
                  ],
                },
              }),
              { status: 200, headers: { "Content-Type": "application/json" } },
            ),
          );
        }
        return Promise.resolve(
          listResponse(
            [summary("00000000-0000-4000-8000-000000000001", "bonjour")],
            null,
          ),
        );
      }),
    );

    render(
      <MemoryRouter initialEntries={[`/app?folderId=${childId}`]}>
        <CardList />
      </MemoryRouter>,
    );

    expect(await screen.findByText("bonjour back")).toBeVisible();
    const heading = screen.getByRole("heading", { name: "Карточки" });
    expect(heading).toBeVisible();
    expect(screen.getByRole("link", { name: "Inbox" })).toHaveAttribute(
      "href",
      "/app",
    );
    expect(within(heading).getByText("Языки")).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: /Учить/ })).toHaveAttribute(
      "href",
      `/app/study/new?folderId=${childId}`,
    );
    expect(screen.queryByText(/Коллекция.*Карточки/)).not.toBeInTheDocument();
  });

  it("retries an initial list error and shows the successful result", async () => {
    const cardsMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockResolvedValueOnce(
        listResponse(
          [summary("00000000-0000-4000-8000-000000000001", "first")],
          null,
        ),
      );
    vi.stubGlobal("fetch", withTree(cardsMock));
    render(
      <MemoryRouter>
        <CardList />
      </MemoryRouter>,
    );
    expect(
      await screen.findByText("Не удалось загрузить карточки."),
    ).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("first")).toBeVisible();
    expect(screen.getByText("Новая")).toBeVisible();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(cardsMock).toHaveBeenCalledTimes(2);
  });

  it("keeps page one when page two fails and retries the same cursor", async () => {
    const first = summary("00000000-0000-4000-8000-000000000001", "first");
    const second = summary("00000000-0000-4000-8000-000000000002", "second");
    const cardsMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([first], "page-two"))
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockResolvedValueOnce(listResponse([second], null));
    vi.stubGlobal("fetch", withTree(cardsMock));
    render(
      <MemoryRouter>
        <CardList />
      </MemoryRouter>,
    );
    expect(await screen.findByText("first")).toBeVisible();
    await userEvent.click(
      screen.getByRole("button", { name: "Загрузить ещё" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Не удалось загрузить карточки.",
    );
    expect(screen.getByText("first")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("second")).toBeVisible();
    expect(screen.getByText("first")).toBeVisible();
    expect(cardsMock).toHaveBeenNthCalledWith(
      3,
      "/api/v1/cards?cursor=page-two",
      expect.anything(),
    );
  });

  it("shows a tenant-safe detail not-found state", async () => {
    vi.stubGlobal(
      "fetch",
      withTree(
        vi.fn().mockResolvedValue(
          new Response(
            JSON.stringify({
              error: {
                code: "card_not_found",
                message: "Карточка не найдена",
                requestId: crypto.randomUUID(),
              },
            }),
            { status: 404, headers: { "Content-Type": "application/json" } },
          ),
        ),
      ),
    );
    render(
      <MemoryRouter initialEntries={["/app/cards/missing"]}>
        <Routes>
          <Route path="/app/cards/:id" element={<CardDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(
      await screen.findByRole("heading", { name: "Карточка не найдена" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Вернуться к списку" }),
    ).toBeVisible();
  });

  it("retries an initial detail network error", async () => {
    const cardsMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockResolvedValueOnce(cardResponse());
    vi.stubGlobal("fetch", withTree(cardsMock));
    render(
      <MemoryRouter
        initialEntries={["/app/cards/00000000-0000-4000-8000-000000000001"]}
      >
        <Routes>
          <Route path="/app/cards/:id" element={<CardDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Не удалось загрузить карточку.",
    );
    await userEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("front", { exact: true })).toBeVisible();
    expect(cardsMock).toHaveBeenCalledTimes(2);
  });

  it("flips between sides, validates them, and preserves input", async () => {
    vi.stubGlobal(
      "fetch",
      withTree(vi.fn().mockRejectedValue(new TypeError("offline"))),
    );
    render(
      <MemoryRouter>
        <NewCard />
      </MemoryRouter>,
    );
    expect(screen.getByRole("textbox", { name: "Front" })).toBeVisible();
    expect(screen.getByRole("textbox", { name: "Front" })).not.toHaveAttribute(
      "placeholder",
    );
    expect(
      screen.queryByRole("textbox", { name: "Back" }),
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(
      screen
        .getByRole("textbox", { name: "Front" })
        .closest(".card-side-field"),
    ).toHaveTextContent("Поле обязательно");
    await userEvent.type(
      screen.getByRole("textbox", { name: "Front" }),
      "front",
    );
    await userEvent.click(screen.getByRole("button", { name: "Сохранить" }));

    expect(
      screen.queryByRole("textbox", { name: "Front" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Back" })).toBeVisible();
    expect(screen.getByRole("textbox", { name: "Back" })).toHaveFocus();
    expect(
      screen.getByRole("textbox", { name: "Back" }).closest(".card-side-field"),
    ).toHaveTextContent("Поле обязательно");
    await userEvent.type(screen.getByRole("textbox", { name: "Back" }), "back");
    await userEvent.click(
      screen.getByRole("button", { name: "Развернуть на Front" }),
    );
    expect(screen.getByRole("textbox", { name: "Front" })).toHaveValue("front");
    expect(screen.getByRole("textbox", { name: "Front" })).toHaveFocus();
    await userEvent.click(
      screen.getByRole("button", { name: "Развернуть на Back" }),
    );
    expect(screen.getByRole("textbox", { name: "Back" })).toHaveValue("back");

    await userEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Не удалось сохранить",
    );
    expect(screen.getByRole("textbox", { name: "Back" })).toHaveValue("back");
    await userEvent.click(
      screen.getByRole("button", { name: "Развернуть на Front" }),
    );
    expect(screen.getByRole("textbox", { name: "Front" })).toHaveValue("front");
  });

  it("renders text safely and requires explicit delete confirmation", async () => {
    const cardsMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          card: {
            id: "00000000-0000-4000-8000-000000000001",
            folderId: "00000000-0000-4000-8000-000000000002",
            front: "<script>alert(1)</script>",
            back: "plain",
            correctCount: 0,
            incorrectCount: 0,
            correctStreak: 0,
            intervalDays: null,
            lastReviewedAt: null,
            dueAt: null,
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", withTree(cardsMock));
    render(
      <MemoryRouter
        initialEntries={["/app/cards/00000000-0000-4000-8000-000000000001"]}
      >
        <Routes>
          <Route path="/app/cards/:id" element={<CardDetail />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(await screen.findByText("<script>alert(1)</script>")).toBeVisible();
    expect(screen.getByText("Front", { exact: true })).toHaveClass(
      "card-side-label",
    );
    expect(screen.getByText("Back", { exact: true })).toHaveClass(
      "card-side-label",
    );
    expect(document.querySelector("script")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Удалить" }));
    expect(screen.getByRole("dialog")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(cardsMock).toHaveBeenCalledOnce();
  });

  it("edits a card with the same prefilled flip editor as creation", async () => {
    const cardsMock = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "PATCH") {
        const body = JSON.parse(String(init.body)) as {
          front: string;
          back: string;
        };
        return Promise.resolve(
          new Response(
            JSON.stringify({
              card: {
                ...summary("00000000-0000-4000-8000-000000000001", body.front),
                back: body.back,
                folderId: "00000000-0000-4000-8000-000000000002",
                correctCount: 0,
                incorrectCount: 0,
                correctStreak: 0,
                intervalDays: null,
                lastReviewedAt: null,
                dueAt: null,
              },
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );
      }
      return Promise.resolve(cardResponse());
    });
    vi.stubGlobal("fetch", withTree(cardsMock));
    render(
      <MemoryRouter
        initialEntries={["/app/cards/00000000-0000-4000-8000-000000000001"]}
      >
        <Routes>
          <Route path="/app/cards/:id" element={<CardDetail />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(
      await screen.findByRole("heading", { name: "Статистика" }),
    ).toBeVisible();
    await userEvent.click(
      screen.getByRole("button", { name: "Редактировать" }),
    );
    expect(screen.getByRole("textbox", { name: "Front" })).toHaveValue("front");
    expect(
      screen.queryByRole("textbox", { name: "Back" }),
    ).not.toBeInTheDocument();

    await userEvent.clear(screen.getByRole("textbox", { name: "Front" }));
    await userEvent.type(
      screen.getByRole("textbox", { name: "Front" }),
      "draft",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Развернуть на Back" }),
    );
    expect(screen.getByRole("textbox", { name: "Back" })).toHaveValue(
      "front back",
    );
    await userEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.getByText("front", { exact: true })).toBeVisible();

    await userEvent.click(
      screen.getByRole("button", { name: "Редактировать" }),
    );
    await userEvent.clear(screen.getByRole("textbox", { name: "Front" }));
    await userEvent.type(
      screen.getByRole("textbox", { name: "Front" }),
      "updated front",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Развернуть на Back" }),
    );
    await userEvent.clear(screen.getByRole("textbox", { name: "Back" }));
    await userEvent.type(
      screen.getByRole("textbox", { name: "Back" }),
      "updated back",
    );
    await userEvent.click(screen.getByRole("button", { name: "Сохранить" }));

    expect(await screen.findByText("updated front")).toBeVisible();
    expect(screen.getByText("updated back")).toBeVisible();
    expect(cardsMock).toHaveBeenLastCalledWith(
      "/api/v1/cards/00000000-0000-4000-8000-000000000001",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({
          front: "updated front",
          back: "updated back",
          folderId: "00000000-0000-4000-8000-000000000002",
        }),
      }),
    );
  });
});
