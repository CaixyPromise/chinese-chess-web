// Package httpapi serves authenticated PVE commands and realtime snapshots.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"chinesechess/internal/admin"
	"chinesechess/internal/analysis"
	"chinesechess/internal/auth"
	"chinesechess/internal/game"
	"chinesechess/internal/pvp"
)

type API struct {
	games        *game.Manager
	rooms        *pvp.Manager
	log          *slog.Logger
	analyses     *analysis.Manager
	auth         *auth.Service
	admin        *admin.Service
	accounts     AccountRepository
	identityGate identityGate
}

func New(games *game.Manager, root string, log *slog.Logger, rooms ...*pvp.Manager) http.Handler {
	var room *pvp.Manager
	if len(rooms) > 0 {
		room = rooms[0]
	}
	return newHandler(games, room, nil, nil, nil, root, log)
}

func NewWithAnalysis(games *game.Manager, rooms *pvp.Manager, analyses *analysis.Manager, root string, log *slog.Logger) http.Handler {
	return newHandler(games, rooms, analyses, nil, nil, root, log)
}

func NewWithAuth(games *game.Manager, rooms *pvp.Manager, analyses *analysis.Manager, service *auth.Service, accounts AccountRepository, root string, log *slog.Logger) http.Handler {
	return newHandler(games, rooms, analyses, service, accounts, root, log)
}

func newHandler(games *game.Manager, rooms *pvp.Manager, analyses *analysis.Manager, service *auth.Service, accounts AccountRepository, root string, log *slog.Logger) http.Handler {
	a := &API{games: games, rooms: rooms, analyses: analyses, log: log, auth: service, accounts: accounts}
	mux := http.NewServeMux()
	if service != nil {
		a.admin = admin.New(service.DB())
		a.adminRoutes(mux)
		a.authRoutes(mux)
		a.accountRoutes(mux)
	}
	mux.HandleFunc("GET /api/session", a.session)
	mux.HandleFunc("GET /api/history", a.history)
	mux.HandleFunc("POST /api/games", a.create)
	mux.HandleFunc("POST /api/games/practice", a.createPractice)
	if analyses != nil {
		a.analysisRoutes(mux)
		mux.HandleFunc("GET /api/replay/{mode}/{id}", a.replaySnapshot)
	}
	mux.HandleFunc("GET /api/games/{id}", a.snapshot)
	mux.HandleFunc("POST /api/games/{id}/commands", a.command)
	mux.HandleFunc("GET /api/games/{id}/socket", a.socket)
	if a.rooms != nil {
		a.pvpRoutes(mux)
	}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]bool{"ok": true}) })
	mux.Handle("GET /legacy/", http.StripPrefix("/legacy/", http.FileServer(http.Dir(filepath.Join(root, "assets", "images")))))
	mux.Handle("GET /sounds/", http.StripPrefix("/sounds/", http.FileServer(http.Dir(filepath.Join(root, "assets", "sounds")))))
	dist := filepath.Join(root, "web", "dist")
	files := http.FileServer(http.Dir(dist))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Clean("/" + r.URL.Path)
		if stat, err := os.Stat(filepath.Join(dist, clean)); err == nil && !stat.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if service == nil && r.Method == "POST" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if origin := r.Header.Get("Origin"); origin != "" && origin != scheme+"://"+r.Host {
				respond(w, 403, map[string]string{"code": "FORBIDDEN"})
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
	if service != nil {
		return a.protected(handler)
	}
	return handler
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
