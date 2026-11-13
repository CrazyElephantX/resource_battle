package app

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// submitTask — задача для подачи командой.
type submitTask struct {
	ID         int64
	Title      string
	Description string
	Points     int
	Repeatable bool
	Count      int // текущий счётчик (если уже подан)
}

// submitPageData — данные для страницы подачи результатов.
type submitPageData struct {
	Team            adminTeam
	Round           *adminRound
	ActiveTasks     []submitTask // одноразовые задачи
	RepeatableTasks []submitTask // повторяемые задачи
	Resources       roundResourceRequest
	FlashOK         string
	FlashErr        string
	NoRound         bool
}

// handleSubmitLanding — страница выбора команды.
func handleSubmitLanding(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teams, err := listTeams(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "submit.html", struct {
			Teams []adminTeam
		}{Teams: teams})
	}
}

// handleSubmitPage — страница подачи результатов для конкретной команды.
func handleSubmitPage(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chiURLParam(r, "teamID")
		teamID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || teamID <= 0 {
			http.Redirect(w, r, "/submit", http.StatusFound)
			return
		}

		// Проверим, существует ли команда.
		var teamName string
		err = pool.QueryRow(r.Context(), `SELECT name FROM teams WHERE id=$1`, teamID).Scan(&teamName)
		if err != nil {
			http.Redirect(w, r, "/submit", http.StatusFound)
			return
		}

		// Найдём текущий открытый раунд.
		var round *adminRound
		err = pool.QueryRow(r.Context(), `
			SELECT id, number, name, started_at, ended_at, created_at
			FROM rounds
			WHERE started_at IS NOT NULL AND ended_at IS NULL
			ORDER BY number DESC
			LIMIT 1`,
		).Scan(
			&round.ID, &round.Number, &round.Name,
			&round.StartedAt, &round.EndedAt, &round.CreatedAt,
		)
		if err != nil {
			// Нет открытого раунда — показываем страницу без раунда.
			round = nil
		}

		// Загрузим текущие данные подачи команды.
		var resources roundResourceRequest
		resources.TeamID = teamID
		resources.TeamName = teamName
		if round != nil {
			err = pool.QueryRow(r.Context(), `
				SELECT team_id, devs, analysts, testers, designers, devops, techlead
				FROM round_resource_requests
				WHERE round_id=$1 AND team_id=$2`,
				round.ID, teamID,
			).Scan(
				&resources.TeamID, &resources.Devs, &resources.Analysts,
				&resources.Testers, &resources.Designers, &resources.DevOps, &resources.TechLead,
			)
			if err != nil {
				// Запрос на ресурсы ещё не подан — используем нулевые значения.
				resources = roundResourceRequest{TeamID: teamID, TeamName: teamName}
			}
		}

		// Загрузим задачи.
		var activeTasks, repeatableTasks []submitTask
		if round != nil {
			// Одноразовые задачи (не повторяемые, активные).
			rows, err := pool.Query(r.Context(), `
				SELECT t.id, t.title, t.description, t.points, t.repeatable
				FROM tasks t
				WHERE t.active = true AND t.repeatable = false
				ORDER BY t.points DESC, t.title ASC`,
			)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var st submitTask
					if err := rows.Scan(&st.ID, &st.Title, &st.Description, &st.Points, &st.Repeatable); err != nil {
						log.Printf("[handleSubmitPage] scan task: %v", err)
						break
					}
					// Проверим, не выполнена ли задача этой командой в этом раунде.
					var count int
					err2 := pool.QueryRow(r.Context(), `
						SELECT count FROM completions
						WHERE team_id=$1 AND round_id=$2 AND task_id=$3`,
						teamID, round.ID, st.ID).Scan(&count)
					if err2 == nil && count > 0 {
						st.Count = count
					}
					activeTasks = append(activeTasks, st)
				}
			}

			// Повторяемые задачи.
			rows2, err := pool.Query(r.Context(), `
				SELECT t.id, t.title, t.description, t.points, t.repeatable
				FROM tasks t
				WHERE t.active = true AND t.repeatable = true
				ORDER BY t.points DESC, t.title ASC`,
			)
			if err == nil {
				defer rows2.Close()
				for rows2.Next() {
					var st submitTask
					if err := rows2.Scan(&st.ID, &st.Title, &st.Description, &st.Points, &st.Repeatable); err != nil {
						log.Printf("[handleSubmitPage] scan repeatable task: %v", err)
						break
					}
					// Проверим текущий счётчик.
					var count int
					err2 := pool.QueryRow(r.Context(), `
						SELECT count FROM completions
						WHERE team_id=$1 AND round_id=$2 AND task_id=$3`,
						teamID, round.ID, st.ID).Scan(&count)
					if err2 == nil && count > 0 {
						st.Count = count
					}
					repeatableTasks = append(repeatableTasks, st)
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "submit_page.html", submitPageData{
			Team:            adminTeam{ID: teamID, Name: teamName},
			Round:           round,
			ActiveTasks:     activeTasks,
			RepeatableTasks: repeatableTasks,
			Resources:       resources,
			NoRound:         round == nil,
		})
	}
}

// handleSubmitPost — обработка формы подачи результатов.
func handleSubmitPost(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chiURLParam(r, "teamID")
		teamID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || teamID <= 0 {
			http.Redirect(w, r, "/submit", http.StatusFound)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/submit", http.StatusFound)
			return
		}

		// Найдём текущий открытый раунд.
		var roundID int64
		err = pool.QueryRow(r.Context(), `
			SELECT id FROM rounds
			WHERE started_at IS NOT NULL AND ended_at IS NULL
			ORDER BY number DESC LIMIT 1`,
		).Scan(&roundID)
		if err != nil {
			http.Redirect(w, r, "/submit?err=no+round", http.StatusFound)
			return
		}

		tx, err := pool.Begin(r.Context())
		if err != nil {
			http.Redirect(w, r, "/submit?err=tx+failed", http.StatusFound)
			return
		}
		defer tx.Rollback(r.Context())

		// 1. Обновим одноразовые задачи (checkbox: on = completed).
		taskIDs := r.Form["task_"]
		taskSet := make(map[string]bool)
		for _, id := range taskIDs {
			taskSet[strings.TrimSpace(id)] = true
		}
		for _, id := range taskIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			taskID, parseErr := strconv.ParseInt(id, 10, 64)
			if parseErr != nil {
				continue
			}
			// Проверим, что задача не повторяемая.
			var repeatable bool
			err = pool.QueryRow(r.Context(), `SELECT repeatable FROM tasks WHERE id=$1`, taskID).Scan(&repeatable)
			if err != nil || repeatable {
				continue
			}
			// Удалим старую запись (если была) и вставим новую.
			_, _ = tx.Exec(r.Context(), `DELETE FROM completions WHERE team_id=$1 AND round_id=$2 AND task_id=$3`, teamID, roundID, taskID)
			if taskSet[id] {
				_, err = tx.Exec(r.Context(), `
					INSERT INTO completions (team_id, round_id, task_id, count, completed_at)
					VALUES ($1, $2, $3, 1, now())`,
					teamID, roundID, taskID)
				if err != nil {
					log.Printf("[handleSubmitPost] insert completion: %v", err)
					http.Redirect(w, r, "/submit?err=save+failed", http.StatusFound)
					return
				}
			}
		}

		// 2. Обновим повторяемые задачи (count).
		for _, prefix := range []string{"repeat_count_", "repeat_count"} {
			for key, vals := range r.Form {
				if !strings.HasPrefix(key, prefix) {
					continue
				}
				idStr := strings.TrimPrefix(key, prefix)
				idStr = strings.TrimPrefix(idStr, "_")
				countStr := vals[0]
				count, parseErr := strconv.Atoi(countStr)
				if parseErr != nil || count < 0 {
					continue
				}
				taskID, parseErr := strconv.ParseInt(idStr, 10, 64)
				if parseErr != nil {
					continue
				}
				// Удалим старую запись и вставим новую.
				_, _ = tx.Exec(r.Context(), `DELETE FROM completions WHERE team_id=$1 AND round_id=$2 AND task_id=$3`, teamID, roundID, taskID)
				if count > 0 {
					_, err = tx.Exec(r.Context(), `
						INSERT INTO completions (team_id, round_id, task_id, count, completed_at)
						VALUES ($1, $2, $3, $4, now())`,
						teamID, roundID, taskID, count)
					if err != nil {
						log.Printf("[handleSubmitPost] insert repeatable completion: %v", err)
						http.Redirect(w, r, "/submit?err=save+failed", http.StatusFound)
						return
					}
				}
			}
		}

		// 3. Обновим запрос на ресурсы.
		Devs := mustInt(r.FormValue("res_devs"), 0)
		Analysts := mustInt(r.FormValue("res_analysts"), 0)
		Testers := mustInt(r.FormValue("res_testers"), 0)
		Designers := mustInt(r.FormValue("res_designers"), 0)
		DevOps := mustInt(r.FormValue("res_devops"), 0)
		TechLead := mustInt(r.FormValue("res_techlead"), 0)

		// Удалим старую запись и вставим новую.
		_, _ = tx.Exec(r.Context(), `DELETE FROM round_resource_requests WHERE round_id=$1 AND team_id=$2`, roundID, teamID)
		_, err = tx.Exec(r.Context(), `
			INSERT INTO round_resource_requests (round_id, team_id, devs, analysts, testers, designers, devops, techlead)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			roundID, teamID, Devs, Analysts, Testers, Designers, DevOps, TechLead)
		if err != nil {
			log.Printf("[handleSubmitPost] insert resource request: %v", err)
			http.Redirect(w, r, "/submit?err=save+failed", http.StatusFound)
			return
		}

		if err = tx.Commit(r.Context()); err != nil {
			http.Redirect(w, r, "/submit?err=commit+failed", http.StatusFound)
			return
		}

		// Перенаправим на страницу подтверждения.
		http.Redirect(w, r, fmt.Sprintf("/submit/%d?ok=submitted", teamID), http.StatusFound)
	}
}

// mustInt парсит int или возвращает defaultVal.
func mustInt(s string, defaultVal int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}

// chiURLParam — обёртка над chi.URLParam для совместимости.
func chiURLParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}
