package app

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
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
	Rows          []dashboardRow
	QRCodesLeft   []qrCodePublic
	QRCodesRight  []qrCodePublic
	ServerURL     string
	SubmissionURL string
}

func leaderboard(ctx context.Context, pool *pgxpool.Pool) ([]dashboardRow, error) {
	// Points are sum of points for completed tasks across all rounds.
	// Teams with no completions still appear with 0 points.
	rows, err := pool.Query(ctx, `
SELECT
  t.id,
  t.name,
  COALESCE(SUM(tsk.points * c.count), 0) AS points,
  COALESCE(SUM(CASE WHEN tsk.repeatable = false THEN c.count ELSE 0 END), 0) AS tasks_completed
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

func listHomeQRCodes(ctx context.Context, pool *pgxpool.Pool) ([]qrCodePublic, error) {
	rows, err := pool.Query(ctx, `
SELECT id, kind, title, description, show_on_home
FROM qr_codes
WHERE active=true AND show_on_home=true
ORDER BY kind ASC, created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query home qr codes: %w", err)
	}
	defer rows.Close()

	var codes []qrCodePublic
	for rows.Next() {
		var q qrCodePublic
		if err := rows.Scan(&q.ID, &q.Kind, &q.Title, &q.Description, &q.ShowOnHome); err != nil {
			return nil, fmt.Errorf("scan qr code: %w", err)
		}
		codes = append(codes, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate qr codes: %w", err)
	}
	return codes, nil
}

func teamStats(ctx context.Context, pool *pgxpool.Pool, teamID int64) (points int, tasksCompleted int, err error) {
	// tasksCompleted: total occurrences (sum(count)) for non-repeatable tasks only
	// points: sum(task.points * count) for all tasks
	row := pool.QueryRow(ctx, `
SELECT
  COALESCE(SUM(tsk.points * c.count), 0) AS points,
  COALESCE(SUM(CASE WHEN tsk.repeatable = false THEN c.count ELSE 0 END), 0) AS tasks_completed
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

// --- Results page ---

type resultsRow struct {
	TeamID   int64
	TeamName string
	Points   int
	Tasks    int
}

type resultsPageData struct {
	Description string
	ImageName   string
	Rows        []resultsRow
	QRCodes     []qrCodePublic
}

func handleResults(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := leaderboardForResults(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		qrCodes, err := listHomeQRCodes(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Compute average score across all teams.
		totalScore := 0
		for _, r := range rows {
			totalScore += r.Points
		}
		if len(rows) > 0 {
			totalScore = totalScore / len(rows)
		}
		desc := generateResultsDescription(totalScore)
		img := getResultImageName(totalScore)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "results.html", resultsPageData{
			Description: desc,
			ImageName:   img,
			Rows:        rows,
			QRCodes:     qrCodes,
		})
	}
}

// leaderboardForResults returns all teams with their total points and tasks completed.
func leaderboardForResults(ctx context.Context, pool *pgxpool.Pool) ([]resultsRow, error) {
	rows, err := pool.Query(ctx, `
SELECT
  t.id,
  t.name,
  COALESCE(SUM(tsk.points * c.count), 0) AS points,
  COALESCE(SUM(CASE WHEN tsk.repeatable = false THEN c.count ELSE 0 END), 0) AS tasks_completed
FROM teams t
LEFT JOIN completions c ON c.team_id = t.id
LEFT JOIN tasks tsk ON tsk.id = c.task_id
GROUP BY t.id, t.name
ORDER BY points DESC, t.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query leaderboard for results: %w", err)
	}
	defer rows.Close()

	out := make([]resultsRow, 0)
	for rows.Next() {
		var r resultsRow
		if err := rows.Scan(&r.TeamID, &r.TeamName, &r.Points, &r.Tasks); err != nil {
			return nil, fmt.Errorf("scan results row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate results: %w", err)
	}
	return out, nil
}

// generateResultsDescription returns a shareable verdict based on the total score.
// Score ranges from roughly -30 to +40.
func generateResultsDescription(totalScore int) string {
	switch {
	case totalScore <= -35:
		return "Корпоративный метеорит. От компании остались чат, логотип и уверенность, что идея была хорошая. Вы почти исследовали нижнюю границу возможного — тоже достижение, хотя инвесторы просили другое. Урок: если каждый спасает только свой отдел, спасать компанию вскоре становится некому."

	case totalScore <= -30:
		return "Банкротство на горизонте. Финансовый директор считает убытки, остальные делают вид, что это дыхательная гимнастика. Сейчас важнее договориться о совместном спасении, чем выяснить, чей именно гениальный план привёл сюда всех."

	case totalScore <= -25:
		return "Пожар по предварительной записи. Ресурсов на всё не хватило, зато срочных задач хватило каждому. Компания держится на энтузиазме и одном удлинителе. Урок: приоритеты нужны до того, как огнетушитель становится главным рабочим инструментом."

	case totalScore <= -20:
		return "Дорогой образовательный опыт. Компания получила убытки, команды — впечатления, а фраза «мы же предупреждали» обрела несколько новых интонаций. Предупреждать полезно, но вместе менять план обычно выгоднее."

	case totalScore <= -15:
		return "Отделы победили, компания проиграла. Возможно, кто-то отлично защитил свой бюджет — жаль, общий результат от этого не поправился. Локальный успех хорош ровно до момента, когда общий корабль начинает уходить под воду."

	case totalScore <= -10:
		return "Героизм вместо управления. Все были заняты, многие устали, результат почему-то отрицательный. Оказалось, количество подвигов не заменяет согласованный план. В следующий раз попробуйте распределить ресурсы раньше, чем появится необходимость распределять вину."

	case totalScore <= -5:
		return "Почти выплыли. Компания добралась до берега, но по дороге потеряла часть ресурсов и веру в слово «быстренько». До хорошего результата не хватило нескольких договорённостей. Их стоило обсудить за столом переговоров, а не на разборе последствий."

	case totalScore <= -1:
		return "Маленький минус с большим намёком. Катастрофы не случилось, но ресурсы незаметно утекли в щели между командами. Никто специально не хотел плохого — просто каждый немного рассчитывал, что договорится кто-нибудь другой."

	case totalScore == 0:
		return "Идеальный баланс суеты. Компания закончила ровно там, где начала, только теперь все устали и лучше знакомы. Ноль — тоже результат, но в следующий раз попробуйте согласовывать не только действия, но и то, ради чего вы их совершаете."

	case totalScore <= 4:
		return "Плюс размером с печеньку. Компания заработала немного, зато разговоров хватило бы на годовой отчёт. Начало положено! Теперь осталось научиться превращать договорённости в результат до того, как закончатся время и печеньки."

	case totalScore <= 9:
		return "Бизнес подаёт признаки прибыли. Результат уже положительный, хотя финансовый директор пока улыбается только одним уголком рта. Отдельные удачные решения сработали. Следующий уровень — сделать так, чтобы они помогали друг другу, а не просто происходили одновременно."

	case totalScore <= 12:
		return "Работает, но иногда искрит. Компания вышла в уверенный плюс, хотя часть успеха всё ещё держится на личной находчивости и удачном стечении обстоятельств. Хорошая новость: вы умеете выкручиваться. Ещё лучше будет научиться не закручиваться."

	case totalScore <= 15:
		return "Крепкий результат без права расслабляться. Компания развивается, ресурсы приносят пользу, переговоры иногда даже заканчиваются договорённостями. До полной победы ещё есть дистанция: ищите не того, у кого можно отнять последнее, а то, на чём можно выиграть вместе."

	case totalScore <= 19:
		return "Победа уже машет из соседнего кабинета. Вы хорошо поработали и почти добрались до полной победы. Где-то не хватило обмена, где-то — доверия, а где-то слишком долго берегли ресурс для особого случая. Кажется, этот случай уже прошёл."

	case totalScore <= 22:
		return "Полная победа! Компания заработала, команды договорились, а ресурсы наконец работали, а не охранялись. Вы доказали, что общий успех не обязательно требует общего аврала. Теперь главное — запомнить удачные решения, пока их не объявили счастливой случайностью."

	case totalScore <= 26:
		return "Победа с запасом прочности. Вы не просто достигли цели, а оставили компании пространство для развития. Похоже, команды научились видеть за своим участком общую картину. Берегите этот навык: обычно его первым теряют при делении следующего бюджета."

	case totalScore <= 30:
		return "Высшая лига договорённостей. Компания получила отличный результат, а взаимодействие команд стало активом, а не статьёй скрытых расходов. Вы вовремя делились ресурсами, замечали зависимости и не путали твёрдую позицию с неподвижной. Вот это уже можно масштабировать."

	case totalScore <= 34:
		return "Компания, которой завидуют на конференциях. Со стороны кажется, что вам просто повезло с ресурсами. Но вы-то знаете, сколько за этим своевременных уступок, точных решений и разговоров по существу. Мудрость раунда: иногда лучший способ усилить себя — вовремя усилить соседнюю команду."

	case totalScore <= 39:
		return "Легендарный результат. Вы подошли вплотную к идеалу: компания работает как единое целое, а не как собрание независимых княжеств с общим Wi-Fi. Почти каждый ресурс нашёл полезное применение. На разборе найдите, что осталось на столе: даже выдающемуся результату есть чему научить."

	default:
		return "Идеальный финал. 12 из 10: если это средний балл, значит, максимум взяла каждая команда. Никто не спрятал свои потери за чужим успехом — вся компания действительно победила. Можно гордиться, но не назначайте совершенство обязательным нормативом на каждый вторник."
	}
}

// getResultImageName returns the image filename based on total score.
func getResultImageName(totalScore int) string {
	switch {
	case totalScore <= 10:
		return "result_bad.png"
	case totalScore <= 20:
		return "result_neutral.png"
	case totalScore <= 31:
		return "result_good.png"
	default:
		return "result_legendary.png"
	}
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
