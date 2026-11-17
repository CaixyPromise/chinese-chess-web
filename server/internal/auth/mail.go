package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"net"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

func (s *Service) sendMail(ctx context.Context, email, purpose, token string) error {
	host, _, err := net.SplitHostPort(s.config.SMTPAddr)
	if err != nil {
		return err
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.config.SMTPAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	local := host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if !local {
		return fmt.Errorf("SMTP TLS required")
	}
	if s.config.SMTPUser != "" {
		if err = client.Auth(smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPassword, host)); err != nil {
			return err
		}
	}
	path, subject := "/account/verify", "Verify your chess email"
	if purpose == "reset" {
		path, subject = "/account/reset", "Reset your chess password"
	}
	link := strings.TrimRight(s.config.PublicURL, "/") + path + "#token=" + url.QueryEscape(token)
	if err = client.Mail(s.config.SMTPFrom); err != nil {
		return err
	}
	if err = client.Rcpt(email); err != nil {
		return err
	}
	data, err := client.Data()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(data, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nOpen this link and confirm on the page:\r\n%s\r\n\r\nIf you did not request this, ignore this email.\r\n", s.config.SMTPFrom, email, subject, link)
	if err != nil {
		return err
	}
	if err = data.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// StartMail deliberately returns the same response when an address is already
// registered (register) or absent (reset). SMTP transport errors stay private.
func (s *Service) StartMail(ctx context.Context, email, purpose, ip string, registration ...RegistrationContext) error {
	var task RegistrationContext
	if purpose == "register" && len(registration) > 0 {
		task = registration[0]
		normalized, err := NormalizeReturnTo(task.ReturnTo)
		if err != nil {
			return err
		}
		task.ReturnTo = normalized
	}
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	if !s.limits.allow("mailip:"+ip, 20, time.Hour, 0) || !s.limits.allow("mail:"+email, 5, time.Hour, time.Minute) {
		return problem("RATE_LIMITED", 429)
	}
	var account string
	err = s.db.QueryRow(ctx, `SELECT id FROM auth_accounts WHERE email=$1 AND status='active'`, email).Scan(&account)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if purpose == "register" && account != "" || purpose == "reset" && account == "" {
		return nil
	}
	raw := randomToken()
	expiry := time.Now().Add(30 * time.Minute).UnixMilli()
	if purpose == "reset" {
		expiry = time.Now().Add(15 * time.Minute).UnixMilli()
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE auth_tokens SET used_at=$3 WHERE email=$1 AND purpose=$2 AND used_at=0`, email, purpose, now()); err != nil {
		return err
	}
	if purpose == "register" {
		if _, err = tx.Exec(ctx, `UPDATE auth_registration_grants SET used_at=$2 WHERE email=$1 AND used_at=0`, email, now()); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_tokens(token_hash,purpose,email,account_id,expires_at,guest_owner,return_path) VALUES($1,$2,$3,$4,$5,$6,$7)`, Hash(raw), purpose, email, account, expiry, task.GuestOwner, task.ReturnTo); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if err = s.sendMail(ctx, email, purpose, raw); err != nil {
		_, _ = s.db.Exec(ctx, `UPDATE auth_tokens SET used_at=$2 WHERE token_hash=$1`, Hash(raw), now())
		slog.Warn("authentication email delivery failed", "purpose", purpose)
		return nil
	}
	return nil
}
