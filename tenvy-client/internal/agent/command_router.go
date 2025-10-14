package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type commandHandlerFunc func(context.Context, *Agent, protocol.Command) protocol.CommandResult

type commandRouter struct {
	handlers map[string]commandHandlerFunc
}

func newCommandRouter() *commandRouter {
	return &commandRouter{handlers: make(map[string]commandHandlerFunc)}
}

func newDefaultCommandRouter() (*commandRouter, error) {
	router := newCommandRouter()
	builtins := map[string]commandHandlerFunc{
		"ping":            pingCommandHandler,
		"shell":           shellCommandHandler,
		"open-url":        openURLCommandHandler,
		"tool-activation": toolActivationCommandHandler,
	}

	for name, handler := range builtins {
		if err := router.register(name, handler); err != nil {
			return nil, err
		}
	}

	return router, nil
}

func (r *commandRouter) register(name string, handler commandHandlerFunc) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("command name cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler for command %q cannot be nil", trimmed)
	}
	if _, exists := r.handlers[trimmed]; exists {
		return fmt.Errorf("command %q already registered", trimmed)
	}
	r.handlers[trimmed] = handler
	return nil
}

func (r *commandRouter) dispatch(ctx context.Context, agent *Agent, cmd protocol.Command) protocol.CommandResult {
	if r == nil {
		return newFailureResult(cmd.ID, "command router not initialized")
	}

	if handler, ok := r.lookup(cmd.Name); ok {
		return handler(ctx, agent, cmd)
	}

	if trimmed := strings.TrimSpace(cmd.Name); trimmed != cmd.Name {
		if handler, ok := r.lookup(trimmed); ok {
			return handler(ctx, agent, cmd)
		}
	}

	if agent != nil && agent.modules != nil {
		if handled, result := agent.modules.HandleCommand(ctx, cmd); handled {
			return result
		}
	}

	return newFailureResult(cmd.ID, fmt.Sprintf("unsupported command: %s", cmd.Name))
}

func (r *commandRouter) lookup(name string) (commandHandlerFunc, bool) {
	handler, ok := r.handlers[name]
	return handler, ok
}
