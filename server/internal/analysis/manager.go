package analysis

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"context"
	"sync"
	"time"
)

type Loader func(context.Context, string, string, string) (Source, error)

type Manager struct {
	repo             Repository
	pool, rules      *engine.Pool
	load             Loader
	ctx              context.Context
	cancel           context.CancelFunc
	done             chan struct{}
	outboxDone       chan struct{}
	mu               sync.Mutex
	activeID         string
	activeOwner      string
	activeGeneration int64
	stop             context.CancelFunc
	lastOwner        string
}

func New(repo Repository, pool, rules *engine.Pool, load Loader) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{repo: repo, pool: pool, rules: rules, load: load, ctx: ctx, cancel: cancel, done: make(chan struct{}), outboxDone: make(chan struct{})}
	setup, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	if err := repo.RecoverAnalyses(setup, EngineVersion); err != nil {
		cancel()
		return nil, err
	}
	go m.work()
	go m.materialize()
	return m, nil
}

func (m *Manager) Close() { m.cancel(); <-m.done; <-m.outboxDone }

func problem(code string) error { return &game.Error{Code: code} }

func (m *Manager) Create(ctx context.Context, owner string, r Request) (Job, error) {
	if (r.Mode != "pve" && r.Mode != "pvp") || !game.ValidID(r.ID) || r.Version < 1 ||
		(r.Scope != "position" && r.Scope != "game") || (r.Profile != "standard" && r.Profile != "deep") ||
		(r.Scope == "game" && r.Profile != "standard") {
		return Job{}, problem("INVALID_REQUEST")
	}
	source, err := m.load(ctx, owner, r.Mode, r.ID)
	if err != nil {
		return Job{}, err
	}
	if source.Version != r.Version {
		if r.SnapshotID == "" {
			return Job{}, problem("VERSION_CONFLICT")
		}
	}
	if r.SnapshotID != "" {
		frozen, err := m.repo.FindReplaySnapshot(ctx, owner, r.SnapshotID)
		if err != nil {
			return Job{}, err
		}
		if frozen.Mode != r.Mode || frozen.ID != r.ID || frozen.Version != r.Version {
			return Job{}, problem("INVALID_REQUEST")
		}
		if r.Mode == "pvp" && (source.Phase != "finished" || frozen.Phase != "finished") {
			return Job{}, problem("ANALYSIS_NOT_AVAILABLE")
		}
		source = frozen
	}
	if source.Mode == "pvp" && source.Phase != "finished" {
		return Job{}, problem("ANALYSIS_NOT_AVAILABLE")
	}
	if !chess.SupportedRules(source.Rules) {
		return Job{}, problem("UNSUPPORTED_RULES")
	}
	if r.Ply < 0 || r.Ply > len(source.Moves) {
		return Job{}, problem("INVALID_REQUEST")
	}
	return m.createFrozen(ctx, owner, source, r.Scope, r.Profile, r.Ply)
}

func (m *Manager) createFrozen(ctx context.Context, owner string, source Source, scope, profile string, ply int) (Job, error) {
	now := time.Now().UnixMilli()
	j := Job{ID: game.ID(), Owner: owner, Source: source, Digest: Digest(source), Engine: EngineVersion,
		Scope: scope, Profile: profile, Ply: ply, NextPly: ply, State: "queued", Generation: 1, Total: 1, Created: now, Updated: now}
	j.Budget = ProfileBudget(profile)
	if scope == "game" {
		j.Ply = 0
		j.NextPly = 0
		j.Total = len(source.Moves) + 1
		if len(source.Moves) == 0 {
			j.Total = 0
			j.State = "completed"
		}
	}
	return m.repo.CreateAnalysis(ctx, j)
}

func (m *Manager) Get(ctx context.Context, owner, id string) (Job, error) {
	return m.repo.FindAnalysis(ctx, owner, id)
}

func (m *Manager) Freeze(ctx context.Context, owner string, source Source) (string, error) {
	return m.repo.SaveReplaySnapshot(ctx, owner, source)
}

func (m *Manager) Frozen(ctx context.Context, owner, id string) (Source, error) {
	return m.repo.FindReplaySnapshot(ctx, owner, id)
}

func (m *Manager) List(ctx context.Context, owner, mode, id string, version int64) ([]Job, error) {
	if (mode != "pve" && mode != "pvp") || !game.ValidID(id) || version < 1 {
		return nil, problem("INVALID_REQUEST")
	}
	if _, err := m.load(ctx, owner, mode, id); err != nil {
		return nil, err
	}
	return m.repo.ListAnalyses(ctx, owner, mode, id, version)
}

func (m *Manager) Positions(ctx context.Context, owner, id string, cursor, limit int) (Page, error) {
	if cursor < 0 || limit < 1 || limit > 100 {
		return Page{}, problem("INVALID_REQUEST")
	}
	return m.repo.AnalysisPositions(ctx, owner, id, cursor, limit)
}
