import { FormEvent, useEffect, useRef, useState } from "react";
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import {
  api,
  Card,
  CardSummary,
  FolderNode,
  RequestError,
} from "../../shared/api";
import {
  flattenFolders,
  FolderPanel,
  FolderSelect,
  useFolderTree,
} from "../folders/Folders";
import { normalizeSide, validateSide } from "./validation";

const statusLabels: Record<CardSummary["status"], string> = {
  new: "Новая",
  learning: "Изучается",
  due: "К повторению",
  mastered: "Освоена",
};

function localDate(value: string | null) {
  return value
    ? new Intl.DateTimeFormat("ru", { dateStyle: "medium" }).format(
        new Date(value),
      )
    : "—";
}

function ErrorBox({ message, retry }: { message: string; retry?: () => void }) {
  return (
    <div className="state error-state" role="alert">
      <p>{message}</p>
      {retry && <button onClick={retry}>Повторить</button>}
    </div>
  );
}

export function CardList() {
  const [params, setParams] = useSearchParams();
  const requestedFolder = params.get("folderId") ?? undefined;
  const folders = useFolderTree();
  const [foldersOpen, setFoldersOpen] = useState(false);
  const [items, setItems] = useState<CardSummary[]>([]);
  const [cursor, setCursor] = useState<string | null | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [loadedFolder, setLoadedFolder] = useState<string>();
  const load = async (next?: string) => {
    setLoading(true);
    setError("");
    try {
      const data = await api.listCards(next, requestedFolder);
      setItems((current) => (next ? [...current, ...data.items] : data.items));
      setCursor(data.nextCursor);
      setLoadedFolder(requestedFolder ?? "");
    } catch (e) {
      if (e instanceof RequestError && e.status === 404 && requestedFolder) {
        setParams({});
      } else if (!(e instanceof RequestError && e.status === 401))
        setError("Не удалось загрузить карточки.");
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    let active = true;
    void api
      .listCards(undefined, requestedFolder)
      .then((data) => {
        if (!active) return;
        setItems(data.items);
        setCursor(data.nextCursor);
        setLoadedFolder(requestedFolder ?? "");
        setError("");
      })
      .catch((e) => {
        if (
          active &&
          e instanceof RequestError &&
          e.status === 404 &&
          requestedFolder
        ) {
          setParams({});
        } else if (active && !(e instanceof RequestError && e.status === 401)) {
          setError("Не удалось загрузить карточки.");
          setLoadedFolder(requestedFolder ?? "");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [requestedFolder, setParams]);
  const selectedId = requestedFolder ?? folders.root?.id ?? "";
  const switchingFolder = loadedFolder !== (requestedFolder ?? "");
  const selected = folders.root
    ? flattenFolders(folders.root).find((folder) => folder.id === selectedId)
    : undefined;
  const selectFolder = (node: FolderNode) => {
    setParams(node.isInbox ? {} : { folderId: node.id });
    setFoldersOpen(false);
  };
  const refreshFolders = async (preferredId?: string) => {
    const root = await folders.load();
    if (!root) return;
    const target =
      preferredId && preferredId !== root.id ? preferredId : undefined;
    setParams(target ? { folderId: target } : {});
  };
  return (
    <section className="content collection">
      <button
        className="folder-toggle secondary"
        onClick={() => setFoldersOpen((value) => !value)}
        aria-expanded={foldersOpen}
        aria-label={foldersOpen ? "Скрыть папки" : "Показать папки"}
      >
        <span aria-hidden="true">{foldersOpen ? "×" : "☰"}</span>
        {foldersOpen ? "Закрыть папки" : "Папки"}
      </button>
      {folders.loading && !folders.root && (
        <div className="folder-loading">Загрузка папок…</div>
      )}
      {folders.error && !folders.root && (
        <ErrorBox message={folders.error} retry={() => void folders.load()} />
      )}
      {folders.root && (
        <div className={foldersOpen ? "folder-drawer open" : "folder-drawer"}>
          <FolderPanel
            root={folders.root}
            selectedId={selectedId}
            onSelect={selectFolder}
            onChanged={refreshFolders}
          />
        </div>
      )}
      <div className="collection-cards">
        <div className="title-row">
          <div>
            <p className="eyebrow">Коллекция &nbsp;›&nbsp; Карточки</p>
            <h1 aria-label="Карточки">{selected?.name ?? "Карточки"}</h1>
          </div>
          <div className="title-actions">
            <Link
              className="button-link secondary-link"
              to={`/app/study/new${requestedFolder ? `?folderId=${requestedFolder}` : ""}`}
            >
              <span aria-hidden="true">▷</span> Учить
            </Link>
            <Link
              className="button-link"
              aria-label="Добавить карточку"
              to={`/app/cards/new${requestedFolder ? `?folderId=${requestedFolder}` : ""}`}
            >
              <span aria-hidden="true">＋</span> Карточка
            </Link>
          </div>
        </div>
        {(loading || switchingFolder) && (
          <div className="card-skeleton" aria-label="Загрузка карточек" />
        )}
        {!switchingFolder && error && items.length === 0 && (
          <ErrorBox message={error} retry={() => void load()} />
        )}
        {!loading && !switchingFolder && !error && items.length === 0 && (
          <div className="state">
            <span className="spark">✦</span>
            <h2>Создайте первую карточку</h2>
            <p>
              Запишите вопрос и ответ — они сохранятся в{" "}
              {selected?.name ?? "Inbox"}.
            </p>
            <Link
              className="button-link"
              to={`/app/cards/new${requestedFolder ? `?folderId=${requestedFolder}` : ""}`}
            >
              Добавить карточку
            </Link>
          </div>
        )}
        {!switchingFolder && items.length > 0 && (
          <div className="card-grid">
            {items.map((card) => (
              <Link
                className="card-item"
                to={`/app/cards/${card.id}`}
                key={card.id}
              >
                <small className={`status-badge status-${card.status}`}>
                  {statusLabels[card.status]}
                </small>
                <strong>{card.front}</strong>
                <span className="card-preview-back">{card.back}</span>
              </Link>
            ))}
          </div>
        )}
        {!switchingFolder && error && items.length > 0 && (
          <ErrorBox
            message={error}
            retry={() => void load(cursor ?? undefined)}
          />
        )}
        {!switchingFolder && cursor && !error && (
          <button
            className="load-more secondary"
            disabled={loading}
            onClick={() => void load(cursor)}
          >
            {loading ? "Загружаем…" : "Загрузить ещё"}
          </button>
        )}
      </div>
    </section>
  );
}

type FormValues = { front: string; back: string };
type CardSide = keyof FormValues;

function CardFields({
  values,
  setValues,
  errors,
  root,
  folderId,
  setFolderId,
  activeSide,
  onFlip,
}: {
  values: FormValues;
  setValues: (v: FormValues) => void;
  errors: Record<string, string>;
  root?: FolderNode | null;
  folderId?: string;
  setFolderId?: (id: string) => void;
  activeSide?: CardSide;
  onFlip?: () => void;
}) {
  const frontRef = useRef<HTMLTextAreaElement>(null);
  const backRef = useRef<HTMLTextAreaElement>(null);
  const sides: Array<{
    id: CardSide;
    label: string;
    placeholder: string;
  }> = [
    { id: "front", label: "Front", placeholder: "Слово, вопрос или фраза" },
    { id: "back", label: "Back", placeholder: "Ответ или пояснение" },
  ];
  const flippable = activeSide !== undefined && onFlip !== undefined;

  useEffect(() => {
    if (!flippable || !activeSide) return;
    (activeSide === "front" ? frontRef : backRef).current?.focus({
      preventScroll: true,
    });
  }, [activeSide, flippable]);

  const renderSide = (side: (typeof sides)[number]) => {
    const active = !flippable || side.id === activeSide;
    return (
      <div
        className={`card-side-field ${side.id}-side-field${flippable ? ` card-flip-face card-flip-${side.id}` : ""}`}
        aria-hidden={flippable ? !active : undefined}
        key={side.id}
      >
        <div className="card-side-heading">
          <label className="card-side-label" htmlFor={side.id}>
            {side.label}
          </label>
          {flippable && (
            <button
              type="button"
              className="card-side-flip"
              aria-label={`Развернуть на ${side.id === "front" ? "Back" : "Front"}`}
              tabIndex={active ? 0 : -1}
              onClick={onFlip}
            >
              <span aria-hidden="true">↻</span> Развернуть
            </button>
          )}
        </div>
        <textarea
          ref={side.id === "front" ? frontRef : backRef}
          id={side.id}
          autoFocus={side.id === "front"}
          tabIndex={active ? 0 : -1}
          rows={flippable ? 1 : undefined}
          placeholder={flippable ? undefined : side.placeholder}
          value={values[side.id]}
          aria-invalid={!!errors[side.id]}
          onChange={(e) => setValues({ ...values, [side.id]: e.target.value })}
        />
        {errors[side.id] && <p className="field-error">{errors[side.id]}</p>}
      </div>
    );
  };

  return (
    <>
      {flippable ? (
        <div className="card-flip-scene">
          <div
            className={`card-flip-card${activeSide === "back" ? " show-back" : ""}`}
          >
            {sides.map(renderSide)}
          </div>
        </div>
      ) : (
        sides.map(renderSide)
      )}
      {root && folderId && setFolderId && (
        <div className="folder-field">
          <label htmlFor="folder">Папка</label>
          <FolderSelect root={root} value={folderId} onChange={setFolderId} />
          {errors.folderId && <p className="field-error">{errors.folderId}</p>}
        </div>
      )}
    </>
  );
}

export function NewCard() {
  const [params] = useSearchParams();
  const folders = useFolderTree();
  const [folderId, setFolderId] = useState(params.get("folderId") ?? "");
  const [values, setValues] = useState<FormValues>({ front: "", back: "" });
  const [activeSide, setActiveSide] = useState<CardSide>("front");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [pending, setPending] = useState(false);
  const [created, setCreated] = useState<Card | null>(null);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const local = {
      front: validateSide(values.front),
      back: validateSide(values.back),
    };
    const clean = Object.fromEntries(
      Object.entries(local).filter(([, v]) => v),
    );
    if (Object.keys(clean).length) {
      setErrors(clean);
      setActiveSide(local.front ? "front" : "back");
      return;
    }
    setPending(true);
    setErrors({});
    try {
      const data = await api.createCard({
        front: normalizeSide(values.front),
        back: normalizeSide(values.back),
        folderId: folderId || folders.root?.id || undefined,
      });
      setCreated(data.card);
    } catch (e) {
      if (e instanceof RequestError && e.body?.error.fields) {
        setErrors(e.body.error.fields);
        if (e.body.error.fields.front) setActiveSide("front");
        else if (e.body.error.fields.back) setActiveSide("back");
      } else if (!(e instanceof RequestError && e.status === 401))
        setErrors({
          form: "Не удалось сохранить карточку. Попробуйте ещё раз.",
        });
    } finally {
      setPending(false);
    }
  };
  const effectiveFolderId = folderId || folders.root?.id || "";
  if (created)
    return (
      <section className="content narrow state">
        <span className="spark">✓</span>
        <h1>Карточка сохранена</h1>
        <div className="actions">
          <button
            onClick={() => {
              setCreated(null);
              setValues({ front: "", back: "" });
              setActiveSide("front");
            }}
          >
            Добавить ещё
          </button>
          <Link
            className="button-link secondary-link"
            to={`/app/cards/${created.id}`}
          >
            Открыть карточку
          </Link>
        </div>
      </section>
    );
  return (
    <section className="content narrow card-editor-page">
      <form className="card-editor-form" onSubmit={submit}>
        <header className="card-editor-header">
          <Link to="/app" className="editor-cancel">
            <span aria-hidden="true">×</span> Отмена
          </Link>
          <h1>Новая карточка</h1>
          <button disabled={pending}>
            <span aria-hidden="true">✓</span>
            {pending ? "Сохраняем…" : "Сохранить"}
          </button>
        </header>
        {folders.root && (
          <div className="folder-banner">
            <span className="folder-icon" aria-hidden="true" />
            <label htmlFor="folder">Папка:</label>
            <FolderSelect
              root={folders.root}
              value={effectiveFolderId}
              onChange={setFolderId}
            />
          </div>
        )}
        <CardFields
          values={values}
          setValues={setValues}
          errors={errors}
          activeSide={activeSide}
          onFlip={() =>
            setActiveSide((side) => (side === "front" ? "back" : "front"))
          }
        />
        {folders.error && (
          <p className="form-error" role="alert">
            {folders.error} Новая карточка будет сохранена в Inbox.
          </p>
        )}
        {errors.form && (
          <p className="form-error" role="alert">
            {errors.form}
          </p>
        )}
      </form>
    </section>
  );
}

export function CardDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [card, setCard] = useState<Card | null>(null);
  const [values, setValues] = useState<FormValues>({ front: "", back: "" });
  const folders = useFolderTree();
  const [folderId, setFolderId] = useState("");
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [pending, setPending] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [notFound, setNotFound] = useState(false);
  const load = async () => {
    setLoading(true);
    setErrors({});
    try {
      const data = await api.getCard(id);
      setCard(data.card);
      setValues({ front: data.card.front, back: data.card.back });
      setFolderId(data.card.folderId);
    } catch (e) {
      if (e instanceof RequestError && e.status === 404) setNotFound(true);
      else if (!(e instanceof RequestError && e.status === 401))
        setErrors({ form: "Не удалось загрузить карточку." });
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    let active = true;
    void api
      .getCard(id)
      .then((data) => {
        if (!active) return;
        setCard(data.card);
        setValues({ front: data.card.front, back: data.card.back });
        setFolderId(data.card.folderId);
      })
      .catch((e) => {
        if (!active) return;
        if (e instanceof RequestError && e.status === 404) setNotFound(true);
        else if (!(e instanceof RequestError && e.status === 401))
          setErrors({ form: "Не удалось загрузить карточку." });
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id]);
  if (loading)
    return (
      <section className="content">
        <div className="card-skeleton" aria-label="Загрузка карточки" />
      </section>
    );
  if (notFound)
    return (
      <section className="content state">
        <h1>Карточка не найдена</h1>
        <Link className="button-link" to="/app">
          Вернуться к списку
        </Link>
      </section>
    );
  if (errors.form && !card)
    return (
      <section className="content">
        <ErrorBox message={errors.form} retry={() => void load()} />
      </section>
    );
  if (!card) return null;
  const save = async (e: FormEvent) => {
    e.preventDefault();
    const local = {
      front: validateSide(values.front),
      back: validateSide(values.back),
    };
    const clean = Object.fromEntries(
      Object.entries(local).filter(([, v]) => v),
    );
    if (Object.keys(clean).length) {
      setErrors(clean);
      return;
    }
    setPending(true);
    setErrors({});
    try {
      const data = await api.updateCard(id, {
        front: normalizeSide(values.front),
        back: normalizeSide(values.back),
        folderId,
      });
      setCard(data.card);
      setEditing(false);
    } catch (e) {
      if (e instanceof RequestError && e.status === 404) {
        setNotFound(true);
      } else if (e instanceof RequestError && e.body?.error.fields)
        setErrors(e.body.error.fields);
      else if (!(e instanceof RequestError && e.status === 401))
        setErrors({ form: "Не удалось сохранить изменения." });
    } finally {
      setPending(false);
    }
  };
  const remove = async () => {
    setPending(true);
    setErrors({});
    try {
      await api.deleteCard(id);
      navigate("/app", { replace: true });
    } catch (e) {
      if (e instanceof RequestError && e.status === 404) {
        setNotFound(true);
        setConfirming(false);
      } else if (!(e instanceof RequestError && e.status === 401))
        setErrors({ form: "Не удалось удалить карточку." });
    } finally {
      setPending(false);
    }
  };
  return (
    <section className="content narrow">
      <Link to="/app">← К карточкам</Link>
      <h1>{editing ? "Редактирование" : "Карточка"}</h1>
      {editing ? (
        <form onSubmit={save}>
          <CardFields
            values={values}
            setValues={setValues}
            errors={errors}
            root={folders.root}
            folderId={folderId}
            setFolderId={setFolderId}
          />
          {errors.form && (
            <p className="form-error" role="alert">
              {errors.form}
            </p>
          )}
          <div className="actions">
            <button disabled={pending}>
              {pending ? "Сохраняем…" : "Сохранить"}
            </button>
            <button
              type="button"
              className="secondary"
              onClick={() => {
                setEditing(false);
                setValues({ front: card.front, back: card.back });
                setFolderId(card.folderId);
                setErrors({});
              }}
            >
              Отмена
            </button>
          </div>
        </form>
      ) : (
        <>
          <article className="detail">
            <small className={`status-badge status-${card.status}`}>
              {statusLabels[card.status]}
            </small>
            <div className="card-detail-side">
              <span className="card-side-label">Front</span>
              <p>{card.front}</p>
            </div>
            <div className="card-detail-side">
              <span className="card-side-label">Back</span>
              <p>{card.back}</p>
            </div>
          </article>
          <dl className="card-progress">
            <div>
              <dt>Правильных ответов</dt>
              <dd>{card.correctCount}</dd>
            </div>
            <div>
              <dt>Ошибок</dt>
              <dd>{card.incorrectCount}</dd>
            </div>
            <div>
              <dt>Текущая серия</dt>
              <dd>{card.correctStreak}</dd>
            </div>
            <div>
              <dt>Интервал</dt>
              <dd>{card.intervalDays ? `${card.intervalDays} дн.` : "—"}</dd>
            </div>
            <div>
              <dt>Последний ответ</dt>
              <dd>{localDate(card.lastReviewedAt)}</dd>
            </div>
            <div>
              <dt>Следующее повторение</dt>
              <dd>{localDate(card.dueAt)}</dd>
            </div>
          </dl>
          {errors.form && (
            <p className="form-error" role="alert">
              {errors.form}
            </p>
          )}
          <div className="actions">
            <button onClick={() => setEditing(true)}>Редактировать</button>
            <button className="danger" onClick={() => setConfirming(true)}>
              Удалить
            </button>
          </div>
        </>
      )}
      {confirming && (
        <div
          className="confirm"
          role="dialog"
          aria-modal="true"
          aria-labelledby="confirm-title"
        >
          <div>
            <h2 id="confirm-title">Удалить карточку?</h2>
            <p>Это действие необратимо.</p>
            <div className="actions">
              <button
                className="danger"
                disabled={pending}
                onClick={() => void remove()}
              >
                {pending ? "Удаляем…" : "Удалить навсегда"}
              </button>
              <button
                className="secondary"
                disabled={pending}
                onClick={() => setConfirming(false)}
              >
                Отмена
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
