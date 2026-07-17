package cli

import (
	"context"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func executeCLIPlan(ctx context.Context, client *connection.Client, lines []string) (map[string][]string, []string, string, error) {
	responses := make(map[string][]string, len(lines))
	applied := make([]string, 0, len(lines))
	for _, line := range lines {
		responseLines, err := client.ExecCLI(ctx, line)
		if err != nil {
			return responses, applied, line, err
		}
		responses[line] = responseLines
		applied = append(applied, line)
	}
	return responses, applied, "", nil
}

func addCLIApplySideEffects(env *output.Envelope, lines []string) {
	for _, line := range lines {
		env.SideEffects = append(env.SideEffects, output.SideEffect{
			Type:    "cli_command",
			Command: line,
			Detail:  "configuration change applied but not saved",
		})
	}
}

func refreshChangePlan(data map[string]any) {
	delete(data, "change_plan")
	data["change_plan"] = withChangePlanRoot(data)["change_plan"]
}
