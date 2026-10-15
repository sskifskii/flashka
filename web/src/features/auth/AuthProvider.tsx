import { createContext, useContext, useEffect, useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, RequestError, User } from "../../shared/api";
type AuthState = {
  user?: User;
  status: "loading" | "guest" | "authenticated" | "error";
  refresh: () => Promise<unknown>;
  signedIn: (user: User) => void;
  logout: () => Promise<void>;
};
const AuthContext = createContext<AuthState | null>(null);
const channelName = "flashka-auth";
export function AuthProvider({ children }: { children: React.ReactNode }) {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: ["auth", "me"],
    queryFn: async () => {
      try {
        return (await api.me()).user;
      } catch (e) {
        if (e instanceof RequestError && e.status === 401) return null;
        throw e;
      }
    },
    retry: false,
  });
  const { data, isError, isLoading, refetch } = query;
  const state = useMemo<AuthState>(() => {
    const status: AuthState["status"] = isLoading
      ? "loading"
      : isError
        ? "error"
        : data
          ? "authenticated"
          : "guest";
    return {
      user: data ?? undefined,
      status,
      refresh: () => refetch(),
      signedIn: (user) => client.setQueryData(["auth", "me"], user),
      logout: async () => {
        await api.logout();
        await client.cancelQueries({ queryKey: ["auth", "me"] });
        client.setQueryData(["auth", "me"], null);
        if ("BroadcastChannel" in window) {
          const c = new BroadcastChannel(channelName);
          c.postMessage("logout");
          c.close();
        }
      },
    };
  }, [client, data, isError, isLoading, refetch]);
  useEffect(() => {
    const onFocus = () => {
      if (document.visibilityState === "visible") void refetch();
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    const c =
      "BroadcastChannel" in window
        ? new BroadcastChannel(channelName)
        : undefined;
    c?.addEventListener("message", (event) => {
      if (event.data === "logout") client.setQueryData(["auth", "me"], null);
    });
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
      c?.close();
    };
  }, [client, refetch]);
  useEffect(() => {
    const unauthenticated = () => client.setQueryData(["auth", "me"], null);
    window.addEventListener("flashka:unauthenticated", unauthenticated);
    return () =>
      window.removeEventListener("flashka:unauthenticated", unauthenticated);
  }, [client]);
  return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>;
}
export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider missing");
  return value;
}
