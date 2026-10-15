import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useNavigate } from "react-router-dom";
import { credentialsSchema, Credentials } from "./validation";
import { RequestError } from "../../shared/api";
import { useAuth } from "./AuthProvider";
import { api } from "../../shared/api";
export function AuthForm({ mode }: { mode: "login" | "register" }) {
  const navigate = useNavigate();
  const auth = useAuth();
  const [general, setGeneral] = useState("");
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Credentials>({ resolver: zodResolver(credentialsSchema) });
  const submit = handleSubmit(async (value) => {
    setGeneral("");
    try {
      const response =
        mode === "login" ? await api.login(value) : await api.register(value);
      auth.signedIn(response.user);
      navigate(mode === "login" ? "/app/study/resume" : "/app", {
        replace: true,
      });
    } catch (e) {
      if (e instanceof RequestError) {
        for (const [name, message] of Object.entries(
          e.body?.error.fields ?? {},
        ))
          setError(name as keyof Credentials, { message });
        if (e.status === 401) setGeneral("Неверный логин или пароль");
        else if (e.status === 429)
          setGeneral("Слишком много попыток. Попробуйте позже");
        else if (e.status === 409)
          setGeneral("Этот логин уже занят. Можно перейти на вход");
        else if (e.status === 0)
          setGeneral("Нет связи с сервером. Проверьте подключение и повторите");
        else if (!e.body?.error.fields) setGeneral(e.message);
      } else setGeneral("Не удалось выполнить запрос");
    }
  });
  return (
    <main className="auth-page">
      <section className="auth-card" aria-labelledby="page-title">
        <a className="brand" href="/">
          Flashka
        </a>
        <h1 id="page-title">
          {mode === "login" ? "С возвращением" : "Создайте аккаунт"}
        </h1>
        <p className="subtitle">
          {mode === "login"
            ? "Продолжите заниматься с того же места."
            : "Ваше личное пространство для карточек."}
        </p>
        {mode === "register" && (
          <p className="warning">
            Важно: восстановить забытый пароль в этой версии нельзя.
          </p>
        )}
        <form onSubmit={submit} noValidate>
          <label htmlFor="login">Логин</label>
          <input
            id="login"
            autoComplete="username"
            aria-invalid={!!errors.login}
            {...register("login")}
          />
          {errors.login && (
            <p className="field-error" role="alert">
              {errors.login.message}
            </p>
          )}
          <label htmlFor="password">Пароль</label>
          <input
            id="password"
            type="password"
            autoComplete={
              mode === "login" ? "current-password" : "new-password"
            }
            aria-invalid={!!errors.password}
            {...register("password")}
          />
          {errors.password && (
            <p className="field-error" role="alert">
              {errors.password.message}
            </p>
          )}
          {general && (
            <p className="form-error" role="alert">
              {general}
            </p>
          )}
          <button type="submit" disabled={isSubmitting}>
            {isSubmitting
              ? "Подождите…"
              : mode === "login"
                ? "Войти"
                : "Создать аккаунт"}
          </button>
        </form>
        <p className="switch">
          {mode === "login" ? (
            <>
              Нет аккаунта? <Link to="/register">Создать</Link>
            </>
          ) : (
            <>
              Уже есть аккаунт? <Link to="/login">Войти</Link>
            </>
          )}
        </p>
      </section>
    </main>
  );
}
