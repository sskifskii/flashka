import { FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, Card, CardSummary, RequestError } from "../../shared/api";
import { normalizeSide, validateSide } from "./validation";

function ErrorBox({ message, retry }: { message: string; retry?: () => void }) {
  return (
    <div className="state error-state" role="alert">
      <p>{message}</p>
      {retry && <button onClick={retry}>Повторить</button>}
    </div>
  );
}

export function CardList() {
  const [items, setItems] = useState<CardSummary[]>([]);
  const [cursor, setCursor] = useState<string | null | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = async (next?: string) => {
    setLoading(true);
    setError("");
    try {
      const data = await api.listCards(next);
      setItems((current) => (next ? [...current, ...data.items] : data.items));
      setCursor(data.nextCursor);
    } catch (e) {
      if (!(e instanceof RequestError && e.status === 401))
        setError("Не удалось загрузить карточки.");
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    let active = true;
    void api
      .listCards()
      .then((data) => {
        if (!active) return;
        setItems(data.items);
        setCursor(data.nextCursor);
      })
      .catch((e) => {
        if (active && !(e instanceof RequestError && e.status === 401))
          setError("Не удалось загрузить карточки.");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);
  return (
    <section className="content">
      <div className="title-row">
        <div>
          <p className="eyebrow">Inbox</p>
          <h1>Карточки</h1>
        </div>
        <Link className="button-link" to="/app/cards/new">
          Добавить карточку
        </Link>
      </div>
      {loading && items.length === 0 && (
        <div className="card-skeleton" aria-label="Загрузка карточек" />
      )}
      {error && items.length === 0 && (
        <ErrorBox message={error} retry={() => void load()} />
      )}
      {!loading && !error && items.length === 0 && (
        <div className="state">
          <span className="spark">✦</span>
          <h2>Создайте первую карточку</h2>
          <p>Запишите вопрос и ответ — они сохранятся в Inbox.</p>
          <Link className="button-link" to="/app/cards/new">
            Добавить карточку
          </Link>
        </div>
      )}
      {items.length > 0 && (
        <div className="card-grid">
          {items.map((card) => (
            <Link
              className="card-item"
              to={`/app/cards/${card.id}`}
              key={card.id}
            >
              <strong>{card.front}</strong>
              <span>{card.back}</span>
            </Link>
          ))}
        </div>
      )}
      {error && items.length > 0 && (
        <ErrorBox
          message={error}
          retry={() => void load(cursor ?? undefined)}
        />
      )}
      {cursor && !error && (
        <button
          className="load-more secondary"
          disabled={loading}
          onClick={() => void load(cursor)}
        >
          {loading ? "Загружаем…" : "Загрузить ещё"}
        </button>
      )}
    </section>
  );
}

type FormValues = { front: string; back: string };
function CardFields({
  values,
  setValues,
  errors,
}: {
  values: FormValues;
  setValues: (v: FormValues) => void;
  errors: Record<string, string>;
}) {
  return (
    <>
      <label htmlFor="front">Лицевая сторона</label>
      <textarea
        id="front"
        autoFocus
        value={values.front}
        aria-invalid={!!errors.front}
        onChange={(e) => setValues({ ...values, front: e.target.value })}
      />
      {errors.front && <p className="field-error">{errors.front}</p>}
      <label htmlFor="back">Обратная сторона</label>
      <textarea
        id="back"
        value={values.back}
        aria-invalid={!!errors.back}
        onChange={(e) => setValues({ ...values, back: e.target.value })}
      />
      {errors.back && <p className="field-error">{errors.back}</p>}
    </>
  );
}

export function NewCard() {
  const [values, setValues] = useState<FormValues>({ front: "", back: "" });
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
      return;
    }
    setPending(true);
    setErrors({});
    try {
      const data = await api.createCard({
        front: normalizeSide(values.front),
        back: normalizeSide(values.back),
      });
      setCreated(data.card);
    } catch (e) {
      if (e instanceof RequestError && e.body?.error.fields)
        setErrors(e.body.error.fields);
      else if (!(e instanceof RequestError && e.status === 401))
        setErrors({
          form: "Не удалось сохранить карточку. Попробуйте ещё раз.",
        });
    } finally {
      setPending(false);
    }
  };
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
    <section className="content narrow">
      <Link to="/app">← К карточкам</Link>
      <h1>Новая карточка</h1>
      <form onSubmit={submit}>
        <CardFields values={values} setValues={setValues} errors={errors} />
        {errors.form && (
          <p className="form-error" role="alert">
            {errors.form}
          </p>
        )}
        <button disabled={pending}>
          {pending ? "Сохраняем…" : "Сохранить"}
        </button>
      </form>
    </section>
  );
}

export function CardDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [card, setCard] = useState<Card | null>(null);
  const [values, setValues] = useState<FormValues>({ front: "", back: "" });
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
          <CardFields values={values} setValues={setValues} errors={errors} />
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
            <h2>Лицевая сторона</h2>
            <p>{card.front}</p>
            <h2>Обратная сторона</h2>
            <p>{card.back}</p>
          </article>
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
