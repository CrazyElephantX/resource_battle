package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"resource_battle/internal/web"
)

// serverURL — базовый URL приложения (для публичных ссылок и QR).
var serverURL string

// New создаёт HTTP-хендлер с опциональным URL сервера.
func New(pool *pgxpool.Pool) http.Handler {
	return NewWithServerURL(pool, "")
}

// NewWithServerURL создаёт HTTP-хендлер с указанным URL сервера.
func NewWithServerURL(pool *pgxpool.Pool, srvURL string) http.Handler {
	if srvURL == "" {
		srvURL = serverURL
	}

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

	r.Route("/media", func(r chi.Router) {
		r.Get("/main-logo", handleMainLogo(pool))
		r.Get("/partner-logo", handlePartnerLogo(pool))
		r.Get("/partner3-logo", handlePartner3Logo(pool))
		r.Get("/qr/{id}", handleQRImage(pool))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		rows, err := leaderboard(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		qrCodes, err := listHomeQRCodes(r.Context(), pool)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Разделяем QR-коды на левые и правые
		mid := len(qrCodes) / 2
		qrLeft := qrCodes[:mid]
		qrRight := qrCodes[mid:]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "dashboard.html", dashboardPageData{
			Rows:          rows,
			QRCodesLeft:   qrLeft,
			QRCodesRight:  qrRight,
			ServerURL:     srvURL,
			SubmissionURL: submissionURL(srvURL),
		})
	})

	r.Route("/admin", func(r chi.Router) {
		mountAdmin(r, pool, tpl)
	})

	r.Get("/qr", handleQRPage(pool, tpl))

	// Публичные маршруты подачи результатов командами
	r.Route("/submit", func(r chi.Router) {
		r.Get("/", handleSubmitLanding(pool, tpl))
		r.Get("/{teamID}", handleSubmitPage(pool, tpl))
		r.Post("/{teamID}", handleSubmitPost(pool))
	})

	return r
}

// submissionURL формирует URL публичной страницы подачи результатов.
// Если serverURL не задан — возвращает относительный путь.
func submissionURL(srvURL string) string {
	if srvURL != "" {
		return srvURL + "/submit"
	}
	return "/submit"
}
