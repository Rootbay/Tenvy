package agent

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	notes "github.com/rootbay/tenvy-client/internal/modules/notes"
	"github.com/rootbay/tenvy-client/internal/plugins"
	"github.com/rootbay/tenvy-client/internal/protocol"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

type Agent struct {
	id                           string
	key                          string
	baseURL                      string
	client                       *http.Client
	config                       protocol.AgentConfig
	logger                       *log.Logger
	resultMu                     sync.Mutex
	pendingResults               []protocol.CommandResult
	startTime                    time.Time
	metadata                     protocol.AgentMetadata
	sharedSecret                 string
	preferences                  BuildPreferences
	notes                        *notes.Manager
	buildVersion                 string
	timing                       TimingOverride
	modules                      *moduleManager
	commands                     *commandRouter
	connectionFlag               atomic.Uint32
	remoteDesktopInputOnce       sync.Once
	remoteDesktopInputSignalOnce sync.Once
	remoteDesktopInputQueue      chan remoteDesktopInputTask
	remoteDesktopInputStopCh     chan struct{}
	remoteDesktopInputStopped    atomic.Bool
	plugins                      *plugins.Manager
}

func (a *Agent) AgentID() string {
	return a.id
}

func (a *Agent) AgentMetadata() protocol.AgentMetadata {
	return a.metadata
}

func (a *Agent) AgentStartTime() time.Time {
	return a.startTime
}

func (a *Agent) pluginSyncPayload() *manifest.SyncPayload {
	if a.plugins == nil {
		return nil
	}
	payload := a.plugins.Snapshot()
	if payload == nil || len(payload.Installations) == 0 {
		return nil
	}
	return payload
}
