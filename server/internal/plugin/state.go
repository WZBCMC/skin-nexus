package plugin

import (
	"context"
	"time"
)

// PluginMetaRecord is the store-neutral lifecycle row. The main package adapts
// *store.Mysql to PlatformStore; the plugin package never imports store.
type PluginMetaRecord struct {
	Name          string
	Version       string
	HookAPI       int
	Runtime       string
	Manifest      []byte
	Config        []byte
	SchemaVersion int
	Source        string
	Checksum      string
	State         string
	LastError     string
	InstalledAt   time.Time
	UpdatedAt     time.Time
}

// PlatformStore is the persistence seam for the v2 host. Implementations must
// namespace every method by plugin name; there is deliberately no raw SQL or
// host-table access here.
type PlatformStore interface {
	StateStore

	GetPluginConfig(ctx context.Context, name string) ([]byte, bool, error)
	SetPluginConfig(ctx context.Context, name string, raw []byte) error

	GetPluginSecret(ctx context.Context, plugin, key string) (ciphertext, nonce []byte, keyVersion int, ok bool, err error)
	SetPluginSecret(ctx context.Context, plugin, key string, ciphertext, nonce []byte, keyVersion int) error
	DeletePluginSecret(ctx context.Context, plugin, key string) error

	UpsertPluginMeta(ctx context.Context, meta PluginMetaRecord) error
	UpdatePluginState(ctx context.Context, name, state, lastErr string) error

	GetPluginKV(ctx context.Context, plugin, ns, key string) (value string, expires *time.Time, ok bool, err error)
	SetPluginKV(ctx context.Context, plugin, ns, key, value string, expires *time.Time) error
	DeletePluginKV(ctx context.Context, plugin, ns, key string) error
	ListPluginKV(ctx context.Context, plugin, ns, prefix string, limit int) ([]KVRecord, error)
	CountPluginKV(ctx context.Context, plugin string) (int, error)

	GetPluginDoc(ctx context.Context, plugin, coll, id string) (doc []byte, ok bool, err error)
	PutPluginDoc(ctx context.Context, plugin, coll, id string, doc []byte) error
	DeletePluginDoc(ctx context.Context, plugin, coll, id string) error
	ListPluginDocs(ctx context.Context, plugin, coll, prefix string, limit int) ([]DocRecord, error)
	CountPluginDocs(ctx context.Context, plugin, coll string) (int, error)

	PluginMigrationLatest(ctx context.Context, plugin string) (int, error)
	RecordPluginMigration(ctx context.Context, plugin string, version int) error
}

type KVRecord struct {
	Namespace string
	Key       string
	Value     string
	ExpiresAt *time.Time
	UpdatedAt time.Time
}

type DocRecord struct {
	Collection string
	ID         string
	Doc        []byte
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (m *Manager) platformStore() PlatformStore {
	if m.cfg.Store != nil {
		return m.cfg.Store
	}
	return nil
}

func (m *Manager) enabledOption(ctx context.Context, name string) bool {
	if st := m.platformStore(); st != nil {
		v, ok, err := st.GetOption(ctx, optionKey(name))
		return err == nil && ok && v == "1"
	}
	if m.cfg.State == nil {
		return false
	}
	v, ok, err := m.cfg.State.GetOption(ctx, optionKey(name))
	return err == nil && ok && v == "1"
}

func (m *Manager) setEnabledOption(ctx context.Context, name string, on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if st := m.platformStore(); st != nil {
		return st.SetOption(ctx, optionKey(name), v)
	}
	if m.cfg.State == nil {
		return nil
	}
	return m.cfg.State.SetOption(ctx, optionKey(name), v)
}
