export type User = { id: string; login: string };
export type CardSummary = {
  id: string;
  front: string;
  back: string;
  status: "new" | "learning" | "due" | "mastered";
  createdAt: string;
  updatedAt: string;
};
export type Card = CardSummary & {
  folderId: string;
  correctCount: number;
  incorrectCount: number;
  correctStreak: number;
  intervalDays: number | null;
  lastReviewedAt: string | null;
  dueAt: string | null;
};
export type Folder = {
  id: string;
  parentId: string;
  name: string;
  isInbox: boolean;
  createdAt: string;
  updatedAt: string;
};
export type FolderNode = Omit<Folder, "parentId"> & {
  parentId: string | null;
  directCardCount: number;
  totalCardCount: number;
  children: FolderNode[];
};
export type StudyPreview = {
  available: number;
  due: number;
  new: number;
  mastered: number;
};
export type StudySession = {
  id: string;
  scopeType: "all" | "folder";
  scopeFolderId: string | null;
  requestedCount: number;
  actualCount: number;
  status: "active";
  createdAt: string;
  first: { position: number; cardId: string; front: string };
};
export type StudySessionState = {
  id: string;
  actualCount: number;
  answeredCount: number;
  status: "active" | "completed" | "abandoned";
  current: {
    position: number;
    cardId: string;
    front: string;
    back: string | null;
    revealed: boolean;
  } | null;
};
export type StudyResult = {
  id: string;
  scopeType: "all" | "folder";
  scopeFolderId: string | null;
  actualCount: number;
  correctCount: number;
  incorrectCount: number;
  correctPercent: number;
  errors: { cardId: string; front: string; back: string }[];
};
export type APIError = {
  error: {
    code: string;
    message: string;
    fields?: Record<string, string>;
    requestId: string;
  };
};
export class RequestError extends Error {
  constructor(
    public status: number,
    public body?: APIError,
  ) {
    super(body?.error.message ?? "Сетевая ошибка");
  }
}
async function request<T>(
  path: string,
  init?: RequestInit,
  expectedStatus: number | number[] = 200,
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/v1${path}`, {
      credentials: "include",
      ...init,
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
    });
  } catch {
    throw new RequestError(0);
  }
  if (response.status === 204) {
    if (!(
      expectedStatus === 204 ||
      (Array.isArray(expectedStatus) && expectedStatus.includes(204))
    ))
      throw new RequestError(response.status);
    return undefined as T;
  }
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    body = undefined;
  }
  if (!response.ok) {
    if (response.status === 401 && path !== "/auth/me")
      window.dispatchEvent(new Event("flashka:unauthenticated"));
    throw new RequestError(response.status, body as APIError);
  }
  if (
    !(Array.isArray(expectedStatus)
      ? expectedStatus.includes(response.status)
      : response.status === expectedStatus)
  )
    throw new RequestError(response.status);
  return body as T;
}
export const api = {
  me: () => request<{ user: User }>("/auth/me"),
  register: (value: { login: string; password: string }) =>
    request<{ user: User }>(
      "/auth/register",
      {
        method: "POST",
        body: JSON.stringify(value),
      },
      201,
    ),
  login: (value: { login: string; password: string }) =>
    request<{ user: User }>("/auth/login", {
      method: "POST",
      body: JSON.stringify(value),
    }),
  logout: () => request<void>("/auth/logout", { method: "POST" }, 204),
  listCards: (cursor?: string, folderId?: string) => {
    const params = new URLSearchParams();
    if (cursor) params.set("cursor", cursor);
    if (folderId) params.set("folderId", folderId);
    const query = params.toString();
    return request<{ items: CardSummary[]; nextCursor: string | null }>(
      `/cards${query ? `?${query}` : ""}`,
    );
  },
  createCard: (value: { front: string; back: string; folderId?: string }) =>
    request<{ card: Card }>(
      "/cards",
      { method: "POST", body: JSON.stringify(value) },
      201,
    ),
  getCard: (id: string) => request<{ card: Card }>(`/cards/${id}`),
  updateCard: (
    id: string,
    value: { front: string; back: string; folderId?: string },
  ) =>
    request<{ card: Card }>(`/cards/${id}`, {
      method: "PATCH",
      body: JSON.stringify(value),
    }),
  deleteCard: (id: string) =>
    request<void>(`/cards/${id}`, { method: "DELETE" }, 204),
  folderTree: () => request<{ root: FolderNode }>("/folders/tree"),
  createFolder: (value: { parentId: string; name: string }) =>
    request<{ folder: Folder }>(
      "/folders",
      { method: "POST", body: JSON.stringify(value) },
      201,
    ),
  updateFolder: (id: string, value: { name?: string; parentId?: string }) =>
    request<{ folder: Folder }>(`/folders/${id}`, {
      method: "PATCH",
      body: JSON.stringify(value),
    }),
  deleteFolder: (id: string) =>
    request<void>(`/folders/${id}`, { method: "DELETE" }, 204),
  studyPreview: (scopeType: "all" | "folder", folderId?: string) => {
    const params = new URLSearchParams({ scopeType });
    if (folderId) params.set("folderId", folderId);
    return request<{ preview: StudyPreview }>(`/study/preview?${params}`);
  },
  createStudySession: (value: {
    scopeType: "all" | "folder";
    folderId?: string;
    count: number;
  }) =>
    request<{ session: StudySession }>(
      "/study/sessions",
      { method: "POST", body: JSON.stringify(value) },
      201,
    ),
  activeStudySession: () =>
    request<{ session: StudySessionState } | undefined>(
      "/study/sessions/active",
      undefined,
      [200, 204],
    ),
  studySession: (id: string) =>
    request<{ session: StudySessionState }>(`/study/sessions/${id}`),
  studyResult: (id: string) =>
    request<{ result: StudyResult }>(`/study/sessions/${id}/result`),
  revealStudyPosition: (id: string, position: number) =>
    request<{ session: StudySessionState }>(
      `/study/sessions/${id}/cards/${position}/reveal`,
      { method: "POST" },
    ),
  answerStudyPosition: (
    id: string,
    position: number,
    rating: "correct" | "incorrect",
    requestId: string,
  ) =>
    request<{ session: StudySessionState }>(
      `/study/sessions/${id}/cards/${position}/answer`,
      { method: "PUT", body: JSON.stringify({ rating, requestId }) },
    ),
  abandonStudySession: (id: string) =>
    request<void>(`/study/sessions/${id}/abandon`, { method: "POST" }, 204),
};
