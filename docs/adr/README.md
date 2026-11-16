# Architecture Decision Records

В этой папке хранится история значимых архитектурных решений. Правила создания и замены ADR описаны в [DEVELOPMENT.md](../DEVELOPMENT.md).

## Реестр

| Номер | Решение | Статус | Заменён |
|---|---|---|---|
| [ADR-0001](0001-modular-monolith.md) | Модульный монолит | Accepted | — |
| [ADR-0002](0002-postgresql-auth-sessions.md) | Сессии аутентификации в PostgreSQL | Accepted | — |
| [ADR-0003](0003-mvp-review-scheduler.md) | Бинарный планировщик повторений MVP | Accepted | — |

## Шаблон

```markdown
# ADR-NNNN: Название решения

Status: Proposed
Date: YYYY-MM-DD

## Контекст

## Решение

## Рассмотренные варианты

## Последствия

## Условия пересмотра
```

Допустимые статусы: `Proposed`, `Accepted`, `Rejected`, `Deprecated` и `Superseded by ADR-NNNN`. После `Accepted` смысл документа не переписывается.
