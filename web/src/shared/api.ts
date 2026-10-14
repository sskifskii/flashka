export type User = { id: string; login: string };
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
  expectedStatus = 200,
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
    if (expectedStatus !== 204) throw new RequestError(response.status);
    return undefined as T;
  }
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    body = undefined;
  }
  if (!response.ok) throw new RequestError(response.status, body as APIError);
  if (response.status !== expectedStatus)
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
};
