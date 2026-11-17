package integration

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/game"
	"encoding/json"
	"errors"
	"testing"
)

func TestAdminBannedAliasCannotCreateOrMaterializeAnalysis(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	owner, alias := game.ID(), game.ID()
	_, err := conn.Exec(ctx, `INSERT INTO auth_accounts(id,email,password_hash,nickname,created_at,status)VALUES($1,$2,'unused','test',1,'banned')`, owner, owner+"@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO auth_owner_links(owner_key,account_id,created_at)VALUES($1,$2,1)`, alias, owner); err != nil {
		t.Fatal(err)
	}
	j := analysisJob(alias)
	_, err = repo.CreateAnalysis(ctx, j)
	var ge *game.Error
	if !errors.As(err, &ge) || ge.Code != "ACCOUNT_BANNED" {
		t.Fatalf("banned create: %v", err)
	}
	source, _ := json.Marshal(j.Source)
	owners, _ := json.Marshal([]string{alias, "other-guest"})
	_, err = conn.Exec(ctx, `INSERT INTO analysis_finished_events(mode,source_id,source_version,source,owners,created_at,frozen_record)VALUES('pve',$1,7,$2,$3,1,'{}')`, j.Source.ID, source, owners)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM analysis_jobs WHERE owner_key=$1`, alias).Scan(&n); err != nil || n != 0 {
		t.Fatalf("banned auto job %d %v", n, err)
	}
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM analysis_jobs WHERE owner_key='other-guest'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("opponent auto job %d %v", n, err)
	}
}
