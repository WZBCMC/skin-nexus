package plugin

import "context"

type InstallTxRecord struct {
	ID, Plugin, Kind, FromVersion, ToVersion, Source, State, Error string
	ActorUID                                                       int64
}

type InstallTxStore interface {
	CreateInstallTx(ctx context.Context, tx InstallTxRecord) error
	UpdateInstallTx(ctx context.Context, id, state, errText string) error
}

func (m *Manager) installTxStore() InstallTxStore {
	if s, ok := m.platformStore().(InstallTxStore); ok {
		return s
	}
	return nil
}
