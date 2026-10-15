import { Link, useParams } from "react-router-dom";

export function StudySessionStart() {
  const { id } = useParams();
  return (
    <section className="content narrow">
      <p className="eyebrow">Сессия готова</p>
      <h1>Раунд сформирован</h1>
      <p className="subtitle">Карточки и их порядок зафиксированы.</p>
      <Link className="button-link secondary-link" to="/app">
        К коллекции
      </Link>
      <span className="visually-hidden">{id}</span>
    </section>
  );
}
