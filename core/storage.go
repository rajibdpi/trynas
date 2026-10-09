package core

import (
	"context"
	"sync"

	"github.com/rclone/rclone/fs/accounting"
)

type StorageConnection struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Provider string            `json:"provider"`
	Root     string            `json:"root"`
	Attached bool              `json:"attached"`
	Options  map[string]string `json:"options,omitempty"`
}

type TransferJob struct {
	ID         string                `json:"id"`
	Connection string                `json:"connection"`
	Direction  string                `json:"direction"`
	Mode       string                `json:"mode"`
	LocalPath  string                `json:"localPath"`
	DryRun     bool                  `json:"dryRun"`
	Status     string                `json:"status"`
	Started    string                `json:"started"`
	Finished   string                `json:"finished,omitempty"`
	Bytes      int64                 `json:"bytes"`
	Files      int64                 `json:"files"`
	Deletes    int64                 `json:"deletes"`
	Stats      *accounting.StatsInfo `json:"-"`
	Cancel     context.CancelFunc    `json:"-"`
}

type ProviderState struct {
	Mu        sync.Mutex
	Job       *TransferJob
	Group     string
	RemoteOps int
}

func (j *TransferJob) UpdateStats() {
	if j.Stats != nil {
		j.Bytes = j.Stats.GetBytes()
		j.Files = j.Stats.GetTransfers()
		j.Deletes = j.Stats.GetDeletes()
	}
}

type EncryptedConnections struct {
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}
