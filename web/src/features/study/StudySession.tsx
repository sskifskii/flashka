import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PointerEvent,
} from "react";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { api, RequestError, type StudySessionState } from "../../shared/api";

type Rating = "correct" | "incorrect";

function isTypingTarget(target: EventTarget | null) {
  const element = target as HTMLElement | null;
  return (
    element?.isContentEditable ||
    ["INPUT", "TEXTAREA", "SELECT"].includes(element?.tagName ?? "")
  );
}

export function StudySessionRun() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [session, setSession] = useState<StudySessionState | null>(null);
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [missingActive, setMissingActive] = useState(false);
  const [drag, setDrag] = useState(0);
  const pointerStart = useRef<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data =
        id === "active" || id === "resume"
          ? await api.activeStudySession()
          : await api.studySession(id);
      if (!data) {
        setMissingActive(true);
        return;
      }
      setSession(data.session);
      if (id === "active" || id === "resume")
        navigate(`/app/study/${data.session.id}`, { replace: true });
    } catch (e) {
      if (!(e instanceof RequestError && e.status === 401))
        setError(
          e instanceof RequestError && e.status === 404
            ? "Сессия не найдена."
            : "Не удалось загрузить сессию.",
        );
    } finally {
      setLoading(false);
    }
  }, [id, navigate]);

  useEffect(() => {
    let active = true;
    const request =
      id === "active" || id === "resume"
        ? api.activeStudySession()
        : api.studySession(id);
    void request
      .then((data) => {
        if (!active) return;
        if (!data) {
          setMissingActive(true);
          return;
        }
        setSession(data.session);
        if (id === "active" || id === "resume")
          navigate(`/app/study/${data.session.id}`, { replace: true });
      })
      .catch((e) => {
        if (!active || (e instanceof RequestError && e.status === 401)) return;
        setError(
          e instanceof RequestError && e.status === 404
            ? "Сессия не найдена."
            : "Не удалось загрузить сессию.",
        );
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id, navigate]);

  const reveal = useCallback(async () => {
    if (!session?.current || pending || session.current.revealed) return;
    setPending(true);
    setError("");
    try {
      const data = await api.revealStudyPosition(
        session.id,
        session.current.position,
      );
      setSession(data.session);
    } catch (e) {
      if (!(e instanceof RequestError && e.status === 401))
        setError("Не удалось показать ответ. Попробуйте ещё раз.");
    } finally {
      setPending(false);
    }
  }, [pending, session]);

  const answer = useCallback(
    async (rating: Rating) => {
      if (!session?.current?.revealed || pending) return;
      const position = session.current.position;
      setPending(true);
      setError("");
      try {
        const data = await api.answerStudyPosition(
          session.id,
          position,
          rating,
          crypto.randomUUID(),
        );
        setSession(data.session);
      } catch (e) {
        if (!(e instanceof RequestError && e.status === 401))
          setError("Ответ не сохранился. Карточка осталась на месте.");
      } finally {
        setPending(false);
      }
    },
    [pending, session],
  );

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (isTypingTarget(event.target)) return;
      if (
        !session?.current?.revealed &&
        (event.key === " " || event.key === "Enter")
      ) {
        event.preventDefault();
        void reveal();
      } else if (session?.current?.revealed && event.key === "ArrowLeft") {
        event.preventDefault();
        void answer("incorrect");
      } else if (session?.current?.revealed && event.key === "ArrowRight") {
        event.preventDefault();
        void answer("correct");
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [answer, reveal, session]);

  const pointerDown = (event: PointerEvent<HTMLElement>) => {
    if (!session?.current?.revealed || pending) return;
    pointerStart.current = event.clientX;
    event.currentTarget.setPointerCapture?.(event.pointerId);
  };
  const pointerMove = (event: PointerEvent<HTMLElement>) => {
    if (pointerStart.current === null) return;
    setDrag(event.clientX - pointerStart.current);
  };
  const pointerUp = () => {
    const distance = drag;
    pointerStart.current = null;
    setDrag(0);
    if (Math.abs(distance) < 70) return;
    void answer(distance > 0 ? "correct" : "incorrect");
  };

  if (missingActive)
    return (
      <Navigate to={id === "resume" ? "/app" : "/app/study/new"} replace />
    );
  if (loading)
    return <section className="content narrow">Загрузка сессии…</section>;
  if (!session)
    return (
      <section className="content narrow state error-state" role="alert">
        <p>{error || "Сессия не найдена."}</p>
        <button onClick={() => void load()}>Повторить</button>
        <Link to="/app">К коллекции</Link>
      </section>
    );
  if (session.status === "completed")
    return <Navigate to={`/app/study/${session.id}/result`} replace />;
  if (session.status === "abandoned") return <Navigate to="/app" replace />;
  if (!session.current)
    return (
      <section className="content narrow state error-state" role="alert">
        <p>Текущая карточка недоступна.</p>
        <Link to="/app">К коллекции</Link>
      </section>
    );

  const current = session.current;
  const swipeLabel = drag > 20 ? "Вспомнил" : drag < -20 ? "Не вспомнил" : "";
  const progress = Math.round(
    ((current.position - 1) / session.actualCount) * 100,
  );
  return (
    <section className="content narrow study-run">
      <div className="study-run-nav">
        <Link to="/app" aria-label="Выйти и продолжить позже">
          <span aria-hidden="true">×</span>
          <span className="study-exit-label">Выйти и продолжить позже</span>
        </Link>
        <span className="study-scope-chip">
          <span className="folder-icon" aria-hidden="true" /> Учебная сессия
        </span>
        <button
          className="danger-outline"
          disabled={pending}
          onClick={async () => {
            if (!window.confirm("Завершить сессию как прерванную?")) return;
            setPending(true);
            setError("");
            try {
              await api.abandonStudySession(session.id);
              navigate("/app", { replace: true });
            } catch {
              setError("Не удалось завершить сессию.");
              setPending(false);
            }
          }}
        >
          Прервать
        </button>
      </div>
      <div className="study-progress-block">
        <div className="study-progress-row">
          <span>Повторение</span>
          <p className="study-progress" aria-label="Прогресс">
            {current.position} / {session.actualCount}
          </p>
        </div>
        <div className="study-progress-track" aria-hidden="true">
          <span style={{ width: `${progress}%` }} />
        </div>
      </div>
      <article
        className={`study-card ${drag > 20 ? "swipe-correct" : ""} ${drag < -20 ? "swipe-incorrect" : ""}`}
        onPointerDown={pointerDown}
        onPointerMove={pointerMove}
        onPointerUp={pointerUp}
        onPointerCancel={() => {
          pointerStart.current = null;
          setDrag(0);
        }}
      >
        {swipeLabel && <span className="swipe-label">{swipeLabel}</span>}
        <div>
          <span>Лицевая сторона</span>
          <p>{current.front}</p>
        </div>
        {current.revealed && (
          <div className="study-back">
            <span>Обратная сторона</span>
            <p>{current.back}</p>
          </div>
        )}
      </article>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {!current.revealed ? (
        <button className="study-reveal" disabled={pending} onClick={reveal}>
          {pending ? "Открываем…" : "Показать ответ"}
        </button>
      ) : (
        <div className="study-rating-actions">
          <button
            className="incorrect-action"
            disabled={pending}
            onClick={() => void answer("incorrect")}
          >
            ← Не вспомнил
          </button>
          <button disabled={pending} onClick={() => void answer("correct")}>
            Вспомнил →
          </button>
        </div>
      )}
      <p className="field-hint study-shortcuts">
        {!current.revealed
          ? "Space или Enter — показать ответ"
          : "Стрелки или свайп — выбрать результат"}
      </p>
    </section>
  );
}
