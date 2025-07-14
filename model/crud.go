package model

import (
	"net"
	"time"
)

type LogEntry struct {
	Timestamp time.Time `ch:"timestamp"`
	Method    string    `ch:"method"`
	Path      string    `ch:"path"`
	Status    uint16    `ch:"status"`
	LatencyMs uint32    `ch:"latency_ms"`
	IP        net.IP    `ch:"ip"`
}
