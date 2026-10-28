package main

import (
	"context"
	"errors"
	"time"

	"skin-nexus/server/internal/plugin"
	"skin-nexus/server/internal/store"
)

// pluginStore adapts *store.Mysql to plugin.PlatformStore. The plugin package
// cannot import store (that would drag MySQL into every VM test), so main owns
// the translation.
type pluginStore struct{ st *store.Mysql }

func (p pluginStore) GetOption(ctx context.Context, k string) (string, bool, error) {
	return p.st.GetOption(ctx, k)
}

func (p pluginStore) SetOption(ctx context.Context, k, v string) error {
	return p.st.SetOption(ctx, k, v)
}

func (p pluginStore) GetPluginConfig(ctx context.Context, name string) ([]byte, bool, error) {
	raw, err := p.st.GetPluginConfig(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, len(raw) > 0, nil
}

func (p pluginStore) SetPluginConfig(ctx context.Context, name string, raw []byte) error {
	return p.st.SetPluginConfig(ctx, name, raw)
}

func (p pluginStore) GetPluginSecret(ctx context.Context, name, key string) ([]byte, []byte, int, bool, error) {
	s, err := p.st.GetPluginSecret(ctx, name, key)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, 0, false, nil
	}
	if err != nil {
		return nil, nil, 0, false, err
	}
	return s.Ciphertext, s.Nonce, s.KeyVersion, true, nil
}

func (p pluginStore) SetPluginSecret(ctx context.Context, name, key string, ciphertext, nonce []byte, keyVersion int) error {
	return p.st.SetPluginSecret(ctx, name, key, store.PluginSecret{
		Ciphertext: ciphertext, Nonce: nonce, KeyVersion: keyVersion,
	})
}

func (p pluginStore) DeletePluginSecret(ctx context.Context, name, key string) error {
	return p.st.DeletePluginSecret(ctx, name, key)
}

func (p pluginStore) UpsertPluginMeta(ctx context.Context, meta plugin.PluginMetaRecord) error {
	return p.st.UpsertPluginMeta(ctx, store.PluginMeta{
		Name: meta.Name, Version: meta.Version, HookAPI: meta.HookAPI, Runtime: meta.Runtime,
		Manifest: meta.Manifest, Config: meta.Config, SchemaVersion: meta.SchemaVersion,
		Source: meta.Source, Checksum: meta.Checksum, State: meta.State, LastError: meta.LastError,
		InstalledAt: meta.InstalledAt, UpdatedAt: meta.UpdatedAt,
	})
}

func (p pluginStore) UpdatePluginState(ctx context.Context, name, state, lastErr string) error {
	return p.st.UpdatePluginState(ctx, name, state, lastErr)
}

func (p pluginStore) GetPluginKV(ctx context.Context, name, ns, key string) (string, *time.Time, bool, error) {
	return p.st.GetPluginKV(ctx, name, ns, key)
}

func (p pluginStore) SetPluginKV(ctx context.Context, name, ns, key, value string, expires *time.Time) error {
	return p.st.SetPluginKV(ctx, name, ns, key, value, expires)
}

func (p pluginStore) DeletePluginKV(ctx context.Context, name, ns, key string) error {
	return p.st.DeletePluginKV(ctx, name, ns, key)
}

func (p pluginStore) ListPluginKV(ctx context.Context, name, ns, prefix string, limit int) ([]plugin.KVRecord, error) {
	rows, err := p.st.ListPluginKV(ctx, name, ns, prefix, limit)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.KVRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, plugin.KVRecord{Namespace: r.Namespace, Key: r.Key, Value: r.Value, ExpiresAt: r.ExpiresAt, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

func (p pluginStore) CountPluginKV(ctx context.Context, name string) (int, error) {
	return p.st.CountPluginKV(ctx, name)
}

func (p pluginStore) GetPluginDoc(ctx context.Context, name, coll, id string) ([]byte, bool, error) {
	d, err := p.st.GetPluginDoc(ctx, name, coll, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return d.Doc, true, nil
}

func (p pluginStore) PutPluginDoc(ctx context.Context, name, coll, id string, doc []byte) error {
	return p.st.PutPluginDoc(ctx, name, coll, id, doc)
}

func (p pluginStore) DeletePluginDoc(ctx context.Context, name, coll, id string) error {
	return p.st.DeletePluginDoc(ctx, name, coll, id)
}

func (p pluginStore) ListPluginDocs(ctx context.Context, name, coll, prefix string, limit int) ([]plugin.DocRecord, error) {
	rows, err := p.st.ListPluginDocs(ctx, name, coll, prefix, limit)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.DocRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, plugin.DocRecord{Collection: r.Collection, ID: r.ID, Doc: r.Doc, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

func (p pluginStore) CountPluginDocs(ctx context.Context, name, coll string) (int, error) {
	return p.st.CountPluginDocs(ctx, name, coll)
}

func (p pluginStore) PluginMigrationLatest(ctx context.Context, name string) (int, error) {
	return p.st.PluginMigrationLatest(ctx, name)
}

func (p pluginStore) RecordPluginMigration(ctx context.Context, name string, version int) error {
	return p.st.RecordPluginMigration(ctx, name, version)
}
