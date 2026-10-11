package core

import "sync"

type StorageConnection struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Provider string            `json:"provider"`
	Root     string            `json:"root"`
	Attached bool              `json:"attached"`
	Options  map[string]string `json:"options,omitempty"`
}

type ProviderState struct {
	Mu        sync.Mutex
	RemoteOps int
}

type EncryptedConnections struct {
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}
