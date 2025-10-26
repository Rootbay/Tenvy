//go:build linux

package startup

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type fakeCronRunner struct {
	mu       sync.Mutex
	commands []string
	inputs   []string
	outputs  map[string][]byte
	errors   map[string]error
}

func newFakeCronRunner() *fakeCronRunner {
	return &fakeCronRunner{outputs: make(map[string][]byte), errors: make(map[string]error)}
}

func (f *fakeCronRunner) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	f.mu.Lock()
	f.commands = append(f.commands, cmd)
	output := f.outputs[cmd]
	err := f.errors[cmd]
	f.mu.Unlock()
	return output, err
}

func (f *fakeCronRunner) runWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	cmd := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	f.mu.Lock()
	f.commands = append(f.commands, cmd)
	f.inputs = append(f.inputs, input)
	err := f.errors[cmd]
	f.mu.Unlock()
	return nil, err
}

func (f *fakeCronRunner) set(command, output string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outputs[command] = []byte(output)
	if err != nil {
		f.errors[command] = err
	} else {
		delete(f.errors, command)
	}
}

func (f *fakeCronRunner) lastInput() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.inputs) == 0 {
		return ""
	}
	return f.inputs[len(f.inputs)-1]
}

func TestStartupLinuxListParsesCron(t *testing.T) {
	runner := newFakeCronRunner()
	cron := `# tenvy-managed id=alpha name=Alpha scope=user
@reboot /usr/bin/alpha --flag
# Comment ignored
@reboot /usr/bin/beta`
	runner.set("crontab -l", cron, nil)

	provider := newTestProviderWithRunner(runner)
	inv, err := provider.List(context.Background(), ListRequest{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(inv.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(inv.Entries))
	}
	if inv.Entries[0].ID == inv.Entries[1].ID {
		t.Fatalf("expected distinct ids")
	}
	managed := inv.Entries[0]
	if managed.ID != "alpha" {
		managed = inv.Entries[1]
	}
	if managed.Name != "Alpha" || managed.Scope != ScopeUser || !managed.Enabled {
		t.Fatalf("unexpected managed entry: %+v", managed)
	}
}

func TestStartupLinuxToggleDisablesEntry(t *testing.T) {
	runner := newFakeCronRunner()
	cron := `# tenvy-managed id=alpha name=Alpha scope=user
@reboot /usr/bin/alpha --flag`
	runner.set("crontab -l", cron, nil)

	provider := newTestProviderWithRunner(runner)
	entry, err := provider.Toggle(context.Background(), ToggleRequest{EntryID: "alpha", Enabled: false})
	if err != nil {
		t.Fatalf("toggle failed: %v", err)
	}
	if entry.Enabled {
		t.Fatalf("expected entry disabled")
	}
	applied := runner.lastInput()
	if !strings.Contains(applied, "#@reboot /usr/bin/alpha --flag") {
		t.Fatalf("expected cron to comment command, got:\n%s", applied)
	}
}

func TestStartupLinuxCreateAppendsEntry(t *testing.T) {
	runner := newFakeCronRunner()
	runner.set("crontab -l", "", fmt.Errorf("no crontab for user"))

	provider := newTestProviderWithRunner(runner)
	entry, err := provider.Create(context.Background(), CreateRequest{Definition: EntryDefinition{
		Name:      "Gamma",
		Path:      "/usr/bin/gamma",
		Arguments: "--verbose",
		Scope:     ScopeUser,
		Enabled:   true,
	}})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if entry.Name != "Gamma" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	applied := runner.lastInput()
	if !strings.Contains(applied, "@reboot /usr/bin/gamma --verbose") {
		t.Fatalf("expected cron entry appended, got:\n%s", applied)
	}
}

func TestStartupLinuxRemoveDeletesEntry(t *testing.T) {
	runner := newFakeCronRunner()
	cron := `# tenvy-managed id=alpha name=Alpha scope=user
@reboot /usr/bin/alpha --flag
# tenvy-managed id=beta name=Beta scope=user
@reboot /usr/bin/beta`
	runner.set("crontab -l", cron, nil)
	provider := newTestProviderWithRunner(runner)
	if _, err := provider.Remove(context.Background(), RemoveRequest{EntryID: "alpha"}); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	applied := runner.lastInput()
	if strings.Contains(applied, "alpha") {
		t.Fatalf("expected alpha entry removed, got:\n%s", applied)
	}
}
