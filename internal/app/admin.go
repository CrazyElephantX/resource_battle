package app

import (
	"context"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
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
	Devs        int
	Analysts    int
	DevOps      int
	Designers   int
	Testers     int
	Repeatable  bool
	Active      bool
	TeamID      *int64
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
	Tasks    []adminTask
	Teams    []adminTeam
	FlashOK  string
	FlashErr string
}

type adminRoundsPageData struct {
	Rounds   []adminRound
	FlashOK  string
	FlashErr string
}

type adminRoundDetailData struct {
	Round    adminRound
	Teams    []adminTeam
	Tasks    []adminTask
	Checked  map[int64]map[int64]bool // teamID -> taskID -> bool
	FlashOK  string
	FlashErr string
}

type completionPair struct{ teamID, taskID int64 }

type adminQRItem struct {
	ID          int64
	Kind        string
	KindLabel   string
	Title       string
	Description string
	ShowOnHome  bool
}

type adminSettingsPageData struct {
	QRCodes  []adminQRItem
	FlashOK  string
	FlashErr string
}

type adminGlobalResources struct {
	Devs      int
	Analysts  int
	Testers   int
	Designers int
	DevOps    int
	TechLead  int
}

type adminResourcesPageData struct {
	Resources adminGlobalResources
	FlashOK   string
	FlashErr  string
}
type roundResourceRequest struct {
	TeamID    int64
	TeamName  string
	Devs      int
	Analysts  int
	Testers   int
	Designers int
	DevOps    int
	TechLead  int
}

type roundResourceAllocation struct {
	TeamID        int64
	TeamName      string
	Devs          int
	Analysts      int
	Testers       int
	Designers     int
	DevOps        int
	TechLead      int
	DevsDiff      int // разница (получено - запрошено)
	AnalystsDiff  int
	TestersDiff   int
	DesignersDiff int
	DevOpsDiff    int
	TechLeadDiff  int
}

type adminRoundResourcesPageData struct {
	Round       adminRound
	Teams       []adminTeam
	Requests    []roundResourceRequest
	RequestsMap map[int64]roundResourceRequest // для быстрого поиска в шаблоне
	Allocations []roundResourceAllocation
	Global      adminGlobalResources
	FlashOK     string
	FlashErr    string
}

func mountAdmin(r chi.Router, pool *pgxpool.Pool, tpl *template.Template) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/tasks", http.StatusFound)
	})

	r.Get("/tasks", adminTasksGet(pool, tpl))
	r.Post("/tasks", adminTasksCreate(pool))
	r.Post("/tasks/{taskID}", adminTasksUpdate(pool))
	r.Post("/tasks/{taskID}/toggle", adminTasksToggle(pool))
	r.Post("/tasks/import", adminTasksImport(pool))

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
	r.Post("/settings/qr/{id}/toggle-show", adminSettingsQRToggleShow(pool))
	r.Post("/settings/qr/{id}/delete", adminSettingsQRDelete(pool))
	r.Get("/settings/qr/{id}/edit", adminSettingsQREditGet(pool, tpl))
	r.Post("/settings/qr/{id}/edit", adminSettingsQREditPost(pool))
	r.Post("/settings/reset", adminResetProgress(pool))

	r.Get("/resources", adminResourcesGet(pool, tpl))
	r.Post("/resources", adminResourcesPost(pool))
	r.Get("/rounds/{roundID}/resources", adminRoundResourcesGet(pool, tpl))
	r.Post("/rounds/{roundID}/resources", adminRoundResourcesPost(pool))
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
		teams, err := listTeams(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ok, e := flashFromQuery(r)
		renderAdmin(tpl, w, "admin_tasks.html", adminTasksPageData{Tasks: tasks, Teams: teams, FlashOK: ok, FlashErr: e})
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
		devs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("devs")))
		analysts, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("analysts")))
		devops, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("devops")))
		designers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("designers")))
		testers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("testers")))
		repeatable := r.FormValue("repeatable") != ""
		teamIDStr := strings.TrimSpace(r.FormValue("team_id"))
		var teamID *int64
		if teamIDStr != "" && teamIDStr != "0" {
			if id, err := strconv.ParseInt(teamIDStr, 10, 64); err == nil && id > 0 {
				teamID = &id
			}
		}
		log.Printf("[adminTasksCreate] title=%q, teamIDStr=%q, teamID=%v", title, teamIDStr, teamID)
		if title == "" || points < -100000 || points > 100000 ||
			!validResource(devs) || !validResource(analysts) || !validResource(devops) || !validResource(designers) || !validResource(testers) {
			http.Redirect(w, r, "/admin/tasks?err=invalid+task", http.StatusFound)
			return
		}
		if err := createTask(r.Context(), pool, title, desc, points, devs, analysts, devops, designers, testers, repeatable, teamID); err != nil {
			log.Printf("[adminTasksCreate] createTask error: %v", err)
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
		devs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("devs")))
		analysts, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("analysts")))
		devops, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("devops")))
		designers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("designers")))
		testers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("testers")))
		repeatable := r.FormValue("repeatable") != ""
		teamIDStr := strings.TrimSpace(r.FormValue("team_id"))
		var teamID *int64
		if teamIDStr != "" && teamIDStr != "0" {
			if id, err := strconv.ParseInt(teamIDStr, 10, 64); err == nil && id > 0 {
				teamID = &id
			}
		}
		log.Printf("[adminTasksUpdate] taskID=%d, title=%q, teamIDStr=%q, teamID=%v", taskID, title, teamIDStr, teamID)
		if title == "" || points < -100000 || points > 100000 ||
			!validResource(devs) || !validResource(analysts) || !validResource(devops) || !validResource(designers) || !validResource(testers) {
			http.Redirect(w, r, "/admin/tasks?err=invalid+task", http.StatusFound)
			return
		}
		if err := updateTask(r.Context(), pool, taskID, title, desc, points, devs, analysts, devops, designers, testers, repeatable, teamID); err != nil {
			log.Printf("[adminTasksUpdate] updateTask error: %v", err)
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

func adminTasksImport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const path = "data/tasks_seed.csv"
		f, err := os.Open(path)
		if err != nil {
			http.Redirect(w, r, "/admin/tasks?err=csv+not+found", http.StatusFound)
			return
		}
		defer f.Close()

		reader := csv.NewReader(f)
		reader.TrimLeadingSpace = true

		records, err := reader.ReadAll()
		if err != nil || len(records) == 0 {
			http.Redirect(w, r, "/admin/tasks?err=csv+read+error", http.StatusFound)
			return
		}

		imported := 0
		skipped := 0
		ctx := r.Context()

		// Expect header: title,description,points,devs,analysts,devops,designers,testers,repeatable
		for i, row := range records {
			if i == 0 {
				continue
			}
			if len(row) < 9 {
				skipped++
				continue
			}
			title := strings.TrimSpace(row[0])
			desc := strings.TrimSpace(row[1])
			if title == "" {
				skipped++
				continue
			}
			points, err := strconv.Atoi(strings.TrimSpace(row[2]))
			if err != nil {
				skipped++
				continue
			}
			devs, _ := strconv.Atoi(strings.TrimSpace(row[3]))
			analysts, _ := strconv.Atoi(strings.TrimSpace(row[4]))
			devops, _ := strconv.Atoi(strings.TrimSpace(row[5]))
			designers, _ := strconv.Atoi(strings.TrimSpace(row[6]))
			testers, _ := strconv.Atoi(strings.TrimSpace(row[7]))
			repeatableStr := strings.TrimSpace(row[8])
			repeatable := repeatableStr == "true" || repeatableStr == "1"

			// Check existing by title + points
			var existingID int64
			err = pool.QueryRow(ctx, `SELECT id FROM tasks WHERE title=$1 AND points=$2 LIMIT 1`, title, points).Scan(&existingID)
			if err != nil && err != pgx.ErrNoRows {
				skipped++
				continue
			}
			if existingID != 0 {
				skipped++
				continue
			}

			if err := createTask(ctx, pool, title, desc, points, devs, analysts, devops, designers, testers, repeatable, nil); err != nil {
				skipped++
				continue
			}
			imported++
		}

		okMsg := fmt.Sprintf("imported:%d,skipped:%d", imported, skipped)
		http.Redirect(w, r, "/admin/tasks?ok="+urlQueryEscape(okMsg), http.StatusFound)
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
		http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d/resources?ok=sprint+started", roundID), http.StatusFound)
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

		// Enforce: each non-repeatable task can be completed only by one team globally.
		if err := ensureGlobalOneTimeUniqueness(r.Context(), pool, pairs, 0); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d?err=task+already+used", roundID), http.StatusFound)
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

func ensureGlobalOneTimeUniqueness(ctx context.Context, pool *pgxpool.Pool, pairs []completionPair, selfTeamID int64) error {
	if len(pairs) == 0 {
		return nil
	}
	// For each pair, ensure there is no completion of this task by another team.
	for _, p := range pairs {
		ok, err := canAssignOneTimeTask(ctx, pool, p.taskID, p.teamID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("task already used by another team")
		}
	}
	return nil
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
SELECT id, title, description, points,
       devs, analysts, devops, designers, testers,
       repeatable, active, team_id, created_at
FROM tasks
ORDER BY active DESC, created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTask
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points,
			&t.Devs, &t.Analysts, &t.DevOps, &t.Designers, &t.Testers,
			&t.Repeatable, &t.Active, &t.TeamID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listTasksByRepeatable(ctx context.Context, pool *pgxpool.Pool, repeatable bool) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points,
       devs, analysts, devops, designers, testers,
       repeatable, active, team_id, created_at
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
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points,
			&t.Devs, &t.Analysts, &t.DevOps, &t.Designers, &t.Testers,
			&t.Repeatable, &t.Active, &t.TeamID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listActiveTasksByRepeatable(ctx context.Context, pool *pgxpool.Pool, repeatable bool) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points,
       devs, analysts, devops, designers, testers,
       repeatable, active, team_id, created_at
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
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points,
			&t.Devs, &t.Analysts, &t.DevOps, &t.Designers, &t.Testers,
			&t.Repeatable, &t.Active, &t.TeamID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func createTask(ctx context.Context, pool *pgxpool.Pool, title, description string, points int, devs, analysts, devops, designers, testers int, repeatable bool, teamID *int64) error {
	_, err := pool.Exec(ctx, `
INSERT INTO tasks(title, description, points, devs, analysts, devops, designers, testers, repeatable, team_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		title, description, points,
		clampResource(devs), clampResource(analysts), clampResource(devops), clampResource(designers), clampResource(testers),
		repeatable, teamID)
	return err
}

func updateTask(ctx context.Context, pool *pgxpool.Pool, id int64, title, description string, points int, devs, analysts, devops, designers, testers int, repeatable bool, teamID *int64) error {
	ct, err := pool.Exec(ctx, `
UPDATE tasks
SET title=$2, description=$3, points=$4,
    devs=$5, analysts=$6, devops=$7, designers=$8, testers=$9,
    repeatable=$10, team_id=$11,
    updated_at=now()
WHERE id=$1`,
		id, title, description, points,
		clampResource(devs), clampResource(analysts), clampResource(devops), clampResource(designers), clampResource(testers),
		repeatable, teamID)
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
	Teams           []adminTeam
	TeamID          int64
	Rounds          []adminRound
	OneTimeTasks    []adminTask
	RepeatableTasks []adminTask
	Completions     []adminCompletionRow
	FlashOK         string
	FlashErr        string
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
			Teams:    teams,
			TeamID:   teamID,
			FlashOK:  ok,
			FlashErr: e,
		}

		if teamID > 0 {
			rounds, err := listActiveRounds(r.Context(), pool)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			oneTime, err := listAvailableOneTimeTasksForTeam(r.Context(), pool, teamID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			repeatable, err := listActiveRepeatableTasksForTeam(r.Context(), pool, teamID)
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
		if ok, err := canAssignOneTimeTask(r.Context(), pool, taskID, teamID); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=save+failed", teamID), http.StatusFound)
			return
		} else if !ok {
			http.Redirect(w, r, fmt.Sprintf("/admin/game?team=%d&err=task+already+used", teamID), http.StatusFound)
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

func canAssignOneTimeTask(ctx context.Context, pool *pgxpool.Pool, taskID, teamID int64) (bool, error) {
	rep, err := isTaskRepeatable(ctx, pool, taskID)
	if err != nil {
		return false, err
	}
	if rep {
		return true, nil
	}
	var exists bool
	err = pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1
  FROM completions c
  JOIN tasks t ON t.id = c.task_id
  WHERE c.task_id = $1 AND t.repeatable = false AND c.team_id <> $2
)`, taskID, teamID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return !exists, nil
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
func adminSettingsQRToggleShow(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+id", http.StatusFound)
			return
		}
		ctx := r.Context()
		_, err = pool.Exec(ctx, `
UPDATE qr_codes
SET show_on_home = NOT show_on_home,
    updated_at = now()
WHERE id = $1 AND active = true`, id)
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/settings?ok=toggled", http.StatusFound)
	}
}

func adminSettingsQRDelete(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+id", http.StatusFound)
			return
		}
		ctx := r.Context()
		// Мягкое удаление: устанавливаем active = false
		_, err = pool.Exec(ctx, `
UPDATE qr_codes
SET active = false,
    updated_at = now()
WHERE id = $1`, id)
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=delete+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/settings?ok=deleted", http.StatusFound)
	}
}

func adminSettingsQREditGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+id", http.StatusFound)
			return
		}
		ctx := r.Context()
		qr, err := getQRCode(ctx, pool, id)
		if err != nil {
			if isNotFound(err) {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/admin/settings?err=not+found", http.StatusFound)
			return
		}
		ok, e := flashFromQuery(r)
		renderAdmin(tpl, w, "admin_qr_edit.html", struct {
			QR       adminQRItem
			FlashOK  string
			FlashErr string
		}{
			QR:       qr,
			FlashOK:  ok,
			FlashErr: e,
		})
	}
}

func adminSettingsQREditPost(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+id", http.StatusFound)
			return
		}
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
		showOnHome := r.FormValue("show_on_home") != ""
		ctx := r.Context()

		// Если загружено новое изображение, обновляем его
		var filename, mime string
		var data []byte
		file, _, err := r.FormFile("image")
		if err == nil {
			// есть новое изображение
			defer file.Close()
			limited := io.LimitReader(file, 512*1024+1)
			b, err := io.ReadAll(limited)
			if err != nil {
				http.Redirect(w, r, "/admin/settings?err=bad+file", http.StatusFound)
				return
			}
			if int64(len(b)) > 512*1024 {
				http.Redirect(w, r, "/admin/settings?err=file+too+large", http.StatusFound)
				return
			}
			mime = http.DetectContentType(b)
			if mime == "" {
				mime = "image/png"
			}
			filename = "uploaded.png"
			data = b
		}

		if len(data) > 0 {
			// обновляем с изображением
			_, err = pool.Exec(ctx, `
UPDATE qr_codes
SET kind=$2, title=$3, description=$4, show_on_home=$5,
    image_filename=$6, image_mime=$7, image_data=$8,
    updated_at=now()
WHERE id=$1 AND active=true`,
				id, kind, title, desc, showOnHome, filename, mime, data)
		} else {
			// обновляем без изображения
			_, err = pool.Exec(ctx, `
UPDATE qr_codes
SET kind=$2, title=$3, description=$4, show_on_home=$5,
    updated_at=now()
WHERE id=$1 AND active=true`,
				id, kind, title, desc, showOnHome)
		}
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/settings?ok=qr+updated", http.StatusFound)
	}
}

func adminResetProgress(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/settings?err=bad+form", http.StatusFound)
			return
		}
		// Optional confirmation token could be added, but for simplicity we just execute.
		ctx := r.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			http.Redirect(w, r, "/admin/settings?err=tx+failed", http.StatusFound)
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// 1. Delete all completions
		if _, err := tx.Exec(ctx, `DELETE FROM completions`); err != nil {
			http.Redirect(w, r, "/admin/settings?err=delete+completions+failed", http.StatusFound)
			return
		}
		// 2. Reset round timestamps
		if _, err := tx.Exec(ctx, `UPDATE rounds SET started_at = NULL, ended_at = NULL`); err != nil {
			http.Redirect(w, r, "/admin/settings?err=reset+rounds+failed", http.StatusFound)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			http.Redirect(w, r, "/admin/settings?err=commit+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/settings?ok=progress+reset", http.StatusFound)
	}
}

func listAllQRCodes(ctx context.Context, pool *pgxpool.Pool) ([]adminQRItem, error) {
	rows, err := pool.Query(ctx, `
SELECT id, kind, title, description, show_on_home
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
		if err := rows.Scan(&it.ID, &it.Kind, &it.Title, &it.Description, &it.ShowOnHome); err != nil {
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

func getQRCode(ctx context.Context, pool *pgxpool.Pool, id int64) (adminQRItem, error) {
	var it adminQRItem
	err := pool.QueryRow(ctx, `
SELECT id, kind, title, description, show_on_home
FROM qr_codes
WHERE id=$1 AND active=true`, id).Scan(&it.ID, &it.Kind, &it.Title, &it.Description, &it.ShowOnHome)
	if err != nil {
		if err == pgx.ErrNoRows {
			return adminQRItem{}, errNotFound("qr code")
		}
		return adminQRItem{}, err
	}
	if it.Kind == "author" {
		it.KindLabel = "Автор"
	} else {
		it.KindLabel = "Партнёр"
	}
	return it, nil
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

func validResource(x int) bool {
	return x >= 0 && x <= 100
}

func clampResource(x int) int {
	if x < 0 {
		return 0
	}
	if x > 100 {
		return 100
	}
	return x
}

func urlQueryEscape(s string) string {
	// Minimal escaping for use in ok=... messages; replace spaces with +.
	return strings.ReplaceAll(s, " ", "+")
}

func listAvailableOneTimeTasks(ctx context.Context, pool *pgxpool.Pool) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points,
       devs, analysts, devops, designers, testers,
       repeatable, active, team_id, created_at
FROM tasks t
WHERE repeatable = false
  AND active = true
  AND NOT EXISTS (
    SELECT 1 FROM completions c
    WHERE c.task_id = t.id
  )
ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTask
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points,
			&t.Devs, &t.Analysts, &t.DevOps, &t.Designers, &t.Testers,
			&t.Repeatable, &t.Active, &t.TeamID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listAvailableOneTimeTasksForTeam(ctx context.Context, pool *pgxpool.Pool, teamID int64) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points,
       devs, analysts, devops, designers, testers,
       repeatable, active, team_id, created_at
FROM tasks t
WHERE repeatable = false
  AND active = true
  AND (team_id IS NULL OR team_id = $1)
  AND NOT EXISTS (
    SELECT 1 FROM completions c
    WHERE c.task_id = t.id
  )
ORDER BY created_at ASC, id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTask
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points,
			&t.Devs, &t.Analysts, &t.DevOps, &t.Designers, &t.Testers,
			&t.Repeatable, &t.Active, &t.TeamID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listActiveRepeatableTasksForTeam(ctx context.Context, pool *pgxpool.Pool, teamID int64) ([]adminTask, error) {
	rows, err := pool.Query(ctx, `
SELECT id, title, description, points,
       devs, analysts, devops, designers, testers,
       repeatable, active, team_id, created_at
FROM tasks
WHERE repeatable = true
  AND active = true
  AND (team_id IS NULL OR team_id = $1)
ORDER BY created_at ASC, id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adminTask
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Points,
			&t.Devs, &t.Analysts, &t.DevOps, &t.Designers, &t.Testers,
			&t.Repeatable, &t.Active, &t.TeamID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func listActiveRounds(ctx context.Context, pool *pgxpool.Pool) ([]adminRound, error) {
	rows, err := pool.Query(ctx, `
SELECT id, number, name, started_at, ended_at, created_at
FROM rounds
WHERE started_at IS NOT NULL AND ended_at IS NULL
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

// --- Global resources management ---

func getGlobalResources(ctx context.Context, pool *pgxpool.Pool) (adminGlobalResources, error) {
	var res adminGlobalResources
	err := pool.QueryRow(ctx, `
SELECT devs, analysts, testers, designers, devops, techlead
FROM global_resources
WHERE id = 1`).Scan(&res.Devs, &res.Analysts, &res.Testers, &res.Designers, &res.DevOps, &res.TechLead)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Insert default row if missing
			_, err = pool.Exec(ctx, `
INSERT INTO global_resources (id, devs, analysts, testers, designers, devops, techlead)
VALUES (1, 0, 0, 0, 0, 0, 0)
ON CONFLICT (id) DO NOTHING`)
			if err != nil {
				return adminGlobalResources{}, err
			}
			// Retry
			return getGlobalResources(ctx, pool)
		}
		return adminGlobalResources{}, err
	}
	return res, nil
}

func updateGlobalResources(ctx context.Context, pool *pgxpool.Pool, res adminGlobalResources) error {
	_, err := pool.Exec(ctx, `
UPDATE global_resources
SET devs = $1, analysts = $2, testers = $3, designers = $4, devops = $5, techlead = $6, updated_at = now()
WHERE id = 1`,
		res.Devs, res.Analysts, res.Testers, res.Designers, res.DevOps, res.TechLead)
	return err
}

func adminResourcesGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := getGlobalResources(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ok, e := flashFromQuery(r)
		renderAdmin(tpl, w, "admin_resources.html", adminResourcesPageData{
			Resources: res,
			FlashOK:   ok,
			FlashErr:  e,
		})
	}
}

func adminResourcesPost(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/resources?err=bad+form", http.StatusFound)
			return
		}
		devs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("devs")))
		analysts, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("analysts")))
		testers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("testers")))
		designers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("designers")))
		devops, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("devops")))
		techlead, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("techlead")))

		// Validate
		if devs < 0 || analysts < 0 || testers < 0 || designers < 0 || devops < 0 || techlead < 0 {
			http.Redirect(w, r, "/admin/resources?err=negative+value", http.StatusFound)
			return
		}
		// Optionally limit to some max
		if devs > 1000 || analysts > 1000 || testers > 1000 || designers > 1000 || devops > 1000 || techlead > 1000 {
			http.Redirect(w, r, "/admin/resources?err=too+large", http.StatusFound)
			return
		}

		res := adminGlobalResources{
			Devs:      devs,
			Analysts:  analysts,
			Testers:   testers,
			Designers: designers,
			DevOps:    devops,
			TechLead:  techlead,
		}
		if err := updateGlobalResources(r.Context(), pool, res); err != nil {
			http.Redirect(w, r, "/admin/resources?err=save+failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin/resources?ok=saved", http.StatusFound)
	}
}

// --- Round resources management ---

func getRoundResourceRequests(ctx context.Context, pool *pgxpool.Pool, roundID int64) ([]roundResourceRequest, error) {
	rows, err := pool.Query(ctx, `
SELECT r.team_id, t.name, r.devs, r.analysts, r.testers, r.designers, r.devops, r.techlead
FROM round_resource_requests r
JOIN teams t ON t.id = r.team_id
WHERE r.round_id = $1
ORDER BY t.name`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []roundResourceRequest
	for rows.Next() {
		var req roundResourceRequest
		if err := rows.Scan(&req.TeamID, &req.TeamName, &req.Devs, &req.Analysts, &req.Testers, &req.Designers, &req.DevOps, &req.TechLead); err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func saveRoundResourceRequests(ctx context.Context, pool *pgxpool.Pool, roundID int64, requests []roundResourceRequest) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Удалить старые записи для этого раунда
	_, err = tx.Exec(ctx, `DELETE FROM round_resource_requests WHERE round_id = $1`, roundID)
	if err != nil {
		return err
	}

	// Вставить новые
	for _, req := range requests {
		_, err = tx.Exec(ctx, `
INSERT INTO round_resource_requests (round_id, team_id, devs, analysts, testers, designers, devops, techlead)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			roundID, req.TeamID, req.Devs, req.Analysts, req.Testers, req.Designers, req.DevOps, req.TechLead)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// allocateResources распределяет глобальные ресурсы между командами пропорционально запросам.
// Возвращает распределение (сколько получила каждая команда) и оставшиеся глобальные ресурсы.
func allocateResources(global adminGlobalResources, requests []roundResourceRequest) ([]roundResourceAllocation, adminGlobalResources) {
	if len(requests) == 0 {
		return nil, global
	}

	// Суммируем запросы по каждому типу
	totalRequested := adminGlobalResources{}
	for _, req := range requests {
		totalRequested.Devs += req.Devs
		totalRequested.Analysts += req.Analysts
		totalRequested.Testers += req.Testers
		totalRequested.Designers += req.Designers
		totalRequested.DevOps += req.DevOps
		totalRequested.TechLead += req.TechLead
	}

	// Если запросов больше, чем доступно, распределяем пропорционально.
	allocations := make([]roundResourceAllocation, len(requests))
	for i, req := range requests {
		alloc := roundResourceAllocation{
			TeamID:   req.TeamID,
			TeamName: req.TeamName,
		}
		// Распределяем каждый тип ресурсов
		alloc.Devs = allocateSingle(global.Devs, totalRequested.Devs, req.Devs)
		alloc.Analysts = allocateSingle(global.Analysts, totalRequested.Analysts, req.Analysts)
		alloc.Testers = allocateSingle(global.Testers, totalRequested.Testers, req.Testers)
		alloc.Designers = allocateSingle(global.Designers, totalRequested.Designers, req.Designers)
		alloc.DevOps = allocateSingle(global.DevOps, totalRequested.DevOps, req.DevOps)
		alloc.TechLead = allocateSingle(global.TechLead, totalRequested.TechLead, req.TechLead)

		// Вычисляем разницу
		alloc.DevsDiff = alloc.Devs - req.Devs
		alloc.AnalystsDiff = alloc.Analysts - req.Analysts
		alloc.TestersDiff = alloc.Testers - req.Testers
		alloc.DesignersDiff = alloc.Designers - req.Designers
		alloc.DevOpsDiff = alloc.DevOps - req.DevOps
		alloc.TechLeadDiff = alloc.TechLead - req.TechLead

		allocations[i] = alloc
	}

	// Вычитаем распределённые ресурсы из глобальных
	remaining := global
	for _, alloc := range allocations {
		remaining.Devs -= alloc.Devs
		remaining.Analysts -= alloc.Analysts
		remaining.Testers -= alloc.Testers
		remaining.Designers -= alloc.Designers
		remaining.DevOps -= alloc.DevOps
		remaining.TechLead -= alloc.TechLead
	}
	return allocations, remaining
}

// allocateSingle возвращает количество ресурса, которое можно выделить команде.
// Если доступного ресурса достаточно для всех запросов, возвращаем запрошенное количество.
// Иначе распределяем пропорционально (целые числа, округление вниз).
func allocateSingle(available, totalRequested, requested int) int {
	if available >= totalRequested {
		return requested
	}
	if totalRequested == 0 {
		return 0
	}
	// пропорция: requested / totalRequested * available
	alloc := (requested * available) / totalRequested
	// гарантируем, что не превысим доступное (из-за округления может быть на 1 меньше)
	if alloc > available {
		alloc = available
	}
	return alloc
}

func adminRoundResourcesGet(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
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
		requests, err := getRoundResourceRequests(r.Context(), pool, roundID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		global, err := getGlobalResources(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		allocations, remaining := allocateResources(global, requests)
		ok, e := flashFromQuery(r)
		log.Printf("[adminRoundResourcesGet] roundID=%d, teams=%d, requests=%d, global=%+v, allocations=%d", roundID, len(teams), len(requests), global, len(allocations))
		// Создаём map для быстрого поиска запроса по teamID
		requestsMap := make(map[int64]roundResourceRequest)
		for _, req := range requests {
			requestsMap[req.TeamID] = req
		}
		renderAdmin(tpl, w, "admin_round_resources.html", adminRoundResourcesPageData{
			Round:       round,
			Teams:       teams,
			Requests:    requests,
			RequestsMap: requestsMap,
			Allocations: allocations,
			Global:      remaining,
			FlashOK:     ok,
			FlashErr:    e,
		})
	}
}

func adminRoundResourcesPost(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roundID, err := parseID(chi.URLParam(r, "roundID"))
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d/resources?err=bad+round+id", roundID), http.StatusFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d/resources?err=bad+form", roundID), http.StatusFound)
			return
		}
		// Получаем список команд
		teams, err := listTeams(r.Context(), pool)
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d/resources?err=internal", roundID), http.StatusFound)
			return
		}
		var requests []roundResourceRequest
		for _, team := range teams {
			prefix := fmt.Sprintf("team_%d_", team.ID)
			devs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(prefix + "devs")))
			analysts, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(prefix + "analysts")))
			testers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(prefix + "testers")))
			designers, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(prefix + "designers")))
			devops, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(prefix + "devops")))
			techlead, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(prefix + "techlead")))
			// Если все нули, можно пропустить, но сохраним для единообразия
			requests = append(requests, roundResourceRequest{
				TeamID:    team.ID,
				TeamName:  team.Name,
				Devs:      devs,
				Analysts:  analysts,
				Testers:   testers,
				Designers: designers,
				DevOps:    devops,
				TechLead:  techlead,
			})
		}
		if err := saveRoundResourceRequests(r.Context(), pool, roundID, requests); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d/resources?err=save+failed", roundID), http.StatusFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/rounds/%d/resources?ok=saved", roundID), http.StatusFound)
	}
}
