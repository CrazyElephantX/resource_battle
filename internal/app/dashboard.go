package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dashboardRow struct {
	Place    int
	TeamID   int64
	TeamName string
	Points   int
	Tasks    int
}

type dashboardPageData struct {
	Rows []dashboardRow
}

func leaderboard(ctx context.Context, pool *pgxpool.Pool) ([]dashboardRow, error) {
	// Points are sum of points for completed tasks across all rounds.
	// Teams with no completions still appear with 0 points.
	rows, err := pool.Query(ctx, `
SELECT
  t.id,
  t.name,
  COALESCE(SUM(tsk.points * c.count), 0) AS points,
  COALESCE(SUM(c.count), 0) AS tasks_completed
FROM teams t
LEFT JOIN completions c ON c.team_id = t.id
LEFT JOIN tasks tsk ON tsk.id = c.task_id
GROUP BY t.id, t.name
ORDER BY points DESC, t.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query leaderboard: %w", err)
	}
	defer rows.Close()

	out := make([]dashboardRow, 0, 6)
	place := 0
	for rows.Next() {
		place++
		var r dashboardRow
		r.Place = place
		if err := rows.Scan(&r.TeamID, &r.TeamName, &r.Points, &r.Tasks); err != nil {
			return nil, fmt.Errorf("scan leaderboard: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leaderboard: %w", err)
	}
	return out, nil
}

func teamStats(ctx context.Context, pool *pgxpool.Pool, teamID int64) (points int, tasksCompleted int, err error) {
	// tasksCompleted: total occurrences (sum(count))
	// points: sum(task.points * count)
	row := pool.QueryRow(ctx, `
SELECT
  COALESCE(SUM(tsk.points * c.count), 0) AS points,
  COALESCE(SUM(c.count), 0) AS tasks_completed
FROM teams t
LEFT JOIN completions c ON c.team_id = t.id
LEFT JOIN tasks tsk ON tsk.id = c.task_id
WHERE t.id = $1
GROUP BY t.id`, teamID)

	if err := row.Scan(&points, &tasksCompleted); err != nil {
		// If no rows (team exists but no completions), the GROUP BY returns no row.
		// Use a second query to distinguish "no completions" from "no team".
		var exists bool
		if err2 := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE id=$1)`, teamID).Scan(&exists); err2 != nil {
			return 0, 0, fmt.Errorf("check team exists: %w", err2)
		}
		if !exists {
			return 0, 0, errNotFound("team")
		}
		return 0, 0, nil
	}
	return points, tasksCompleted, nil
}

type apiError struct {
	Error string `json:"error"`
}

type notFoundError struct {
	what string
}

func (e notFoundError) Error() string { return e.what + " not found" }

func errNotFound(what string) error { return notFoundError{what: what} }

func isNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleTeamStats(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "teamID")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid team id"})
			return
		}
		points, tasks, err := teamStats(r.Context(), pool, id)
		if err != nil {
			if isNotFound(err) {
				writeJSON(w, http.StatusNotFound, apiError{Error: "team not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"team_id":         id,
			"points":          points,
			"tasks_completed": tasks,
		})
	}
}

