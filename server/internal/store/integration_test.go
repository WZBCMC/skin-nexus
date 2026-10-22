//go:build integration

// integration_test.go: MySQL round-trip coverage for the 0003 (OIDC) and
// 0004 (Union) schemas plus the web/session/audit helpers they rely on.
//
// Runs outside the default build (CGO-free dev boxes have no MySQL):
//
//	SKIN_TEST_DSN='skin:pass@tcp(127.0.0.1:3306)/skin_nexus_test?parseTime=true' \
//	  go test -tags integration ./internal/store/
//
// The target database MUST be empty/disposable: the suite drops and
// re-creates every table it touches, idempotently. CI runs it against a
// MySQL 8 service container (see .github/workflows/ci.yml).
package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"skin-nexus/server/internal/migrate"
	"skin-nexus/server/internal/oidc"
)

// testTables mirrors the DDL the suite owns; dropped before apply so reruns
// on a reused database stay green.
var testTables = []string{
	"oidc_device_auth", "oidc_refresh_tokens", "oidc_access_tokens",
	"oidc_auth_requests", "oidc_clients",
	"union_kv", "web_sessions", "audit_log", "options",
	"ygg_join_sessions", "ygg_tokens", "uuid_map", "textures", "players", "users",
}

func openTestStore(t *testing.T) *Mysql {
	t.Helper()
	dsn := os.Getenv("SKIN_TEST_DSN")
	if dsn == "" {
		t.Skip("SKIN_TEST_DSN not set; skipping MySQL integration suite")
	}
	m, err := Open(dsn, "https://skin.test", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	ctx := context.Background()
	_, _ = m.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0")
	for _, tbl := range testTables {
		_, _ = m.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tbl)
	}
	applyMigrations(t, m)
	return m
}

// applyMigrations executes migrations/*.sql in filename order via the shared
// migrate.ApplySQLDir helper (also used by the empty-DB boot path).
func applyMigrations(t *testing.T, m *Mysql) {
	t.Helper()
	dir := filepath.Join("..", "..", "migrations")
	if err := migrate.ApplySQLDir(m.db, dir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}

func seedUser(t *testing.T, m *Mysql) int64 {
	t.Helper()
	uid, err := m.CreateUser(context.Background(), "it@example.com", "it_user", "x", 3)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return uid
}

// ---- 0003 OIDC tables ----

func TestIntegrationOidcRoundTrips(t *testing.T) {
	m := openTestStore(t)
	ctx := context.Background()

	// clients: upsert -> get -> list -> delete.
	rec := oidc.ClientRecord{
		ID: "it-client", Name: "Integration", SecretHash: "$argon2id$fake",
		RedirectURIs: []string{"https://a/cb"}, GrantTypes: []string{"authorization_code"},
	}
	if err := m.EnsureOidcClient(ctx, rec); err != nil {
		t.Fatalf("ensure client: %v", err)
	}
	got, err := m.GetClient(ctx, "it-client")
	if err != nil || got.Name != "Integration" || len(got.RedirectURIs) != 1 {
		t.Fatalf("get client = %+v %v", got, err)
	}
	if list, err := m.ListClients(ctx); err != nil || len(list) == 0 {
		t.Fatalf("list clients = %v %v", list, err)
	}
	if err := m.DeleteClient(ctx, "it-client"); err != nil {
		t.Fatalf("delete client: %v", err)
	}
	if _, err := m.GetClient(ctx, "it-client"); err != oidc.ErrNotFound {
		t.Fatalf("deleted client err = %v", err)
	}

	uid := seedUser(t, m)

	// access tokens: save -> get -> list live -> revoke one -> list empty.
	uidNum := uid
	at := oidc.AccessTokenRecord{
		JTI: "jti-1", UserID: uidNum, ClientID: "2",
		Scopes:    []string{"openid", "Yggdrasil.Server.Join"},
		ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}
	if err := m.SaveAccessToken(ctx, at); err != nil {
		t.Fatalf("save at: %v", err)
	}
	if got, err := m.GetAccessToken(ctx, "jti-1"); err != nil || got.UserID != uidNum {
		t.Fatalf("get at = %+v %v", got, err)
	}
	live, err := m.ListUserAccessTokens(ctx, uidNum)
	if err != nil || len(live) != 1 || live[0].JTI != "jti-1" {
		t.Fatalf("list live = %v %v", live, err)
	}
	if err := m.RevokeAccessToken(ctx, "jti-1"); err != nil {
		t.Fatalf("revoke at: %v", err)
	}
	if live, _ := m.ListUserAccessTokens(ctx, uidNum); len(live) != 0 {
		t.Fatalf("revoked token still live: %v", live)
	}

	// refresh tokens: save -> rotate -> old dead, new live -> client-scoped revoke.
	old := oidc.RefreshTokenRecord{
		Token: "rt-old", UserID: uidNum, ClientID: "2",
		ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}
	newTok := oidc.RefreshTokenRecord{
		Token: "rt-new", UserID: uidNum, ClientID: "2",
		ExpiresAt: time.Now().Add(2 * time.Hour), CreatedAt: time.Now(),
	}
	if err := m.SaveRefreshToken(ctx, old); err != nil {
		t.Fatalf("save rt: %v", err)
	}
	if err := m.RotateRefreshToken(ctx, old, newTok); err != nil {
		t.Fatalf("rotate rt: %v", err)
	}
	if got, _ := m.GetRefreshToken(ctx, "rt-old"); !got.Revoked {
		t.Fatal("old refresh not revoked after rotate")
	}
	if got, err := m.GetRefreshToken(ctx, "rt-new"); err != nil || got.Revoked {
		t.Fatalf("new refresh = %+v %v", got, err)
	}
	if err := m.RevokeClientRefreshTokens(ctx, uidNum, "2"); err != nil {
		t.Fatalf("client revoke: %v", err)
	}
	if got, _ := m.GetRefreshToken(ctx, "rt-new"); !got.Revoked {
		t.Fatal("client-scoped revoke missed live refresh")
	}
	// user-wide revoke path
	if err := m.RevokeUserRefreshTokens(ctx, uidNum); err != nil {
		t.Fatalf("user revoke: %v", err)
	}

	// device auth: save -> get by both codes -> update -> duplicate code.
	da := oidc.DeviceAuthRecord{
		DeviceCode: "dev-1", UserCode: "AAAA-BBBB", ClientID: "2",
		Scopes: []string{"openid"}, ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}
	if err := m.SaveDeviceAuth(ctx, da); err != nil {
		t.Fatalf("save device: %v", err)
	}
	if _, err := m.GetDeviceAuthByUserCode(ctx, "AAAA-BBBB"); err != nil {
		t.Fatalf("get by user code: %v", err)
	}
	da.Done, da.UserID, da.SelectedPID = true, uidNum, 7
	if err := m.UpdateDeviceAuth(ctx, da); err != nil {
		t.Fatalf("update device: %v", err)
	}
	if got, _ := m.GetDeviceAuthByDeviceCode(ctx, "dev-1"); !got.Done || got.SelectedPID != 7 {
		t.Fatalf("device after update = %+v", got)
	}
	dup := da
	dup.DeviceCode = "dev-2"
	if err := m.SaveDeviceAuth(ctx, dup); err == nil {
		t.Fatal("duplicate user_code must fail")
	}
}

// ---- 0004 union_kv + audit + options ----

func TestIntegrationUnionAndAudit(t *testing.T) {
	m := openTestStore(t)
	ctx := context.Background()

	// union_kv round trip
	if err := m.SetUnionKV(ctx, "k1", "v1"); err != nil {
		t.Fatalf("set kv: %v", err)
	}
	if v, ok, err := m.GetUnionKV(ctx, "k1"); err != nil || !ok || v != "v1" {
		t.Fatalf("get kv = %q %v %v", v, ok, err)
	}
	if err := m.SetUnionKV(ctx, "k1", "v2"); err != nil {
		t.Fatalf("overwrite kv: %v", err)
	}
	if v, _, _ := m.GetUnionKV(ctx, "k1"); v != "v2" {
		t.Fatalf("kv overwrite = %q", v)
	}
	if _, ok, err := m.GetUnionKV(ctx, "missing"); err != nil || ok {
		t.Fatalf("missing kv = %v %v", ok, err)
	}

	// options round trip (member key persistence path)
	if err := m.SetOption(ctx, "union_member_key", "mk-1"); err != nil {
		t.Fatalf("set option: %v", err)
	}
	if v, ok, err := m.GetOption(ctx, "union_member_key"); err != nil || !ok || v != "mk-1" {
		t.Fatalf("get option = %q %v %v", v, ok, err)
	}

	// audit: insert (system + user actor) -> RecentAudit sees them.
	uid := seedUser(t, m)
	m.InsertAudit(ctx, 0, "union.member_key_rotated", map[string]any{"k": "v"})
	m.InsertAudit(ctx, uid, "oidc.device_approve", map[string]any{"pid": 7})
	rows, err := m.RecentAudit(ctx, 10)
	if err != nil || len(rows) < 2 {
		t.Fatalf("recent audit = %v %v", rows, err)
	}
	found := map[string]bool{}
	for _, r := range rows {
		found[r["action"].(string)] = true
	}
	if !found["union.member_key_rotated"] || !found["oidc.device_approve"] {
		t.Fatalf("audit rows missing: %v", found)
	}
}

func TestIntegrationProfileHooks(t *testing.T) {
	m := openTestStore(t)
	ctx := context.Background()
	uid := seedUser(t, m)

	pid, uuid, err := m.CreatePlayer(ctx, uid, "HookSteve")
	if err != nil || pid == 0 || uuid == "" {
		t.Fatalf("create player = %d %q %v", pid, uuid, err)
	}

	// ProfileNameByUUID powers union POST /sync.
	list, err := m.ProfileNameByUUID(ctx)
	if err != nil || list[uuid] != "HookSteve" {
		t.Fatalf("profile list = %v %v", list, err)
	}

	// EmailByPlayerName powers queryemail.
	if email, ok, err := m.EmailByPlayerName(ctx, "hooksteve"); err != nil || !ok || email != "it@example.com" {
		t.Fatalf("email lookup = %q %v %v", email, ok, err)
	}
	if _, ok, err := m.EmailByPlayerName(ctx, "NoSuchPlayer"); err != nil || ok {
		t.Fatalf("missing player lookup ok=%v err=%v", ok, err)
	}

	// RemapUUIDs applies hub-issued batch; only mapped rows move.
	if err := m.RemapUUIDs(ctx, map[string]string{uuid: "cafebabecafebabecafebabecafebabe"}); err != nil {
		t.Fatalf("remap: %v", err)
	}
	if p, err := m.PlayerByUUID(ctx, "cafebabecafebabecafebabecafebabe"); err != nil || p.ID != pid {
		t.Fatalf("after remap = %+v %v", p, err)
	}
	// Unknown old uuid: no error, nothing moved.
	if err := m.RemapUUIDs(ctx, map[string]string{"0000000000000000000000000000dead": "0000000000000000000000000000beef"}); err != nil {
		t.Fatalf("remap unknown: %v", err)
	}
}

// ---- 0002 sessions ----

func TestIntegrationSessionLifecycle(t *testing.T) {
	m := openTestStore(t)
	ctx := context.Background()
	uid := seedUser(t, m)
	tok := "it-session-token"
	if err := m.CreateSession(ctx, tok, uid, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if got, err := m.SessionUser(ctx, tok); err != nil || got != uid {
		t.Fatalf("session user = %d %v", got, err)
	}
	if err := m.DeleteSession(ctx, tok); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := m.SessionUser(ctx, tok); err == nil {
		t.Fatal("deleted session must not resolve")
	}
}

// ---- login lookup + password verify round trip ----
// Regression anchor for the 2026-10-23 incident: `? COLLATE ...` in
// UserByIdentifier was a syntax error on MySQL 5.7 and made every login fail
// while the register->session path stayed healthy. Pins the lookup and verify
// path against a real database (8.0 here; 5.7 compatibility is a SQL-shape
// property — keep COLLATE on the column expression).

func TestIntegrationLoginRoundTrip(t *testing.T) {
	m := openTestStore(t)
	ctx := context.Background()
	uid := seedUser(t, m)

	// identifier lookup: nickname hit, case-insensitive email hit, miss
	if u, err := m.UserByIdentifier(ctx, "it_user"); err != nil || u.ID != uid {
		t.Fatalf("by nickname = %+v %v", u, err)
	}
	if u, err := m.UserByIdentifier(ctx, "IT@Example.com"); err != nil || u.ID != uid {
		t.Fatalf("by email ci = %+v %v", u, err)
	}
	if _, err := m.UserByIdentifier(ctx, "missing_user"); err == nil {
		t.Fatal("missing identifier must error")
	}

	// password verify: right password passes, wrong fails
	if ok, err := m.VerifyPassword(ctx, uid, "hunter2-xyz"); err != nil || !ok {
		t.Fatalf("verify ok = %v %v", ok, err)
	}
	if ok, err := m.VerifyPassword(ctx, uid, "wrong-pass"); err != nil || ok {
		t.Fatalf("verify wrong = %v %v", ok, err)
	}
}
