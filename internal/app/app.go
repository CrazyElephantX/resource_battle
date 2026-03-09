package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"resource_battle/internal/web"
)

func New(pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	tpl := web.MustTemplates()

	static, err := staticHandler()
	if err != nil {
		panic(err)
	}
	r.Mount("/static/", static)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		rows, err := leaderboard(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "dashboard.html", dashboardPageData{Rows: rows})
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/teams/{teamID}/stats", handleTeamStats(pool))
	})

	r.Route("/admin", func(r chi.Router) {
		mountAdmin(r, pool, tpl)
	})

	return r
}

