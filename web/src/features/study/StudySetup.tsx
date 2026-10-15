import { FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api, RequestError, StudyPreview } from "../../shared/api";
import { flattenFolders, useFolderTree } from "../folders/Folders";

export function StudySetup() {
  const folders = useFolderTree();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [scope, setScope] = useState(params.get("folderId") ?? "all");
  const [count, setCount] = useState(10);
  const [preview, setPreview] = useState<StudyPreview | null>(null);
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const folderId = scope === "all" ? undefined : scope;

  useEffect(() => {
    let active = true;
    void api
      .studyPreview(folderId ? "folder" : "all", folderId)
      .then((data) => {
        if (active) setPreview(data.preview);
      })
      .catch((e) => {
        if (active && !(e instanceof RequestError && e.status === 401)) {
          setError("Не удалось загрузить сводку.");
          setPreview(null);
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [folderId]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!preview?.available || count < 1 || count > 100) return;
    setPending(true);
    setError("");
    try {
      const data = await api.createStudySession({
        scopeType: folderId ? "folder" : "all",
        folderId,
        count,
      });
      navigate(`/app/study/${data.session.id}`);
    } catch (e) {
      if (e instanceof RequestError && e.body?.error.fields?.count)
        setError(e.body.error.fields.count);
      else if (!(e instanceof RequestError && e.status === 401))
        setError(
          e instanceof RequestError ? e.message : "Не удалось начать сессию.",
        );
    } finally {
      setPending(false);
    }
  };

  const actual = Math.min(count || 0, preview?.available ?? 0);
  return (
    <section className="content narrow study-setup">
      <Link to="/app">← К карточкам</Link>
      <p className="eyebrow">Новый раунд</p>
      <h1>Что будем учить?</h1>
      <form className="study-form" onSubmit={submit}>
        <label htmlFor="study-scope">Область</label>
        <select
          id="study-scope"
          value={scope}
          disabled={folders.loading || !folders.root}
          onChange={(event) => {
            setLoading(true);
            setError("");
            setPreview(null);
            setScope(event.target.value);
          }}
        >
          <option value="all">Все карточки</option>
          {folders.root &&
            flattenFolders(folders.root).map((folder) => (
              <option value={folder.id} key={folder.id}>
                {folder.label}
              </option>
            ))}
        </select>
        <p className="field-hint">
          Папка включает карточки из всех её подпапок.
        </p>

        <fieldset>
          <legend>Размер сессии</legend>
          <div className="quick-counts">
            {[5, 10, 20].map((value) => (
              <button
                type="button"
                className={count === value ? "selected" : "secondary"}
                onClick={() => setCount(value)}
                key={value}
              >
                {value}
              </button>
            ))}
          </div>
          <label htmlFor="study-count">Другое количество</label>
          <input
            id="study-count"
            type="number"
            min="1"
            max="100"
            value={count}
            aria-invalid={count < 1 || count > 100}
            onChange={(event) => setCount(Number(event.target.value))}
          />
          {(count < 1 || count > 100) && (
            <p className="field-error">Введите число от 1 до 100</p>
          )}
        </fieldset>

        {loading && (
          <div className="card-skeleton" aria-label="Загрузка сводки" />
        )}
        {!loading && preview && (
          <div className="study-preview" aria-live="polite">
            <strong>Доступно: {preview.available}</strong>
            <span>К повторению: {preview.due}</span>
            <span>Новых: {preview.new}</span>
            <span>Освоенных: {preview.mastered}</span>
            <p>В раунде будет: {actual}</p>
          </div>
        )}
        {!loading && preview?.available === 0 && (
          <div className="state compact">
            <p>
              В этой области пока нет карточек. Выберите другую папку или
              добавьте карточку.
            </p>
            <Link
              className="button-link secondary-link"
              to={`/app/cards/new${folderId ? `?folderId=${folderId}` : ""}`}
            >
              Добавить карточку
            </Link>
          </div>
        )}
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <button
          disabled={
            pending ||
            loading ||
            !preview?.available ||
            count < 1 ||
            count > 100
          }
        >
          {pending ? "Создаём раунд…" : `Начать ${actual || ""}`.trim()}
        </button>
      </form>
    </section>
  );
}
