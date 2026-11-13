# Архитектура проекта Resource Battle

## 1. Обзор

**Resource Battle** — веб-приложение (дашборд и админ-панель) для IT-командной настольной игры «Ресурсный батл». Приложение управляет раундами, задачами, очками команд, распределением ресурсов и отображает публичное табло лидеров.

- **Язык:** Go 1.24
- **Роутер:** go-chi/chi/v5
- **База данных:** PostgreSQL 16
- **Драйвер:** jackc/pgx/v5 (raw SQL, без ORM)
- **Фронтенд:** Server-side rendering (Go templates), vanilla CSS/JS
- **Деплой:** Docker Compose

## 2. Структура проекта

```
resource_battle/
├── cmd/server/main.go              # Точка входа, bootstrap
├── data/
│   └── tasks_seed.csv              # CSV-данные для начальной загрузки задач (62 шт.)
├── internal/
│   ├── app/
│   │   ├── app.go                  # Настройка роутера Chi, регистрация маршрутов
│   │   ├── admin.go                # Все хендлеры админ-панели (~2116 строк)
│   │   ├── dashboard.go            # Запрос табло лидеров, API статистики команд
│   │   ├── qr.go                   # Публичная страница QR-кодов
│   │   ├── media.go                # Хендлеры для отдачи медиа (логотипы, QR)
│   │   ├── seed.go                 # Начальная загрузка задач из CSV
│   │   └── static.go               # Настройка отдачи статических файлов
│   ├── config/
│   │   └── config.go               # Конфигурация из ENV (PORT, DATABASE_URL)
│   ├── db/
│   │   └── db.go                   # Connection pool (pgxpool)
│   ├── migrate/
│   │   ├── migrate.go              # Runner миграций (embedded SQL)
│   │   └── migrations/             # 15 SQL-миграций
│   └── web/
│       ├── web.go                  # Embed templates/static, кастомные функции шаблонов
│       ├── templates/              # 10 HTML-шаблонов
│       └── static/                 # CSS, JS, логотипы
├── docker-compose.yml              # Службы: db + app
├── Dockerfile                      # Multi-stage build
├── go.mod                          # Модуль: resource_battle
└── README.md                       # Документация для пользователей
```

## 3. Диаграмма пакетов

```
cmd/server/main.go
    │
    ├─► internal/config      (PORT, DATABASE_URL)
    ├─► internal/db          (pgxpool)
    ├─► internal/migrate     (применение миграций)
    ├─► internal/app         (HTTP-хендлеры + роутер)
    │       │
    │       ├─► admin.go     (CRUD задач, раундов, настроек)
    │       ├─► dashboard.go (табло лидеров)
    │       ├─► qr.go        (QR-коды)
    │       ├─► media.go     (отдача изображений)
    │       ├─► seed.go      (задача из CSV)
    │       └─► static.go    (статика)
    └─► internal/web         (templates + static embed)
```

## 4. Схема базы данных

### Основные таблицы

```
┌──────────┐       ┌─────────────┐       ┌──────────┐
│  teams   │       │   rounds    │       │  tasks   │
├──────────┤       ├─────────────┤       ├──────────┤
│ id BIGSER│       │ id BIGSER   │       │ id BIGSER│
│ name TEXT│       │ number INT  │       │ title    │
│ created  │       │ name TEXT   │       │ desc     │
└────┬─────┘       │ started_at  │       │ points   │
     │             │ ended_at    │       │ devs     │
     │             └──────┬──────┘       │ analysts │
     │                    │              │ devops   │
     │                    │              │ designers│
     │              ┌─────┴──────┐       │ testers  │
     │              │ completions│       │ repeat   │
     │              ├────────────┤       └────┬─────┘
     │              │ id BIGSER  │            │
     │              │ team_id FK │            │
     │              │ round_id FK│            │
     │              │ task_id FK │            │
     │              │ count INT  │            │
     │              └────────────┘            │
     │                    ▲                   │
     └────────────────────┼───────────────────┘
                          │
                    task_id (nullable FK)
                    для привязки задачи к команде
```

### Таблицы настроек

```
┌──────────────────┐   ┌──────────────┐   ┌───────────────────┐
│  app_settings    │   │  qr_codes    │   │ global_resources  │
├──────────────────┤   ├──────────────┤   ├───────────────────┤
│ main_logo_*      │   │ id BIGSER    │   │ id BIGSER         │
│ partner_logo_*   │   │ kind TEXT    │   │ devs              │
│ partner3_logo_*  │   │ title        │   │ analysts          │
│ (filename/mime/  │   │ description  │   │ testers           │
 │  data BYTEA)     │   │ image_data   │   │ designers         │
└──────────────────┘   │ show_on_home │   │ devops            │
                       └──────────────┘   │ techlead          │
                                            └───────────────────┘
┌──────────────────────────┐
│ round_resource_requests  │
├──────────────────────────┤
│ round_id FK              │
│ team_id FK               │
│ devs / analysts / ...    │
│ (unique: round_id,team_id)│
└──────────────────────────┘
```

_* — колонки: *_filename, *_mime, *_data_

## 5. Маршруты (Routes)

### Публичные маршруты

| Method | Path | Описание |
|--------|------|----------|
| GET | `/` | Табло лидеров + QR-коды |
| GET | `/healthz` | Health check |
| GET | `/qr` | Страница QR-кодов |
| GET | `/static/*` | Статические файлы |
| GET | `/media/main-logo` | Главный логотип |
| GET | `/media/partner-logo` | Логотип партнёра |
| GET | `/media/partner3-logo` | Логотип партнёра 3 |
| GET | `/media/qr/{id}` | QR-изображение |

### Админ-маршруты (prefix `/admin`)

| Method | Path | Описание |
|--------|------|----------|
| GET | `/admin/` | Редирект на `/admin/tasks` |
| GET/POST | `/admin/tasks` | CRUD задач + CSV импорт |
| POST | `/admin/tasks/{id}/toggle` | Активировать/деактивировать задачу |
| GET | `/admin/rounds` | CRUD раундов |
| POST | `/admin/rounds/{id}/start` | Начать раунд |
| POST | `/admin/rounds/{id}/end` | Завершить раунд |
| GET | `/admin/rounds/{id}` | Детали раунда (чекбоксы выполнений) |
| POST | `/admin/rounds/{id}/completions` | Сохранить выполнения |
| GET | `/admin/game` | Управление выполнениями по командам |
| POST | `/admin/game/complete` | Отметить задачу выполненной |
| POST | `/admin/game/repeatable` | Изменить счётчик повторяемой задачи |
| POST | `/admin/game/delete` | Удалить запись о выполнении |
| GET | `/admin/settings` | Настройки (логотипы, QR, имена команд) |
| POST | `/admin/settings/*` | Операции с настройками |
| GET | `/admin/resources` | Глобальные ресурсы |
| POST | `/admin/resources` | Сохранить глобальные ресурсы |
| GET/POST | `/admin/rounds/{id}/resources` | Ресурсы раунда |

## 6. Бизнес-логика

### Подсчёт очков
```
total_points = Σ(task.points × completion.count)
```
Суммируется по всем раундам и всем задачам.

### Повторяемые vs одноразовые задачи
- **Одноразовые:** Могут быть выполнены одной командой один раз globally.
- **Повторяемые (repeatable):** Могут быть выполнены многократно, счётчик регулируется.

### Распределение ресурсов
При суммарных запросах команд > доступных ресурсов — применяется пропорциональное распределение с округлением вниз (floor).

### Привязка задач к команде
Задачи могут иметь `team_id` (FK) — тогда они видны только привязанной команде в игровом потоке. NULL = общая задача.

## 7. Поток данных (Request Flow)

```
HTTP Request
    │
    ▼
Chi Router (internal/app/app.go)
    │
    ├─► Middleware: RequestID, RealIP, Logger, Recoverer
    │
    ▼
Handler (admin.go / dashboard.go / qr.go / media.go)
    │
    ▼
pgx Query/Exec (direct SQL)
    │
    ▼
PostgreSQL
    │
    ▼
Response: template.Execute (html/template) or JSON or Image
```

## 8. Миграции

Миграции хранятся в `internal/migrate/migrations/`, пронумерованы префиксом `NNNN_`.
При запуске `main.go` автоматически применяет неприменённые миграции в порядке номеров через `migrate.go`.

Текущая версия: **015** (add_main_logo)

## 9. Шаблоны (Templates)

| Шаблон | Назначение |
|--------|-----------|
| `_menu.html` | Навигация, переключатель темы, бургер-меню |
| `dashboard.html` | Публичное табло лидеров |
| `admin_tasks.html` | Управление задачами |
| `admin_rounds.html` | Управление раундами |
| `admin_round_detail.html` | Детали раунда |
| `admin_game.html` | Игровой поток |
| `admin_settings.html` | Настройки |
| `admin_resources.html` | Глобальные ресурсы |
| `admin_round_resources.html` | Ресурсы раунда |
| `admin_qr_edit.html` | Редактирование QR |
| `qr.html` | Публичные QR-коды |
