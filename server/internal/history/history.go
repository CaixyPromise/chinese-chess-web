// Package history defines authenticated history pagination shared by both game modes.
package history

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

var ErrInvalidQuery = errors.New("invalid history query")

type Position struct {
	UpdatedAt int64  `json:"updatedAt"`
	ID        string `json:"id"`
	Mode      string `json:"mode"`
	Side      string `json:"side,omitempty"`
}

type Query struct {
	Mode   string
	Limit  int
	Before *Position
}

type Practice struct {
	SourceGroupID string `json:"sourceGroupId,omitempty"`
	SourceMode    string `json:"sourceMode"`
	SourceID      string `json:"sourceId"`
	SourceVersion int64  `json:"sourceVersion"`
	Ply           int    `json:"ply"`
}

type Entry struct {
	Practice     *Practice `json:"practice,omitempty"`
	ID           string    `json:"id"`
	Mode         string    `json:"mode"`
	Side         string    `json:"side"`
	RulesVersion string    `json:"rulesVersion"`
	Outcome      string    `json:"outcome"`
	Reason       string    `json:"reason,omitempty"`
	MoveCount    int       `json:"moveCount"`
	UpdatedAt    int64     `json:"updatedAt"`
}

type Page struct {
	Items      []Entry `json:"items"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

type cursor struct {
	Version int    `json:"v"`
	Filter  string `json:"filter"`
	Position
}

func validMode(mode string) bool { return mode == "all" || mode == "pve" || mode == "pvp" }
func validPosition(p Position) bool {
	if p.UpdatedAt < 0 || len(p.ID) != 32 || (p.Mode != "pve" && p.Mode != "pvp") || (p.Side != "" && p.Side != "red" && p.Side != "black") {
		return false
	}
	_, err := hex.DecodeString(p.ID)
	return err == nil
}

func (q Query) Validate() error {
	if !validMode(q.Mode) || q.Limit < 1 || q.Limit > 50 {
		return ErrInvalidQuery
	}
	if q.Before != nil && (!validPosition(*q.Before) || (q.Mode != "all" && q.Mode != q.Before.Mode)) {
		return ErrInvalidQuery
	}
	return nil
}

func ParseQuery(mode, token string, limit int) (Query, error) {
	if mode == "" {
		mode = "all"
	}
	if limit == 0 {
		limit = 20
	}
	q := Query{Mode: mode, Limit: limit}
	if err := q.Validate(); err != nil {
		return Query{}, err
	}
	if token == "" {
		return q, nil
	}
	if len(token) > 512 {
		return Query{}, ErrInvalidQuery
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || base64.RawURLEncoding.EncodeToString(b) != token {
		return Query{}, ErrInvalidQuery
	}
	var c cursor
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&c); err != nil {
		return Query{}, ErrInvalidQuery
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return Query{}, ErrInvalidQuery
	}
	if c.Version != 1 || c.Filter != mode {
		return Query{}, ErrInvalidQuery
	}
	// Accept only our canonical encoding, rejecting missing/duplicate keys and alternate forms.
	canonical, err := json.Marshal(c)
	if err != nil || !bytes.Equal(canonical, b) {
		return Query{}, ErrInvalidQuery
	}
	q.Before = &c.Position
	if err = q.Validate(); err != nil {
		return Query{}, err
	}
	return q, nil
}

func Cursor(mode string, e Entry) string {
	b, _ := json.Marshal(cursor{Version: 1, Filter: mode, Position: Position{UpdatedAt: e.UpdatedAt, ID: e.ID, Mode: e.Mode, Side: e.Side}})
	return base64.RawURLEncoding.EncodeToString(b)
}
