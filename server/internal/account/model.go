// Package account defines private personal-center projections and calendar ranges.
package account

import (
	"chinesechess/internal/history"
	"errors"
	"time"
)

const TimeZone = "Asia/Singapore"

var Location = time.FixedZone(TimeZone, 8*60*60)
var ErrInvalidQuery = errors.New("invalid account query")

type Counts struct {
	Win   int `json:"win"`
	Draw  int `json:"draw"`
	Loss  int `json:"loss"`
	Total int `json:"total"`
}
type AnalysisCounts struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Queued    int    `json:"queued"`
	Running   int    `json:"running"`
	Paused    int    `json:"paused"`
	Failed    int    `json:"failed"`
}
type Statistics struct {
	Mode     string `json:"mode"`
	From     string `json:"from"`
	To       string `json:"to"`
	TimeZone string `json:"timeZone"`
	Results  Counts `json:"results"`
	Sides    struct {
		Red   Counts `json:"red"`
		Black Counts `json:"black"`
	} `json:"sides"`
	ExcludedSelfGames int            `json:"excludedSelfGames"`
	Analysis          AnalysisCounts `json:"analysis"`
}
type Active struct {
	ID         string            `json:"id"`
	Mode       string            `json:"mode"`
	Side       string            `json:"side"`
	Phase      string            `json:"phase"`
	MoveCount  int               `json:"moveCount"`
	UpdatedAt  int64             `json:"updatedAt"`
	Difficulty string            `json:"difficulty,omitempty"`
	Practice   *history.Practice `json:"practice,omitempty"`
}
type Analysis struct {
	ID            string `json:"id"`
	SourceMode    string `json:"sourceMode"`
	SourceID      string `json:"sourceId"`
	SourceVersion int64  `json:"sourceVersion"`
	Scope         string `json:"scope"`
	State         string `json:"state"`
	Completed     int    `json:"completed"`
	Total         int    `json:"total"`
	UpdatedAt     int64  `json:"updatedAt"`
}
type Overview struct {
	Active   []Active        `json:"active"`
	Recent   []history.Entry `json:"recent"`
	Analyses []Analysis      `json:"analyses"`
}
type Day struct {
	Date      string `json:"date"`
	Game      int    `json:"game"`
	Practice  int    `json:"practice"`
	Manual    int    `json:"manual"`
	Automatic int    `json:"automatic"`
	Partial   bool   `json:"partial"`
}
type Totals struct {
	Game      int `json:"game"`
	Practice  int `json:"practice"`
	Manual    int `json:"manual"`
	Automatic int `json:"automatic"`
}
type Activity struct {
	From         string `json:"from"`
	To           string `json:"to"`
	TimeZone     string `json:"timeZone"`
	Days         []Day  `json:"days"`
	Totals       Totals `json:"totals"`
	TrackedSince int64  `json:"trackedSince"`
}

func Range(from, to string, now time.Time) (time.Time, time.Time, error) {
	today := now.In(Location)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, Location)
	if from == "" && to == "" {
		return today.AddDate(0, 0, -364), today.AddDate(0, 0, 1), nil
	}
	start, err := time.ParseInLocation(time.DateOnly, from, Location)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidQuery
	}
	end, err := time.ParseInLocation(time.DateOnly, to, Location)
	if err != nil || end.Before(start) || end.After(today) || end.Sub(start) >= 366*24*time.Hour {
		return time.Time{}, time.Time{}, ErrInvalidQuery
	}
	return start, end.AddDate(0, 0, 1), nil
}

func Add(c *Counts, outcome, side string) {
	if side != "red" && side != "black" {
		return
	}
	if outcome != "draw" && outcome != "red_win" && outcome != "black_win" {
		return
	}
	c.Total++
	if outcome == "draw" {
		c.Draw++
	} else if outcome == side+"_win" {
		c.Win++
	} else {
		c.Loss++
	}
}
