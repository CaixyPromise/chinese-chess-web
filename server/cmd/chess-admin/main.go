// chess-admin changes an existing verified account's role from the server console.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"chinesechess/internal/admin"
	"chinesechess/internal/game"
	"chinesechess/internal/store"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 || (os.Args[1] != "grant" && os.Args[1] != "revoke") {
		return fmt.Errorf("usage: chess-admin grant|revoke --email EMAIL --reason REASON")
	}
	flags := flag.NewFlagSet("chess-admin", flag.ContinueOnError)
	email := flags.String("email", "", "existing verified account email")
	reason := flags.String("reason", "", "required audit reason")
	request := flags.String("request-id", game.ID(), "unique audit request identifier")
	if e := flags.Parse(os.Args[2:]); e != nil {
		return e
	}
	if strings.TrimSpace(*email) == "" || strings.TrimSpace(*reason) == "" {
		return fmt.Errorf("email and reason are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	repo, e := store.Open(ctx, dsn)
	if e != nil {
		return fmt.Errorf("database unavailable")
	}
	defer repo.Close()
	service := admin.New(repo.AuthDB())
	target, e := service.RoleTarget(ctx, *email)
	if e != nil {
		return fmt.Errorf("target unavailable")
	}
	role := "admin"
	if os.Args[1] == "revoke" {
		role = "user"
	}
	fmt.Printf("Target: %s <%s>\nID: %s\nRole: %s -> %s; status: %s\nReason: %s\nType the target email to confirm: ", target.Nickname, target.Email, target.ID, target.Role, role, target.Status, *reason)
	line, e := bufio.NewReader(os.Stdin).ReadString('\n')
	if e != nil || strings.TrimSpace(line) != target.Email {
		return fmt.Errorf("confirmation did not match; no role change")
	}
	result, e := service.SetRole(ctx, target.ID, role, admin.Action{Reason: *reason, RequestID: *request, Version: target.Version})
	if e != nil {
		return e
	}
	fmt.Printf("Role set to %s; audit request %s. Revoked sessions are rejected immediately and socket polling closes them.\n", result.Status, result.RequestID)
	return nil
}
