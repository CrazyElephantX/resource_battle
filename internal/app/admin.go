package app

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type adminTask struct {
	ID          int64
	Title       string
	Description string
	Points      int
	Repeatable  bool
	Active      bool
	CreatedAt   time.Time
}

type adminRound struct {
	ID        int64
	Number    int
	Name      string
	StartedAt *time.Time
	EndedAt   *time.Time
	CreatedAt time.Time
}

type adminTeam struct {
	ID   int64
	Name string
}

type adminTasksPageData struct {
	Tasks   []adminTask
	FlashOK string
	FlashErr string
}

type adminRoundsPageData struct {
	Rounds   []adminRound
	FlashOK  string
	FlashErr string
}

type adminRoundDetailData struct {
	Round      adminRound
	Teams      []adminTeam
	Tasks      []adminTask
	Checked    map[int64]map[int64]bool // teamID -> taskID -> bool
	FlashOK    string
	FlashErr   string
}

type completionPair struct{ teamID, taskID int64 }

type adminQRItem struct {
	ID          int64
	Kind        string
	KindLabel   string
	Title       string
	Description string
}

type adminSettingsPageData struct {
	QRCodes []adminQRItem
	FlashOK string
	FlashErr string
}

func mountAdmin(r chi.Router, pool *pgxpool.Pool, tpl *template.Template) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/tasks", http.StatusFound)
	})

	r.Get("/tasks", adminTasksGet(pool, tpl))
	r.Post("/tasks", adminTasksCreate(pool))
	r.Post("/tasks/{taskID}", adminTasksUpdate(pool))
	r.Post("/tasks/{taskID}/toggle", adminTasksToggle(pool))

	r.Get("/rounds", adminRoundsGet(pool, tpl))
	r.Post("/rounds", adminRoundsCreate(pool))
	r.Post("/rounds/{roundID}/start", adminRoundStart(pool))
	r.Post("/rounds/{roundID}/end", adminRoundEnd(pool))

	r.Get("/rounds/{roundID}", adminRoundDetailGet(pool, tpl))
	r.Post("/rounds/{roundID}/completions", adminRoundDetailPost(pool))

	r.Get("/game", adminGameGet(pool, tpl))
	r.Post("/game/complete", adminGameComplete(pool))
	r.Post("/game/repeatable", adminGameRepeatable(pool))
	r.Post("/game/delete", adminGameDelete(pool))

	r.Get("/settings", adminSettingsGet(pool, tpl))
	r.Post("/settings/logo", adminSettingsLogo(pool))
	r.Post("/settings/qr", adminSettingsQR(pool))
}

func renderAdmin(tpl *template.Template, w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tpl.ExecuteTemplate(w, name, data)
}

func flashFromQuery(r *http.Request) (ok, err string) {
	q := r.URL.Query()
	ok = q.Get("ok")
	err = q.Get("err")
	return ok, err
}

func adminTasksGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tasks, err := listTasks(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ok, e := flashFromQuery(r)
		renderAdmin(tpl, w, "admin_tasks.html", adminTasksPageData{Tasks: tasks, FlashOK: ok, FlashErr: e})
	}
}

func adminTasksCreate(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/tasks?err=bad+form", http.StatusFound)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		desc := strings.TrimSpace(r.FormValue("description"))
		points, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("points")))
		if title == "" || points < -100000 || points > 100000 {
			http.Redirect(w, r, "/admin/tasks?err=invalid+task", http.StatusFound)
			return
		}
		if err := createTask(r.Context(), pool, title, desc, points); err != nil {
			http.Redirect(w, r, "/admin/tasks?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/tasks?ok=saved", http.StatusFound)
	}
}

func adminTasksUpdate(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID, err := parseID(chi.URLParam(r, "taskID"))
		if err != nil {
			http.Redirect(w, r, "/admin/tasks?err=bad+task+id", http.StatusFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/tasks?err=bad+form", http.StatusFound)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		desc := strings.TrimSpace(r.FormValue("description"))
		points, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("points")))
		if title == "" || points < -100000 || points > 100000 {
			http.Redirect(w, r, "/admin/tasks?err=invalid+task", http.StatusFound)
			return
		}
		if err := updateTask(r.Context(), pool, taskID, title, desc, points); err != nil {
			http.Redirect(w, r, "/admin/tasks?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/tasks?ok=saved", http.StatusFound)
	}
}

func adminTasksToggle(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID, err := parseID(chi.URLParam(r, "taskID"))
		if err != nil {
			http.Redirect(w, r, "/admin/tasks?err=bad+task+id", http.StatusFound)
			return
		}
		if err := toggleTask(r.Context(), pool, taskID); err != nil {
			http.Redirect(w, r, "/admin/tasks?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/tasks?ok=saved", http.StatusFound)
	}
}

func adminRoundsGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rounds, err := listRounds(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ok, e := flashFromQuery(r)
		renderAdmin(tpl, w, "admin_rounds.html", adminRoundsPageData{Rounds: rounds, FlashOK: ok, FlashErr: e})
	}
}

func adminRoundsCreate(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/rounds?err=bad+form", http.StatusFound)
			return
		}
		number, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("number")))
		name := strings.TrimSpace(r.FormValue("name"))
		if number <= 0 {
			http.Redirect(w, r, "/admin/rounds?err=invalid+round", http.StatusFound)
			return
		}
		if err := createRound(r.Context(), pool, number, name); err != nil {
			http.Redirect(w, r, "/admin/rounds?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/rounds?ok=saved", http.StatusFound)
	}
}

func adminRoundStart(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roundID, err := parseID(chi.URLParam(r, "roundID"))
		if err != nil {
			http.Redirect(w, r, "/admin/rounds?err=bad+round+id", http.StatusFound)
			return
		}
		if err := setRoundTimestamp(r.Context(), pool, roundID, "started_at"); err != nil {
			http.Redirect(w, r, "/admin/rounds?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/rounds?ok=saved", http.StatusFound)
	}
}

func adminRoundEnd(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roundID, err := parseID(chi.URLParam(r, "roundID"))
		if err != nil {
			http.Redirect(w, r, "/admin/rounds?err=bad+round+id", http.StatusFound)
			return
		}
		if err := setRoundTimestamp(r.Context(), pool, roundID, "ended_at"); err != nil {
			http.Redirect(w, r, "/admin/rounds?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/rounds?ok=saved", http.StatusFound)
	}
}

func adminRoundDetailGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roundID, err := parseID(chi.URLParam(r, "roundID"))
		if err != nil {
			http.Error(w, "bad round id", http.StatusBadRequest)
			return
		}
		round, err := getRound(r.Context(), pool, roundID)
		if err != nil {
			if isNotFound(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		teams, err := listTeams(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Round matrix is for one-time tasks only (repeatable tasks are managed in /admin/game).
		tasks, err := listTasksByRepeatable(r.Context(), pool, false)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		checked, err := listCompletionsForRound(r.Context(), pool, roundID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ok, e := flashFromQuery(r)
		renderAdmin(tpl, w, "admin_round_detail.html", adminRoundDetailData{
			Round:    round,
			Teams:    teams,
			Tasks:    tasks,
			Checked:  checked,
			FlashOK:  ok,
			FlashErr: e,
		})
	}
}

func adminRoundDetailPost(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roundID, err := parseID(chi.URLParam(r, "roundID"))
		if err != nil {
			http.Error(w, "bad round id", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d?err=bad+form", roundID), http.StatusFound)
			return
		}

		// Parse all checkboxes with name c_<teamID>_<taskID>
		var pairs []completionPair
		for key := range r.PostForm {
			if !strings.HasPrefix(key, "c_") {
				continue
			}
			parts := strings.Split(key, "_")
			if len(parts) != 3 {
				continue
			}
			teamID, err1 := parseID(parts[1])
			taskID, err2 := parseID(parts[2])
			if err1 != nil || err2 != nil {
				continue
			}
			pairs = append(pairs, completionPair{teamID: teamID, taskID: taskID})
		}

		// Filter out repeatable tasks if any were sent (defense-in-depth).
		pairs, err = filterPairsNonRepeatable(r.Context(), pool, pairs)
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d?err=save+failed", roundID), http.StatusFound)
			return
		}

		// Sort for stable inserts.
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].teamID == pairs[j].teamID {
				return pairs[i].taskID < pairs[j].taskID
			}
			return pairs[i].teamID < pairs[j].teamID
		})

		if err := replaceCompletionsForRound(r.Context(), pool, roundID, pairs); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d?err=save+failed", roundID), http.StatusFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d?ok=saved", roundID), http.StatusFound)
	}
}

func filterPairsNonRepeatable(ctx context.Context, pool *pgxpool.Pool, pairs []completionPair) ([]completionPair, error) {
	if len(pairs) == 0 {
		return pairs, nil
	}
	out := make([]completionPair, 0, len(pairs))
	for _, p := range pairs {
		rep, err := isTaskRepeatable(ctx, pool, p.taskID)
		if err != nil {
			return nil, err
		}
		if rep {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

// --- DB helpers ---

func listTeams(ctx context.Context, pool *pgxpool.Pool) ([]adminTeam, error) {
	rows, err := pool.Query(ctx, `SELECT id, name FROM teams ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTeam
	for rows.Next() {
		var t adminTeam
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listTasks(ctx context.Context, pool *pgxpool.Pool) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points, repeatable, active, created_at
FROM tasks
ORDER BY active DESC, created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTask
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points, &t.Repeatable, &t.Active, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listTasksByRepeatable(ctx context.Context, pool *pgxpool.Pool, repeatable bool) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points, repeatable, active, created_at
FROM tasks
WHERE repeatable = $1
ORDER BY active DESC, created_at ASC, id ASC`, repeatable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTask
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points, &t.Repeatable, &t.Active, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listActiveTasksByRepeatable(ctx context.Context, pool *pgxpool.Pool, repeatable bool) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points, repeatable, active, created_at
FROM tasks
WHERE repeatable = $1 AND active = true
ORDER BY created_at ASC, id ASC`, repeatable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTask
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points, &t.Repeatable, &t.Active, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func createTask(ctx context.Context, pool *pgxpool.Pool, title, description string, points int) error {
	_, err := pool.Exec(ctx, `
INSERT INTO tasks(title, description, points)
VALUES ($1, $2, $3)`, title, description, points)
	return err
}

func updateTask(ctx context.Context, pool *pgxpool.Pool, id int64, title, description string, points int) error {
	ct, err := pool.Exec(ctx, `
UPDATE tasks
SET title=$2, description=$3, points=$4, updated_at=now()
WHERE id=$1`, id, title, description, points)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errNotFound("task")
	}
	return nil
}

func toggleTask(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	ct, err := pool.Exec(ctx, `
UPDATE tasks
SET active = NOT active, updated_at=now()
WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errNotFound("task")
	}
	return nil
}

func listRounds(ctx context.Context, pool *pgxpool.Pool) ([]adminRound, error) {
	rows, err := pool.Query(ctx, `
SELECT id, number, name, started_at, ended_at, created_at
FROM rounds
ORDER BY number ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminRound
	for rows.Next() {
		var rd adminRound
		if err := rows.Scan(&rd.ID, &rd.Number, &rd.Name, &rd.StartedAt, &rd.EndedAt, &rd.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

func createRound(ctx context.Context, pool *pgxpool.Pool, number int, name string) error {
	_, err := pool.Exec(ctx, `
INSERT INTO rounds(number, name)
VALUES ($1, $2)`, number, name)
	return err
}

func getRound(ctx context.Context, pool *pgxpool.Pool, id int64) (adminRound, error) {
	var rd adminRound
	err := pool.QueryRow(ctx, `
SELECT id, number, name, started_at, ended_at, created_at
FROM rounds
WHERE id=$1`, id).Scan(&rd.ID, &rd.Number, &rd.Name, &rd.StartedAt, &rd.EndedAt, &rd.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return adminRound{}, errNotFound("round")
		}
		return adminRound{}, err
	}
	return rd, nil
}

func setRoundTimestamp(ctx context.Context, pool *pgxpool.Pool, id int64, col string) error {
	if col != "started_at" && col != "ended_at" {
		return fmt.Errorf("invalid column")
	}
	ct, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE rounds SET %s=COALESCE(%s, now()) WHERE id=$1`, col, col), id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errNotFound("round")
	}
	return nil
}

func listCompletionsForRound(ctx context.Context, pool *pgxpool.Pool, roundID int64) (map[int64]map[int64]bool, error) {
	rows, err := pool.Query(ctx, `SELECT team_id, task_id FROM completions WHERE round_id=$1`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]map[int64]bool)
	for rows.Next() {
		var teamID, taskID int64
		if err := rows.Scan(&teamID, &taskID); err != nil {
			return nil, err
		}
		m, ok := out[teamID]
		if !ok {
			m = make(map[int64]bool)
			out[teamID] = m
		}
		m[taskID] = true
	}
	return out, rows.Err()
}

func replaceCompletionsForRound(ctx context.Context, pool *pgxpool.Pool, roundID int64, pairs []completionPair) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// This endpoint manages only one-time tasks, keep repeatable completions untouched.
	if _, err := tx.Exec(ctx, `
DELETE FROM completions c
USING tasks t
WHERE c.task_id = t.id
  AND t.repeatable = false
  AND c.round_id = $1`, roundID); err != nil {
		return err
	}
	if len(pairs) > 0 {
		batch := &pgx.Batch{}
		for _, p := range pairs {
			// Enforce "one-time tasks can be completed only once" by removing completion in other rounds.
			batch.Queue(`
DELETE FROM completions c
USING tasks t
WHERE c.task_id = t.id
  AND t.repeatable = false
  AND c.team_id = $1
  AND c.task_id = $2`, p.teamID, p.taskID)
			batch.Queue(`INSERT INTO completions(team_id, round_id, task_id, count) VALUES ($1,$2,$3,1) ON CONFLICT (team_id, round_id, task_id) DO UPDATE SET count=1, completed_at=now()`, p.teamID, roundID, p.taskID)
		}
		br := tx.SendBatch(ctx, batch)
		for range pairs {
			// delete
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return err
			}
			// insert
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return err
			}
		}
		if err := br.Close(); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// --- Game flow page (/admin/game) ---

type adminCompletionRow struct {
	ID             int64
	RoundID        int64
	RoundName      string
	TaskID         int64
	TaskTitle      string
	TaskPoints     int
	TaskRepeatable bool
	Count          int
	TotalPoints    int
}

type adminGamePageData struct {
	Teams          []adminTeam
	TeamID         int64
	Rounds         []adminRound
	OneTimeTasks   []adminTask
	RepeatableTasks []adminTask
	Completions    []adminCompletionRow
	FlashOK        string
	FlashErr       string
}

func adminGameGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teams, err := listTeams(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ok, e := flashFromQuery(r)

		teamIDStr := strings.TrimSpace(r.URL.Query().Get("team"))
		teamID := int64(0)
		if teamIDStr != "" {
			if id, err := parseID(teamIDStr); err == nil {
				teamID = id
			}
		}

		data := adminGamePageData{
			Teams:   teams,
			TeamID:  teamID,
			FlashOK: ok,
			FlashErr: e,
		}

		if teamID > 0 {
			rounds, err := listRounds(r.Context(), pool)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			oneTime, err := listActiveTasksByRepeatable(r.Context(), pool, false)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			repeatable, err := listActiveTasksByRepeatable(r.Context(), pool, true)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			comps, err := listCompletionsForTeam(r.Context(), pool, teamID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			data.Rounds = rounds
			data.OneTimeTasks = oneTime
			data.RepeatableTasks = repeatable
			data.Completions = comps
		}

		renderAdmin(tpl, w, "admin_game.html", data)
	}
}

func adminGameComplete(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/game?err=bad+form", http.StatusFound)
			return
		}
		teamID, err1 := parseID(r.FormValue("team_id"))
		taskID, err2 := parseID(r.FormValue("task_id"))
		roundID, err3 := parseID(r.FormValue("round_id"))
		if err1 != nil || err2 != nil || err3 != nil {
			http.Redirect(w, r, "/admin/game?err=invalid+input", http.StatusFound)
			return
		}
		rep, err := isTaskRepeatable(r.Context(), pool, taskID)
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=save+failed", teamID), http.StatusFound)
			return
		}
		if rep {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=task+is+repeatable", teamID), http.StatusFound)
			return
		}
		if err := setOneTimeCompletion(r.Context(), pool, teamID, taskID, roundID); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=save+failed", teamID), http.StatusFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&ok=saved", teamID), http.StatusFound)
	}
}

func adminGameRepeatable(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/game?err=bad+form", http.StatusFound)
			return
		}
		teamID, err1 := parseID(r.FormValue("team_id"))
		taskID, err2 := parseID(r.FormValue("task_id"))
		roundID, err3 := parseID(r.FormValue("round_id"))
		delta, err4 := strconv.Atoi(strings.TrimSpace(r.FormValue("delta")))
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || delta == 0 {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=invalid+input", teamID), http.StatusFound)
			return
		}
		rep, err := isTaskRepeatable(r.Context(), pool, taskID)
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=save+failed", teamID), http.StatusFound)
			return
		}
		if !rep {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=task+is+not+repeatable", teamID), http.StatusFound)
			return
		}
		if err := adjustRepeatableCompletion(r.Context(), pool, teamID, taskID, roundID, delta); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=save+failed", teamID), http.StatusFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&ok=saved", teamID), http.StatusFound)
	}
}

func adminGameDelete(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/game?err=bad+form", http.StatusFound)
			return
		}
		teamID, err1 := parseID(r.FormValue("team_id"))
		compID, err2 := parseID(r.FormValue("completion_id"))
		if err1 != nil || err2 != nil {
			http.Redirect(w, r, "/admin/game?err=invalid+input", http.StatusFound)
			return
		}
		_, _ = pool.Exec(r.Context(), `DELETE FROM completions WHERE id=$1 AND team_id=$2`, compID, teamID)
		http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&ok=saved", teamID), http.StatusFound)
	}
}

func listCompletionsForTeam(ctx context.Context, pool *pgxpool.Pool, teamID int64) ([]adminCompletionRow, error) {
	rows, err := pool.Query(ctx, `
SELECT
  c.id,
  c.round_id,
  r.name,
  c.task_id,
  t.title,
  t.points,
  t.repeatable,
  c.count
FROM completions c
JOIN rounds r ON r.id = c.round_id
JOIN tasks t ON t.id = c.task_id
WHERE c.team_id = $1
ORDER BY r.number ASC, t.repeatable ASC, t.title ASC, c.id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminCompletionRow
	for rows.Next() {
		var x adminCompletionRow
		if err := rows.Scan(&x.ID, &x.RoundID, &x.RoundName, &x.TaskID, &x.TaskTitle, &x.TaskPoints, &x.TaskRepeatable, &x.Count); err != nil {
			return nil, err
		}
		x.TotalPoints = x.TaskPoints * x.Count
		out = append(out, x)
	}
	return out, rows.Err()
}

func isTaskRepeatable(ctx context.Context, pool *pgxpool.Pool, taskID int64) (bool, error) {
	var rep bool
	err := pool.QueryRow(ctx, `SELECT repeatable FROM tasks WHERE id=$1`, taskID).Scan(&rep)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, errNotFound("task")
		}
		return false, err
	}
	return rep, nil
}

func setOneTimeCompletion(ctx context.Context, pool *pgxpool.Pool, teamID, taskID, roundID int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Remove completion in any round, then set it in the selected round.
	if _, err := tx.Exec(ctx, `
DELETE FROM completions c
USING tasks t
WHERE c.task_id=t.id
  AND t.repeatable=false
  AND c.team_id=$1
  AND c.task_id=$2`, teamID, taskID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO completions(team_id, round_id, task_id, count)
VALUES ($1,$2,$3,1)
ON CONFLICT (team_id, round_id, task_id) DO UPDATE
SET count=1, completed_at=now()`, teamID, roundID, taskID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func adjustRepeatableCompletion(ctx context.Context, pool *pgxpool.Pool, teamID, taskID, roundID int64, delta int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if delta > 0 {
		if _, err := tx.Exec(ctx, `
INSERT INTO completions(team_id, round_id, task_id, count)
VALUES ($1,$2,$3,$4)
ON CONFLICT (team_id, round_id, task_id) DO UPDATE
SET count = completions.count + EXCLUDED.count,
    completed_at=now()`, teamID, roundID, taskID, delta); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// delta < 0
	var newCount int
	err = tx.QueryRow(ctx, `
UPDATE completions
SET count = count + $4,
    completed_at=now()
WHERE team_id=$1 AND round_id=$2 AND task_id=$3
RETURNING count`, teamID, roundID, taskID, delta).Scan(&newCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return tx.Commit(ctx)
		}
		return err
	}
	if newCount <= 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM completions WHERE team_id=$1 AND round_id=$2 AND task_id=$3`, teamID, roundID, taskID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// --- Settings (partner logo + QR codes) ---

func adminSettingsGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, e := flashFromQuery(r)
		items, err := listAllQRCodes(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		renderAdmin(tpl, w, "admin_settings.html", adminSettingsPageData{
			QRCodes:  items,
			FlashOK:  ok,
			FlashErr: e,
		})
	}
}

func adminSettingsLogo(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil { // 1 MiB
			http.Redirect(w, r, "/admin/settings?err=bad+form", http.StatusFound)
			return
		}
		filename, mime, data, err := readUploadedImage(r, "logo", 512*1024)
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+file", http.StatusFound)
			return
		}
		_, err = pool.Exec(r.Context(), `
INSERT INTO app_settings(id, partner_logo_filename, partner_logo_mime, partner_logo_data)
VALUES (1,$1,$2,$3)
ON CONFLICT (id) DO UPDATE
SET partner_logo_filename=$1,
    partner_logo_mime=$2,
    partner_logo_data=$3,
    updated_at=now()`, filename, mime, data)
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/settings?ok=logo+updated", http.StatusFound)
	}
}

func adminSettingsQR(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+form", http.StatusFound)
			return
		}
		kind := strings.TrimSpace(r.FormValue("kind"))
		if kind != "author" && kind != "partner" {
			http.Redirect(w, r, "/admin/settings?err=bad+kind", http.StatusFound)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		desc := strings.TrimSpace(r.FormValue("description"))
		if title == "" {
			http.Redirect(w, r, "/admin/settings?err=bad+title", http.StatusFound)
			return
		}
		filename, mime, data, err := readUploadedImage(r, "image", 512*1024)
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+file", http.StatusFound)
			return
		}
		_, err = pool.Exec(r.Context(), `
INSERT INTO qr_codes(kind, title, description, image_filename, image_mime, image_data)
VALUES ($1,$2,$3,$4,$5,$6)`, kind, title, desc, filename, mime, data)
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/settings?ok=qr+added", http.StatusFound)
	}
}

func listAllQRCodes(ctx context.Context, pool *pgxpool.Pool) ([]adminQRItem, error) {
	rows, err := pool.Query(ctx, `
SELECT id, kind, title, description
FROM qr_codes
WHERE active=true
ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminQRItem
	for rows.Next() {
		var it adminQRItem
		if err := rows.Scan(&it.ID, &it.Kind, &it.Title, &it.Description); err != nil {
			return nil, err
		}
		if it.Kind == "author" {
			it.KindLabel = "Автор"
		} else {
			it.KindLabel = "Партнёр"
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func readUploadedImage(r *http.Request, field string, maxBytes int64) (filename, mime string, data []byte, err error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", "", nil, err
	}
	defer file.Close()
	if maxBytes <= 0 {
		maxBytes = 512 * 1024
	}
	limited := io.LimitReader(file, maxBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return "", "", nil, err
	}
	if int64(len(b)) > maxBytes {
		return "", "", nil, fmt.Errorf("file too large")
	}
	mime = header.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/png"
	}
	filename = header.Filename
	return filename, mime, b, nil
}

