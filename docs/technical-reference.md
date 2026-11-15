# Техническое описание проекта Resource Battle

## 1. Стек технологий

| Компонент | Технология | Версия |
|-----------|-----------|--------|
| Язык | Go | 1.24 (toolchain 1.24.5) |
| Роутер | go-chi/chi/v5 | — |
| БД | PostgreSQL | 16 |
| Драйвер БД | jackc/pgx/v5 | raw SQL |
| Контейнеризация | Docker + Docker Compose | — |
| Фронтенд | Go templates + CSS + JS | vanilla |

## 2. Конфигурация

Переменные окружения:

| Переменная | Описание | По умолчанию |
|-----------|----------|-------------|
| `PORT` | Порт HTTP-сервера | `8080` |
| `DATABASE_URL` | DSN PostgreSQL | — (обязательна) |

Файл конфигурации: `internal/config/config.go`

## 3. Запуск и сборка

### Локальный запуск
```bash
# Установка зависимостей
go mod download

# Запуск (требуется PostgreSQL на порту 5432)
DATABASE_URL="postgres://resource_battle:resource_battle@localhost:5432/resource_battle" go run ./cmd/server/
```

### Docker Compose
```bash
docker compose up --build
# Приложение: http://localhost:8080
```

### Сборка образа
```bash
docker build -t resource_battle .
```

## 4. База данных

### Connection Pool
Файл: `internal/db/db.go`

Параметры:
- Max lifetime: 30 минут
- Max idle time: 5 минут
- Ping при подключении

### Миграции
Директория: `internal/migrate/migrations/`

Файлы: `NNNN_name.sql` (4 цифры + подчёркивание).
Применяются автоматически при старте через `internal/migrate/migrate.go`.

Текущие миграции (16 штук):
```
001_init.sql                              — начальная схема
002_seed_teams.sql                        — 6 команд по умолчанию
003_seed_rounds.sql                       — 4 раунда по умолчанию
004_tasks_repeatable_and_negative_points  — repeatable, count, отрицательные очки
005_seed_special_tasks                    — "Плюс балл", "Минус балл"
006_settings_and_qrcodes                  — app_settings, qr_codes
007_task_resources                        — ресурсы в задачах
008_add_qrcodes_show_on_home              — show_on_home
009_rename_teams                          — Команда A-F
010_add_qrcodes_updated_at                — updated_at
011_add_task_team_id                      — team_id FK в tasks
012_global_resources                      — global_resources
013_round_resource_requests               — round_resource_requests
014_add_partner3_logo                     — partner3 logo
015_add_main_logo                         — main logo
016_add_server_url                        — server_url в app_settings
```

### Seed-данные
- Команды: `002_seed_teams.sql` — 6 команд (Команда A–F)
- Раунды: `003_seed_rounds.sql` — 4 раунда
- Задачи: `data/tasks_seed.csv` — 62 задачи, загружаются при первом запуске через `internal/app/seed.go`

## 5. Пакеты и модули

### `cmd/server/main.go`
Bootstrap: чтение конфига → пул БД → миграции → seed → роутер → сервер.
Graceful shutdown на SIGINT/SIGTERM.

### `internal/config/config.go`
Чтение ENV-переменных, defaults.

### `internal/db/db.go`
Создание pgxpool, настройка параметров пула.

### `internal/migrate/migrate.go`
- Embed миграций через `//go:embed`
- Парсинг версии из имени файла
- Применение в порядке номеров внутри транзакции
- Таблица `schema_migrations` для трекинга

### `internal/app/app.go`
- Создание `chi.Mux`
- Middleware: `chi.RequestID`, `chi.RealIP`, `chi.Logger`, `chi.Recoverer`
- Регистрация всех маршрутов

### `internal/app/admin.go` (~2116 строк)
Все хендлеры админ-панели. Основные группы:

#### Задачи (Tasks)
```go
func (a *App) handleAdminTasksGET(w http.ResponseWriter, r *http.Request)
func (a *App) handleAdminTasksPOST(w http.ResponseWriter, r *http.Request)
func (a *App) handleAdminTaskUpdate(w http.ResponseWriter, r *http.Request)
func (a *App) handleAdminTaskToggle(w http.ResponseWriter, r *http.Request)
func (a *App) handleTasksImport(w http.ResponseWriter, r *http.Request)
```

#### Раунды (Rounds)
```go
func (a *App) handleAdminRoundsGET(w http.ResponseWriter, r *http.Request)
func (a *App) handleAdminRoundsPOST(w http.ResponseWriter, r *http.Request)
func (a *App) handleStartRound(w http.ResponseWriter, r *http.Request)
func (a *App) handleEndRound(w http.ResponseWriter, r *http.Request)
func (a *App) handleRoundDetail(w http.ResponseWriter, r *http.Request)
func (a *App) handleSaveCompletions(w http.ResponseWriter, r *http.Request)
```

#### Игровой поток (Game Flow)
```go
func (a *App) handleGame(w http.ResponseWriter, r *http.Request)
func (a *App) handleCompleteTask(w http.ResponseWriter, r *http.Request)
func (a *App) handleRepeatableAdjust(w http.ResponseWriter, r *http.Request)
func (a *App) handleDeleteCompletion(w http.ResponseWriter, r *http.Request)
```

#### Настройки (Settings)
```go
func (a *App) handleSettingsGET(w http.ResponseWriter, r *http.Request)
func (a *App) handleUpdateTeams(w http.ResponseWriter, r *http.Request)
func (a *App) handleUploadLogo(w http.ResponseWriter, r *http.Request)
func (a *App) handleDeleteLogo(w http.ResponseWriter, r *http.Request)
func (a *App) handleUploadQR(w http.ResponseWriter, r *http.Request)
func (a *App) handleToggleQRShow(w http.ResponseWriter, r *http.Request)
func (a *App) handleDeleteQR(w http.ResponseWriter, r *http.Request)
func (a *App) handleEditQRGET(w http.ResponseWriter, r *http.Request)
func (a *App) handleEditQRPOST(w http.ResponseWriter, r *http.Request)
func (a *App) handleResetProgress(w http.ResponseWriter, r *http.Request)
```

#### Ресурсы (Resources)
```go
func (a *App) handleResourcesGET(w http.ResponseWriter, r *http.Request)
func (a *App) handleSaveResources(w http.ResponseWriter, r *http.Request)
func (a *App) handleRoundResourcesGET(w http.ResponseWriter, r *http.Request)
func (a *App) handleSaveRoundResources(w http.ResponseWriter, r *http.Request)
```

### `internal/app/dashboard.go`
Запрос табло лидеров:
```sql
SELECT t.name, SUM(tk.points * c.count) as total_points, ...
FROM teams t
LEFT JOIN completions c ON c.team_id = t.id
LEFT JOIN tasks tk ON tk.id = c.task_id
GROUP BY t.id, t.name
ORDER BY total_points DESC
```

### `internal/app/qr.go`
Публичная страница QR-кодов.

### `internal/app/media.go`
Отдача изображений из БД:
- `/media/main-logo` — из `app_settings`
- `/media/partner-logo` — из `app_settings`
- `/media/partner3-logo` — из `app_settings`
- `/media/qr/{id}` — из `qr_codes`

### `internal/app/seed.go`
Загрузка задач из CSV при первом запуске. Дедупликация по `title + points`.

### `internal/app/static.go`
`http.FileServer` для статических файлов.

### `internal/web/web.go`
- Embed шаблонов и статики: `//go:embed templates/ static/`
- Кастомные функции: `checked`, `formatTime`, `derefInt64`

## 6. Модели данных (Go-структуры)

### Team
```go
type Team struct {
    ID       int64     `db:"id"`
    Name     string    `db:"name"`
    CreatedAt time.Time `db:"created_at"`
}
```

### Task
```go
type Task struct {
    ID          int64     `db:"id"`
    Title       string    `db:"title"`
    Description string    `db:"description"`
    Points      int       `db:"points"`
    Devs        int       `db:"devs"`
    Analysts    int       `db:"analysts"`
    DevOps      int       `db:"devops"`
    Designers   int       `db:"designers"`
    Testers     int       `db:"testers"`
    Repeatable  bool      `db:"repeatable"`
    Active      bool      `db:"active"`
    TeamID      *int64    `db:"team_id"`
    CreatedAt   time.Time `db:"created_at"`
    UpdatedAt   time.Time `db:"updated_at"`
}
```

### Round
```go
type Round struct {
    ID        int64      `db:"id"`
    Number    int        `db:"number"`
    Name      string     `db:"name"`
    StartedAt *time.Time `db:"started_at"`
    EndedAt   *time.Time `db:"ended_at"`
    CreatedAt time.Time  `db:"created_at"`
}
```

### Completion
```go
type Completion struct {
    ID          int64     `db:"id"`
    TeamID      int64     `db:"team_id"`
    RoundID     int64     `db:"round_id"`
    TaskID      int64     `db:"task_id"`
    Count       int       `db:"count"`
    CompletedAt time.Time `db:"completed_at"`
}
```

### GlobalResource
```go
type GlobalResource struct {
    ID        int64  `db:"id"`
    Devs      int    `db:"devs"`
    Analysts  int    `db:"analysts"`
    Testers   int    `db:"testers"`
    Designers int    `db:"designers"`
    DevOps    int    `db:"devops"`
    TechLead  int    `db:"techlead"`
}
```

### RoundResourceRequest
```go
type RoundResourceRequest struct {
    RoundID    int64 `db:"round_id"`
    TeamID     int64 `db:"team_id"`
    Devs       int   `db:"devs"`
    Analysts   int   `db:"analysts"`
    Testers    int   `db:"testers"`
    Designers  int   `db:"designers"`
    DevOps     int   `db:"devops"`
    TechLead   int   `db:"techlead"`
}
```

## 7. Шаблоны

Директория: `internal/web/templates/`

| Файл | Описание |
|------|---------|
| `_menu.html` | Навигация, бургер-меню, переключатель темы |
| `dashboard.html` | Табло лидеров, QR-коды по бокам |
| `results.html` | Итоги мероприятия — общий результат всех команд |
| `admin_tasks.html` | CRUD задач, CSV-импорт |
| `admin_rounds.html` | CRUD раундов |
| `admin_round_detail.html` | Чекбоксы выполнений по командам |
| `admin_game.html` | Управление выполнениями по командам |
| `admin_settings.html` | Логотипы, QR-коды, команды, сброс |
| `admin_resources.html` | Глобальные ресурсы |
| `admin_round_resources.html` | Ресурсы раунда |
| `admin_qr_edit.html` | Редактирование QR |
| `qr.html` | Публичные QR-коды |

## 8. Статические файлы

Директория: `internal/web/static/`

| Файл | Описание |
|------|---------|
| `app.css` | Тёмная/светлая тема, CSS custom properties, responsive |
| `app.js` | Theme toggle, burger menu, details toggle |
| `logo/T1.png` | Логотип T1 IT Holding |
| `logo/infotecs.png` | Логотип Infotecs |
| `logo/TN-Digital.png` | Логотип TN Digital |

## 9. Паттерны разработки

### Добавление нового хендлера
1. Добавить метод в `*App` в `internal/app/admin.go` (или вынести в новый файл)
2. Зарегистрировать маршрут в `internal/app/app.go` в функции `setupRoutes()`
3. Добавить шаблон в `internal/web/templates/` (если нужен)
4. Если нужны изменения в БД — добавить миграцию в `internal/migrate/migrations/`

### Добавление миграции
1. Создать файл `internal/migrate/migrations/NNNN_name.sql` (номер > 15)
2. Написать SQL-скрипт (должен быть атомарным — внутри транзакции)
3. Запустить `docker compose up --build` — миграция применится автоматически

### Добавление seed-данных
1. Добавить записи в `data/tasks_seed.csv`
2. Или создать SQL-миграцию с `INSERT`

### Работа с БД
- Использовать `db.Query()` / `db.Exec()` из `*pgxpool.Pool`
- Структуры маппятся через тэги `db:"column_name"`
- NULL-поля — указатели (`*int64`, `*time.Time`)

## 10. Зависимости

```
go.mod:
  github.com/go-chi/chi/v5    — HTTP router
  github.com/jackc/pgx/v5     — PostgreSQL driver
```

## 11. CSS Custom Properties

Основные переменные темы (`app.css`):
```css
:root {
    --bg-primary: #1a1a2e;
    --bg-secondary: #16213e;
    --bg-card: #0f3460;
    --text-primary: #e4e4e4;
    --text-secondary: #a0a0a0;
    --accent: #e94560;
    --border: #2a2a4a;
}
```

Переключение на светлую тему через `data-theme="light"`.

## 13. Версии игры

Система поддерживает две версии правил. База данных общая — различия только в правилах, которые озвучивает ведущий.

### Классика
Стандартная механика с пропорциональным распределением ресурсов.

### Новая версия
Альтернативные правила:
- Пошаговое распределение ресурсов (команда по очереди распределяет все ресурсы).
- Переговоры вдвоём (один торгует, второй бегает к другим командам).
- Джокеры с отложенным эффектом (действуют со следующего спринта).
- Убеждение команды-распределителя через дискуссию капитанов.

Страница результатов (`/results`) доступна для обеих версий.
Описания результатов генерируются функцией `generateResultsDescription()` в `dashboard.go` на основе общего балла.

## 14. Известные ограничения

1. **admin.go** — монолитный файл (~2116 строк), рекомендуется рефакторинг по хендлерам
2. **Нет аутентификации** — админ-панель доступна всем, кто имеет доступ к серверу
3. **Без ORM** — все запросы написаны на raw SQL
4. **Без валидации** — минимум валидации входных данных
5. **Без тестов** — unit/integration tests отсутствуют
6. **CSV-seed** — дедупликация по `title + points`, не по `title`
7. **Placeholder-картинки** — папка `static/results/` содержит placeholder-файлы, нужно заменить реальными изображениями
