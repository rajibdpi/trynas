package core

import (
	"time"
)

type NetworkSample struct {
	Received, Sent uint64
	At             time.Time
	RX, TX         float64
}
