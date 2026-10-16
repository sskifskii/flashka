import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  api,
  RequestError,
  type StudyResult as Result,
} from "../../shared/api";

function errorMessage(e: unknown) {
  return e instanceof RequestError && e.status === 404
    ? "Сессия не найдена."
    : e instanceof RequestError && e.status === 409
      ? "Итог появится после завершения сессии."
      : "Не удалось загрузить итог.";
}

export function StudyResult() {
  const { id = "" } = useParams();
  const [result, setResult] = useState<Result | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setResult((await api.studyResult(id)).result);
    } catch (e) {
      if (!(e instanceof RequestError && e.status === 401)) {
        setError(errorMessage(e));
      }
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    let active = true;
    void api
      .studyResult(id)
      .then((data) => {
        if (active) setResult(data.result);
      })
      .catch((e) => {
        if (active && !(e instanceof RequestError && e.status === 401)) {
          setError(errorMessage(e));
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id]);

  if (loading)
    return <section className="content narrow">Загрузка результата…</section>;
  if (!result)
    return (
      <section className="content narrow state error-state" role="alert">
        <p>{error}</p>
        <button onClick={() => void load()}>Повторить</button>
        <Link to="/app">К коллекции</Link>
      </section>
    );

  const repeat =
    result.scopeType === "folder" && result.scopeFolderId
      ? `/app/study/new?folderId=${result.scopeFolderId}`
      : "/app/study/new";
  return (
    <section className="content narrow study-result">
      <p className="eyebrow">Раунд завершён</p>
      <h1>Результат</h1>
      <div className="result-summary" aria-label="Итог раунда">
        <div>
          <strong>{result.correctCount}</strong>
          <span>Вспомнил</span>
        </div>
        <div>
          <strong>{result.incorrectCount}</strong>
          <span>Не вспомнил</span>
        </div>
        <div>
          <strong>{result.correctPercent}%</strong>
          <span>Правильных</span>
        </div>
        <div>
          <strong>{result.actualCount}</strong>
          <span>Карточек</span>
        </div>
      </div>
      <div className="actions result-actions">
        <Link className="button-link" to={repeat}>
          Повторить эту область
        </Link>
        <Link className="button-link secondary-link" to="/app">
          К коллекции
        </Link>
      </div>
      <h2>Ошибки</h2>
      {result.errors.length === 0 ? (
        <div className="state compact success-state">
          Все ответы правильные.
        </div>
      ) : (
        <div className="result-errors">
          {result.errors.map((item) => (
            <article key={item.cardId}>
              <div>
                <span className="card-side-label">Front</span>
                <p>{item.front}</p>
              </div>
              <div>
                <span className="card-side-label">Back</span>
                <p>{item.back}</p>
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
