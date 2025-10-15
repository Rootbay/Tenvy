package agent

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	notes "github.com/rootbay/tenvy-client/internal/modules/notes"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type Agent struct {
	id             string
	key            string
	baseURL        string
	client         *http.Client
	config         protocol.AgentConfig
	logger         *log.Logger
	resultMu       sync.Mutex
	pendingResults []protocol.CommandResult
	startTime      time.Time
	metadata       protocol.AgentMetadata
	sharedSecret   string
	preferences    BuildPreferences
	notes          *notes.Manager
	buildVersion   string
	timing         TimingOverride
	modules        *moduleRegistry
	commands       *commandRouter
	connectionFlag atomic.Uint32
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
