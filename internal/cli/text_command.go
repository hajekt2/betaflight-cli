package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) textCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "text", Short: "Read and update Betaflight text metadata"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read pilot, craft, profile, build, and release text over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				text, err := bfcommands.ReadTextStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"text": text,
				})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set FIELD VALUE",
		Short: "Set writable pilot, craft, or profile text over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			field, ok := bfcommands.TextFieldByKey(args[0])
			if !ok {
				return validationFailureMessage(a, cmd, fmt.Sprintf("field must be one of: %s", writableTextFieldKeys()))
			}
			request := bfcommands.TextSetRequest{TextField: field, Value: args[1]}
			if _, err := bfcommands.EncodeTextSet(request); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "text set changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetText(cmd.Context(), client, request)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"text": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "text_set",
					Command: "MSP2_SET_TEXT",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-json FILE",
		Short: "Set writable pilot, craft, or profile text from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			request, err := parseTextSetJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if _, err := bfcommands.EncodeTextSet(request); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "text set changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetText(cmd.Context(), client, request)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"text": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "text_set",
					Command: "MSP2_SET_TEXT",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	})
	return cmd
}

func writableTextFieldKeys() string {
	fields := bfcommands.WritableTextFields()
	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.Key)
	}
	return strings.Join(keys, ", ")
}

func parseTextSetJSON(data []byte) (bfcommands.TextSetRequest, error) {
	var raw struct {
		Field   *string                    `json:"field"`
		Key     *string                    `json:"key"`
		Value   *string                    `json:"value"`
		Text    *bfcommands.TextSetRequest `json:"text"`
		Set     *bfcommands.TextSetRequest `json:"set"`
		Request *bfcommands.TextSetRequest `json:"request"`
		Result  *struct {
			Request *bfcommands.TextSetRequest `json:"request"`
		} `json:"text_set"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return bfcommands.TextSetRequest{}, err
	}
	switch {
	case raw.Text != nil:
		return completeTextSetRequest(*raw.Text)
	case raw.Set != nil:
		return completeTextSetRequest(*raw.Set)
	case raw.Request != nil:
		return completeTextSetRequest(*raw.Request)
	case raw.Result != nil && raw.Result.Request != nil:
		return completeTextSetRequest(*raw.Result.Request)
	case raw.Value != nil && raw.Field != nil:
		return textSetRequestFromKey(*raw.Field, *raw.Value)
	case raw.Value != nil && raw.Key != nil:
		return textSetRequestFromKey(*raw.Key, *raw.Value)
	}
	var request bfcommands.TextSetRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return bfcommands.TextSetRequest{}, err
	}
	return completeTextSetRequest(request)
}

func completeTextSetRequest(request bfcommands.TextSetRequest) (bfcommands.TextSetRequest, error) {
	if request.Key == "" {
		return bfcommands.TextSetRequest{}, fmt.Errorf("text field key is required")
	}
	field, ok := bfcommands.TextFieldByKey(request.Key)
	if !ok {
		return bfcommands.TextSetRequest{}, fmt.Errorf("field must be one of: %s", writableTextFieldKeys())
	}
	request.TextField = field
	return request, nil
}

func textSetRequestFromKey(key string, value string) (bfcommands.TextSetRequest, error) {
	request := bfcommands.TextSetRequest{TextField: bfcommands.TextField{Key: key}, Value: value}
	return completeTextSetRequest(request)
}
