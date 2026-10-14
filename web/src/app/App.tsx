import { useState } from "react";
import { Navigate, Outlet, Route, Routes, useNavigate } from "react-router-dom";
import { useAuth } from "../features/auth/AuthProvider";
import { AuthForm } from "../features/auth/AuthForm";
function Loading() {
  return (
    <main className="loading" aria-live="polite">
      <span className="spinner" />
      Проверяем сессию…
    </main>
  );
}
function BootstrapError() {
  const { refresh } = useAuth();
  const [pending, setPending] = useState(false);
  return (
    <main className="loading" role="alert">
      <div>
        <h1>Не удалось проверить сессию</h1>
        <p>Проверьте подключение и попробуйте ещё раз.</p>
        <button
          disabled={pending}
          onClick={async () => {
            setPending(true);
            try {
              await refresh();
            } finally {
              setPending(false);
            }
          }}
        >
          {pending ? "Проверяем…" : "Повторить"}
        </button>
      </div>
    </main>
  );
}
function GuestOnly() {
  const { user, status } = useAuth();
  if (status === "loading") return <Loading />;
  if (status === "error") return <BootstrapError />;
  return user ? <Navigate to="/app" replace /> : <Outlet />;
}
function Protected() {
  const { user, status } = useAuth();
  if (status === "loading") return <Loading />;
  if (status === "error") return <BootstrapError />;
  return user ? <Outlet /> : <Navigate to="/login" replace />;
}
function Shell() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [logoutPending, setLogoutPending] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  return (
    <main className="shell">
      <header>
        <a className="brand" href="/app">
          Flashka
        </a>
        <div>
          <span className="user-login">{user?.login}</span>
          <button
            className="secondary"
            disabled={logoutPending}
            onClick={async () => {
              if (logoutPending) return;
              setLogoutPending(true);
              setLogoutError("");
              try {
                await logout();
                navigate("/login", { replace: true });
              } catch {
                setLogoutError(
                  "Не удалось выйти. Проверьте подключение и повторите.",
                );
              } finally {
                setLogoutPending(false);
              }
            }}
          >
            {logoutPending ? "Выходим…" : "Выйти"}
          </button>
        </div>
      </header>
      {logoutError && (
        <p className="shell-error" role="alert">
          {logoutError}
        </p>
      )}
      <section className="empty">
        <span className="spark">✦</span>
        <h1>Добро пожаловать, {user?.login}</h1>
        <p>
          Ваше пространство готово. Карточки появятся в следующем обновлении.
        </p>
      </section>
    </main>
  );
}
export function App() {
  return (
    <Routes>
      <Route element={<GuestOnly />}>
        <Route path="/login" element={<AuthForm mode="login" />} />
        <Route path="/register" element={<AuthForm mode="register" />} />
      </Route>
      <Route element={<Protected />}>
        <Route path="/app" element={<Shell />} />
      </Route>
      <Route path="*" element={<Navigate to="/app" replace />} />
    </Routes>
  );
}
