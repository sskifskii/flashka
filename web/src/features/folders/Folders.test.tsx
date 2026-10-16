import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FolderNode } from "../../shared/api";
import { FolderPanel } from "./Folders";

afterEach(() => vi.unstubAllGlobals());

const now = new Date().toISOString();
const grandchild: FolderNode = {
  id: "00000000-0000-4000-8000-000000000003",
  parentId: "00000000-0000-4000-8000-000000000002",
  name: "Nested",
  isInbox: false,
  directCardCount: 0,
  totalCardCount: 0,
  createdAt: now,
  updatedAt: now,
  children: [],
};
const child: FolderNode = {
  id: "00000000-0000-4000-8000-000000000002",
  parentId: "00000000-0000-4000-8000-000000000001",
  name: "Topic",
  isInbox: false,
  directCardCount: 2,
  totalCardCount: 3,
  createdAt: now,
  updatedAt: now,
  children: [grandchild],
};
const root: FolderNode = {
  id: "00000000-0000-4000-8000-000000000001",
  parentId: null,
  name: "Inbox",
  isInbox: true,
  directCardCount: 1,
  totalCardCount: 4,
  createdAt: now,
  updatedAt: now,
  children: [child],
};

describe("folder panel", () => {
  it("selects a folder and explains direct/total counts", async () => {
    const select = vi.fn();
    render(
      <FolderPanel
        root={root}
        selectedId={root.id}
        onSelect={select}
        onChanged={vi.fn()}
      />,
    );
    expect(screen.getByText("в папке / с подпапками")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: /Topic/ }));
    expect(select).toHaveBeenCalledWith(child);
  });

  it("shows Inbox beside top-level folders while preserving deeper nesting", () => {
    render(
      <FolderPanel
        root={root}
        selectedId={root.id}
        onSelect={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    const inboxItem = screen
      .getByRole("button", { name: /Inbox/ })
      .closest("li");
    const topicItem = screen
      .getByRole("button", { name: /Topic/ })
      .closest("li");
    const nestedItem = screen
      .getByRole("button", { name: /Nested/ })
      .closest("li");

    expect(inboxItem?.parentElement).toBe(topicItem?.parentElement);
    expect(nestedItem?.parentElement).toBe(topicItem?.querySelector("ul"));
  });

  it("creates a trimmed child and preserves server field errors", async () => {
    const changed = vi.fn().mockResolvedValue(undefined);
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            error: {
              code: "folder_name_taken",
              message: "Название уже используется",
              fields: { name: "Название уже используется" },
              requestId: crypto.randomUUID(),
            },
          }),
          { status: 409, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ folder: { ...child, name: "New" } }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);
    render(
      <FolderPanel
        root={root}
        selectedId={root.id}
        onSelect={vi.fn()}
        onChanged={changed}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Создать внутри" }),
    );
    await userEvent.type(screen.getByLabelText("Название"), "  New  ");
    await userEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Название уже используется",
    );
    await userEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(changed).toHaveBeenCalledWith(child.id);
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/api/v1/folders",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ parentId: root.id, name: "New" }),
      }),
    );
  });
});
