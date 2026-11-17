package main

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/auth"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/pvp"
	"chinesechess/internal/store"
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	root := flag.String("root", "..", "project root")
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	absolute, err := filepath.Abs(*root)
	if err != nil {
		log.Error("invalid root")
		os.Exit(1)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	repo, err := store.Open(ctx, dsn)
	cancel()
	if err != nil {
		log.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer repo.Close()
	binary := filepath.Join(absolute, "build", "engine", "xiangqi-engine")
	rules, bots := engine.NewPool(binary, 2, 16), engine.NewPool(binary, 2, 0)
	defer rules.Close()
	defer bots.Close()
	manager, err := game.New(repo, rules, bots, 2)
	if err != nil {
		log.Error("game initialization failed", "error", err)
		os.Exit(1)
	}
	defer manager.Close()
	rooms, err := pvp.New(repo, rules)
	if err != nil {
		log.Error("PVP initialization failed", "error", err)
		return
	}
	defer rooms.Close()
	analysisPool := engine.NewPool(binary, 1, 0)
	defer analysisPool.Close()
	analyses, err := analysis.New(repo, analysisPool, rules, httpapi.AnalysisLoader(manager, rooms))
	if err != nil {
		log.Error("analysis initialization failed", "error", err)
		return
	}
	defer analyses.Close()
	authConfig := auth.Config{PublicURL: os.Getenv("AUTH_PUBLIC_URL"), SMTPAddr: os.Getenv("SMTP_ADDR"), SMTPFrom: os.Getenv("SMTP_FROM"), SMTPUser: os.Getenv("SMTP_USER"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), TrustedProxyIP: os.Getenv("AUTH_TRUSTED_PROXY_IP")}
	if authConfig.PublicURL == "" {
		authConfig.PublicURL = "http://127.0.0.1:8765"
	}
	if authConfig.SMTPAddr == "" {
		authConfig.SMTPAddr = "127.0.0.1:1025"
	}
	if authConfig.SMTPFrom == "" {
		authConfig.SMTPFrom = "chess@example.test"
	}
	authService, err := auth.New(repo.AuthDB(), authConfig, repo.ReloadIdentity)
	if err != nil {
		log.Error("authentication initialization failed")
		return
	}
	server := &http.Server{Addr: *addr, Handler: httpapi.NewWithAuth(manager, rooms, analyses, authService, repo, absolute, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("listen failed", "error", err)
		return
	}
	serveErr := make(chan error, 1)
	go func() {
		log.Info("chess web listening", "address", *addr)
		serveErr <- server.Serve(listener)
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case <-signals:
	case err = <-serveErr:
		if err != http.ErrServerClosed {
			log.Error("HTTP server failed", "error", err)
		}
	}
	signal.Stop(signals)
	analyses.Close()
	rooms.Close()
	manager.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
