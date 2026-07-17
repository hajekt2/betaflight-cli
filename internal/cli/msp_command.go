package cli

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
	"github.com/spf13/cobra"
)

func (a *app) mspCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "msp", Short: "Raw MSP diagnostics"}
	var payloadHex string
	var decodePayload bool
	var directionFilter, sourceFilter, protocolFilter, nameFilter string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List known MSP commands from the compiled registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			direction := msp.CommandDirection(strings.ToLower(strings.TrimSpace(directionFilter)))
			if direction != "" && direction != msp.DirectionRead && direction != msp.DirectionWrite && direction != msp.DirectionBoth && direction != msp.DirectionUnknown {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "invalid --direction; use read, write, both, or unknown"))
			}
			protocolFilter = strings.TrimSpace(protocolFilter)
			protocolValue := 0
			if protocolFilter != "" {
				p, err := strconv.Atoi(protocolFilter)
				if err != nil {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "invalid --protocol; expected integer"))
				}
				if p < 0 || p > 255 {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "invalid --protocol; expected 0-255"))
				}
				protocolValue = p
			}
			commands := msp.ListCommands()
			var filtered []map[string]any
			for _, command := range commands {
				if nameFilter != "" && !strings.Contains(strings.ToLower(command.Name), strings.ToLower(nameFilter)) {
					continue
				}
				if direction != "" && command.Direction != direction {
					continue
				}
				if protocolFilter != "" && command.Protocol != uint8(protocolValue) {
					continue
				}
				if sourceFilter != "" && !strings.Contains(strings.ToLower(command.Source), strings.ToLower(sourceFilter)) {
					continue
				}
				filtered = append(filtered, map[string]any{
					"name":      command.Name,
					"code":      command.Code,
					"protocol":  command.Protocol,
					"direction": command.Direction,
					"source":    command.Source,
					"line":      command.Line,
				})
			}
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"msp": map[string]any{
					"registry_version": msp.GeneratedMSPSourceVersion,
					"count":            len(filtered),
					"commands":         filtered,
				},
			}))
		},
	}
	listCmd.Flags().StringVar(&directionFilter, "direction", "", "filter by command direction: read, write, both, unknown")
	listCmd.Flags().StringVar(&protocolFilter, "protocol", "", "filter by MSP protocol version")
	listCmd.Flags().StringVar(&sourceFilter, "source", "", "filter by source file substring")
	listCmd.Flags().StringVar(&nameFilter, "name", "", "filter by command name substring")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "metadata [CODE]",
		Short: "Show compiled MSP registry metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				commands := msp.ListCommands()
				directions := map[string]int{}
				protocols := map[string]int{}
				sources := map[string]int{}
				for _, command := range commands {
					directions[string(command.Direction)]++
					protocols[strconv.Itoa(int(command.Protocol))]++
					if command.Source != "" {
						sources[command.Source]++
					}
				}
				return a.render(output.Success(commandPath(cmd), nil, map[string]any{
					"msp": map[string]any{
						"registry_version": msp.GeneratedMSPSourceVersion,
						"count":            len(commands),
						"directions":       directions,
						"protocols":        protocols,
						"sources":          sources,
					},
				}))
			}
			code, meta, err := parseMSPCode(args[0])
			if err != nil {
				return err
			}
			env := output.Success(commandPath(cmd), nil, map[string]any{
				"msp": map[string]any{
					"registry_version": msp.GeneratedMSPSourceVersion,
					"code":             code,
					"code_name":        meta.Name,
					"protocol":         meta.Protocol,
					"direction":        meta.Direction,
					"source":           meta.Source,
					"line":             meta.Line,
				},
			})
			if meta.Source == "" {
				env.Warnings = append(env.Warnings, output.Warning{Code: "unknown_command", Message: "command has no known metadata and may be unsupported"})
			}
			return a.render(env)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "batch CODE...",
		Short: "Send an MSP_MULTIPLE_MSP read-only batch request",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requests, payload, err := buildMSPBatchRequest(args)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				frame, err := client.Request(cmd.Context(), msp.MSPMultipleMsp, payload)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				responses, warnings := parseMSPBatchResponses(requests, frame.Payload, decodePayload)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"msp": map[string]any{
						"code":           frame.Code,
						"code_name":      "MSP_MULTIPLE_MSP",
						"protocol":       frame.Version,
						"version":        frame.Version,
						"direction_hint": string(msp.DirectionRead),
						"command_source": "src/main/msp/msp_protocol.h",
						"request_count":  len(requests),
						"response_count": len(responses),
						"truncated":      len(responses) < len(requests),
						"responses":      responses,
					},
				})
				env.Warnings = append(env.Warnings, warnings...)
				return env
			})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "request CODE",
		Short: "Send a raw MSP request and return raw payload hex",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, meta, err := parseMSPCode(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			if blockedRawMSPCode(code) {
				return a.render(output.Failure(commandPath(cmd), nil, "dangerous_action_blocked", "raw motor, DShot, and receiver override MSP commands are blocked; use a bounded domain-specific workflow"))
			}
			payload, err := hex.DecodeString(strings.TrimPrefix(payloadHex, "0x"))
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			op := connection.ReadOnly
			unknownCommand := meta.Source == ""
			if unknownCommand || msp.IsLikelyWriteCode(code) {
				if !a.opts.yes {
					message := "raw MSP write-like command requires --yes"
					if unknownCommand {
						message = "raw MSP command without compiled metadata requires --yes"
					}
					return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", message))
				}
				op = connection.Dangerous
			}
			return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
				frame, err := client.Request(cmd.Context(), code, payload)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				mspData := map[string]any{
					"code":           frame.Code,
					"code_name":      meta.Name,
					"protocol":       frame.Version,
					"version":        frame.Version,
					"direction_hint": string(meta.Direction),
					"command_source": meta.Source,
					"command_line":   meta.Line,
					"payload_hex":    hex.EncodeToString(frame.Payload),
					"length":         len(frame.Payload),
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"msp": mspData,
				})
				if op != connection.ReadOnly {
					detail := "raw MSP write-like command sent"
					if unknownCommand {
						detail = "raw MSP command without compiled metadata sent"
					}
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "raw_msp", Detail: detail})
				}
				if decodePayload {
					decoded, supported, decodeErr := decodeMSPPayload(code, frame.Payload)
					if decodeErr != nil {
						env.Warnings = append(env.Warnings, output.Warning{
							Code:    "decode_failed",
							Message: decodeErr.Error(),
						})
					}
					if supported {
						mspData["decoded"] = decoded
						mspData["decode_supported"] = true
						mspData["decode_requested"] = true
					} else {
						mspData["decode_supported"] = false
						mspData["decode_requested"] = true
					}
				}
				return env
			})
		},
	})
	cmd.PersistentFlags().StringVar(&payloadHex, "payload-hex", "", "hex payload bytes")
	cmd.PersistentFlags().BoolVar(&decodePayload, "decode", false, "decode known payloads into structured JSON")
	return cmd
}

func blockedRawMSPCode(code uint16) bool {
	switch code {
	case msp.MSPSetRawRC, msp.MSPSetMotor, msp.MSP2SendDshotCommand:
		return true
	default:
		return false
	}
}

type mspBatchRequest struct {
	Code      uint16 `json:"code"`
	CodeName  string `json:"code_name"`
	Protocol  uint8  `json:"protocol"`
	Direction string `json:"direction"`
	Source    string `json:"source"`
	Line      int    `json:"line"`
}

func buildMSPBatchRequest(args []string) ([]mspBatchRequest, []byte, error) {
	requests := make([]mspBatchRequest, 0, len(args))
	payload := make([]byte, 0, len(args))
	for _, arg := range args {
		code, meta, err := parseMSPCode(arg)
		if err != nil {
			return nil, nil, err
		}
		if code > 0xff {
			return nil, nil, fmt.Errorf("%s uses MSP v%d code 0x%x; MSP_MULTIPLE_MSP supports only one-byte MSP v1 codes", meta.Name, meta.Protocol, code)
		}
		if meta.Source == "" {
			return nil, nil, fmt.Errorf("%s has no compiled metadata; use msp request --yes for raw diagnostics", arg)
		}
		if meta.Protocol != 1 {
			return nil, nil, fmt.Errorf("%s uses MSP protocol v%d; MSP_MULTIPLE_MSP supports MSP v1 command codes", meta.Name, meta.Protocol)
		}
		if meta.Direction != msp.DirectionRead {
			return nil, nil, fmt.Errorf("%s is %s; MSP batch accepts generated read-only commands only", meta.Name, meta.Direction)
		}
		if code == msp.MSPMultipleMsp {
			return nil, nil, errors.New("MSP_MULTIPLE_MSP cannot be nested in an MSP batch request")
		}
		requests = append(requests, mspBatchRequest{
			Code:      code,
			CodeName:  meta.Name,
			Protocol:  meta.Protocol,
			Direction: string(meta.Direction),
			Source:    meta.Source,
			Line:      meta.Line,
		})
		payload = append(payload, byte(code))
	}
	return requests, payload, nil
}

func parseMSPBatchResponses(requests []mspBatchRequest, payload []byte, decode bool) ([]map[string]any, []output.Warning) {
	responses := make([]map[string]any, 0, len(requests))
	var warnings []output.Warning
	offset := 0
	for i, request := range requests {
		if offset >= len(payload) {
			warnings = append(warnings, output.Warning{Code: "batch_response_missing", Message: fmt.Sprintf("missing MSP batch response for %s", request.CodeName)})
			break
		}
		length := int(payload[offset])
		offset++
		if offset+length > len(payload) {
			warnings = append(warnings, output.Warning{Code: "batch_response_truncated", Message: fmt.Sprintf("truncated MSP batch response for %s", request.CodeName)})
			length = len(payload) - offset
		}
		responsePayload := payload[offset : offset+length]
		offset += length
		entry := map[string]any{
			"index":          i,
			"code":           request.Code,
			"code_name":      request.CodeName,
			"protocol":       request.Protocol,
			"direction_hint": request.Direction,
			"command_source": request.Source,
			"command_line":   request.Line,
			"payload_hex":    hex.EncodeToString(responsePayload),
			"length":         len(responsePayload),
		}
		if decode {
			decoded, supported, decodeErr := decodeMSPPayload(request.Code, responsePayload)
			if decodeErr != nil {
				warnings = append(warnings, output.Warning{Code: "decode_failed", Message: fmt.Sprintf("%s: %s", request.CodeName, decodeErr.Error())})
			}
			entry["decode_requested"] = true
			entry["decode_supported"] = supported
			if supported {
				entry["decoded"] = decoded
			}
		}
		responses = append(responses, entry)
	}
	if offset < len(payload) {
		warnings = append(warnings, output.Warning{Code: "batch_extra_bytes", Message: fmt.Sprintf("%d trailing bytes after MSP batch responses", len(payload)-offset)})
	}
	return responses, warnings
}

func parseMSPCode(raw string) (uint16, msp.CommandMeta, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, msp.CommandMeta{}, errors.New("MSP code is required")
	}
	if command, ok := msp.LookupCommandByName(trimmed); ok {
		return command.Code, command, nil
	}

	upper := strings.ToUpper(trimmed)
	if command, ok := msp.LookupCommandByName(upper); ok {
		return command.Code, command, nil
	}
	if !strings.HasPrefix(upper, "MSP_") {
		if command, ok := msp.LookupCommandByName("MSP_" + upper); ok {
			return command.Code, command, nil
		}
	}

	code64, err := strconv.ParseUint(trimmed, 0, 16)
	if err != nil {
		return 0, msp.CommandMeta{}, fmt.Errorf("invalid MSP code %q", trimmed)
	}
	code := uint16(code64)
	command, ok := msp.LookupCommand(code)
	if ok {
		return code, command, nil
	}
	return code, msp.CommandMeta{Name: fmt.Sprintf("MSP_%d", code)}, nil
}
