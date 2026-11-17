// Package analysis owns frozen, authorized analysis tasks separately from games.
package analysis

import "context"

const EngineVersion = "0.5.2"

type Source struct {
	Mode    string   `json:"mode"`
	ID      string   `json:"id"`
	Version int64    `json:"version"`
	Moves   []string `json:"moves"`
	Rules   string   `json:"rulesVersion"`
	Phase   string   `json:"phase"`
	Fen     string   `json:"fen"`
	Outcome string   `json:"outcome"`
	Reason  string   `json:"reason,omitempty"`
	GroupID string   `json:"groupId"`
	Origin  string   `json:"origin"`
	Keys    []string `json:"ruleKeys,omitempty"`
	Checks  []bool   `json:"ruleChecks,omitempty"`
}

type Job struct {
	ViewerSide string `json:"viewerSide,omitempty"`
	ID         string `json:"id"`
	Owner      string `json:"-"`
	Source     Source `json:"source"`
	Digest     string `json:"digest"`
	Engine     string `json:"engineVersion"`
	Budget     Budget `json:"budget"`
	Scope      string `json:"scope"`
	Profile    string `json:"profile"`
	Ply        int    `json:"ply"`
	State      string `json:"state"`
	Generation int64  `json:"generation"`
	NextPly    int    `json:"nextPly"`
	Completed  int    `json:"completed"`
	Total      int    `json:"total"`
	Automatic  bool   `json:"automatic"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Created    int64  `json:"createdAt"`
	Updated    int64  `json:"updatedAt"`
}

type Line struct {
	Score             int      `json:"score"` // normalized red perspective, legacy units
	Depth             int      `json:"depth"`
	Reliable          bool     `json:"reliable"`
	Kind              string   `json:"kind"` // evaluation, mate, draw, unavailable
	PV                []string `json:"pv"`
	Move              string   `json:"move,omitempty"`
	Millis            int      `json:"elapsedMs"`
	NativeScore       int      `json:"nativeScore"`
	NativePerspective string   `json:"nativeScorePerspective"`
	Producer          string   `json:"producer"`
	StopReason        string   `json:"stopReason"`
	CompletedNanos    uint64   `json:"completedNanos"`
	Nodes             uint64   `json:"nodes"`
	Evaluations       uint64   `json:"evaluations"`
	CacheHits         uint64   `json:"cacheHits"`
	Cutoffs           uint64   `json:"cutoffs"`
}

type Budget struct {
	Depth   int    `json:"depth"`
	Millis  int    `json:"timeMs"`
	HashMiB int    `json:"hashMiB"`
	Nodes   uint64 `json:"nodes"`
}

type Candidate struct {
	Move string `json:"move"`
	Role string `json:"role"` // teacher_recommended, never the played move by implication
	Line Line   `json:"line"`
}

type Position struct {
	SchemaVersion    int         `json:"schemaVersion"`
	AnalysisVersion  string      `json:"analysisVersion"`
	SampleID         string      `json:"sampleId"`
	PositionID       string      `json:"positionId"`
	GroupID          string      `json:"groupId"`
	SourceMode       string      `json:"sourceMode"`
	SourceID         string      `json:"sourceId"`
	SourceVersion    int64       `json:"sourceVersion"`
	Origin           string      `json:"origin"`
	Fen              string      `json:"fen"`
	Turn             string      `json:"turn"`
	History          []string    `json:"history"`
	Rules            string      `json:"rulesVersion"`
	Teacher          string      `json:"teacherEngineVersion"`
	Budget           Budget      `json:"budget"`
	ScorePerspective string      `json:"scorePerspective"`
	ScoreUnits       string      `json:"scoreUnits"`
	Outcome          string      `json:"outcome"`
	EndReason        string      `json:"endReason,omitempty"`
	Final            bool        `json:"resultFinal"`
	Candidates       []Candidate `json:"candidates"`
	Ply              int         `json:"ply"`
	Best             Line        `json:"best"`
	Actual           *Line       `json:"actual,omitempty"`
	ActualMove       string      `json:"actualMove,omitempty"`
	Comparable       bool        `json:"comparable"`
	Loss             int         `json:"loss"` // actor perspective; only meaningful when comparable
	Note             string      `json:"note,omitempty"`
}

type Page struct {
	Items []Position `json:"items"`
	Next  int        `json:"nextCursor,omitempty"`
}

type Request struct {
	SnapshotID string `json:"snapshotId,omitempty"`
	Mode       string `json:"sourceMode"`
	ID         string `json:"sourceId"`
	Version    int64  `json:"expectedVersion"`
	Scope      string `json:"scope"`
	Profile    string `json:"profile"`
	Ply        int    `json:"ply"`
}

type Repository interface {
	SaveReplaySnapshot(context.Context, string, Source) (string, error)
	FindReplaySnapshot(context.Context, string, string) (Source, error)
	CreateAnalysis(context.Context, Job) (Job, error)
	FindAnalysis(context.Context, string, string) (Job, error)
	ListAnalyses(context.Context, string, string, string, int64) ([]Job, error)
	ReadyAnalyses(context.Context, int) ([]Job, error)
	SaveAnalysis(context.Context, Job, int64, string) error
	CompletePosition(context.Context, Job, int64, Position, string) error
	AnalysisPositions(context.Context, string, string, int, int) (Page, error)
	CachedAnalysisPosition(context.Context, string, int) (Position, bool, error)
	RecoverAnalyses(context.Context, string) error
	MaterializeAnalyses(context.Context, string) error
}
