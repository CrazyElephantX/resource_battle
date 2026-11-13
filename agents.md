# Agents Resource Battle

## Документация

### Архитектура
- **docs/architecture.md** — общая архитектура проекта: структура, схема БД, маршруты, бизнес-логика, диаграммы

### Техническая документация
- **docs/technical-reference.md** — техническое описание для задач по доработке: стек, конфиги, запуск, пакеты, модели данных, паттерны разработки

## Структура документации

```
docs/
├── architecture.md           — Архитектура проекта
└── technical-reference.md    — Техническая справка для разработчиков
```

## Ключевые файлы проекта

| Что | Путь |
|-----|------|
| Точка входа | `cmd/server/main.go` |
| Роутер | `internal/app/app.go` |
| Админ-хендлеры | `internal/app/admin.go` |
| Табло лидеров | `internal/app/dashboard.go` |
| Конфигурация | `internal/config/config.go` |
| Подключение к БД | `internal/db/db.go` |
| Runner миграций | `internal/migrate/migrate.go` |
| Миграции | `internal/migrate/migrations/` |
| Шаблоны | `internal/web/templates/` |
| Статика | `internal/web/static/` |
| Seed-данные задач | `data/tasks_seed.csv` |
| Docker Compose | `docker-compose.yml` |
| Dockerfile | `Dockerfile` |
| README | `README.md` |

## Быстрый справочник

- **Добавить маршрут:** `internal/app/app.go` → `setupRoutes()` + `internal/app/admin.go`
- **Добавить миграцию:** `internal/migrate/migrations/NNNN_name.sql`
- **Добавить шаблон:** `internal/web/templates/`
- **Добавить задачу в seed:** `data/tasks_seed.csv`
