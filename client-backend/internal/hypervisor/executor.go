package hypervisor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type CommandExecutor interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
