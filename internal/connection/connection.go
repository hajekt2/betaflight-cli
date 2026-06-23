package connection

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"time"

	"go.bug.st/serial"

	"github.com/hajekt2/betaflight-cli/internal/support"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type OperationClass int

const (
	ReadOnly OperationClass = iota
	Write
	Dangerous
)

type Config struct {
	Port             string
	Baud             int
	Timeout          time.Duration
	AutoPort         bool
	AllowUnsupported bool
}

type PortInfo struct {
	Name      string `json:"name"`
	Candidate bool   `json:"candidate"`
	Reason    string `json:"reason,omitempty"`
	WillProbe bool   `json:"will_probe,omitempty"`
}

type TargetInfo struct {
	Port            string `json:"port,omitempty"`
	AutoDetected    bool   `json:"auto_detected,omitempty"`
	SelectionReason string `json:"selection_reason,omitempty"`
	Variant         string `json:"variant,omitempty"`
	FirmwareVersion string `json:"firmware_version,omitempty"`
	MSPAPIVersion   string `json:"msp_api_version,omitempty"`
	MSPProtocol     uint8  `json:"msp_protocol_version"`
}

type APIVersion struct {
	MSPProtocol uint8
	Major       uint8
	Minor       uint8
}

type Client struct {
	port    Port
	timeout time.Duration
	target  TargetInfo
}

type Port interface {
	io.Reader
	io.Writer
	ResetInputBuffer() error
	ResetOutputBuffer() error
	SetReadTimeout(time.Duration) error
	Close() error
}

type CodedError struct {
	Code       string
	Message    string
	Candidates []TargetInfo
}

func (e *CodedError) Error() string {
	return e.Message
}

func ListPorts() ([]PortInfo, error) {
	names, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	ports := make([]PortInfo, 0, len(names))
	for _, name := range names {
		candidate, reason := classifyPort(name)
		ports = append(ports, PortInfo{
			Name:      name,
			Candidate: candidate,
			Reason:    reason,
		})
	}
	return ports, nil
}

func Connect(ctx context.Context, cfg Config, op OperationClass) (*Client, TargetInfo, error) {
	if cfg.Baud == 0 {
		cfg.Baud = 115200
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.Port != "" {
		client, err := open(cfg.Port, cfg.Baud, cfg.Timeout)
		if err != nil {
			return nil, TargetInfo{}, &CodedError{Code: "transport_error", Message: err.Error()}
		}
		target, err := client.Handshake(ctx)
		if err != nil {
			client.Close()
			return nil, TargetInfo{}, err
		}
		target.Port = cfg.Port
		client.target = target
		if err := checkSupported(target, cfg.AllowUnsupported); err != nil {
			client.Close()
			return nil, target, err
		}
		return client, target, nil
	}
	if op != ReadOnly && !cfg.AutoPort {
		return nil, TargetInfo{}, &CodedError{
			Code:    "auto_port_required",
			Message: "writes require explicit --port or explicit --auto-port",
		}
	}
	client, target, err := autoDetect(ctx, cfg)
	if err != nil {
		return nil, TargetInfo{}, err
	}
	if err := checkSupported(target, cfg.AllowUnsupported); err != nil {
		client.Close()
		return nil, target, err
	}
	return client, target, nil
}

func checkSupported(target TargetInfo, allow bool) error {
	if target.MSPAPIVersion == "" {
		return &CodedError{Code: "unsupported_firmware", Message: "missing MSP API version"}
	}
	if allow {
		return nil
	}
	if !isSupportedFirmwareVersion(target.FirmwareVersion) {
		return &CodedError{
			Code:    "unsupported_firmware",
			Message: fmt.Sprintf("firmware %q is outside the supported metadata set; pass --allow-unsupported to continue", target.FirmwareVersion),
		}
	}
	return nil
}

func isSupportedFirmwareVersion(version string) bool {
	return support.IsSupportedFirmwareVersion(version)
}

func autoDetect(ctx context.Context, cfg Config) (*Client, TargetInfo, error) {
	ports, err := ListPorts()
	if err != nil {
		return nil, TargetInfo{}, &CodedError{Code: "transport_error", Message: err.Error()}
	}
	var responders []TargetInfo
	var selected *Client
	for _, p := range ports {
		if !p.Candidate {
			continue
		}
		client, err := open(p.Name, cfg.Baud, cfg.Timeout)
		if err != nil {
			continue
		}
		target, err := client.Handshake(ctx)
		if err != nil {
			client.Close()
			continue
		}
		target.Port = p.Name
		target.AutoDetected = true
		target.SelectionReason = "single Betaflight-compatible MSP responder"
		responders = append(responders, target)
		if selected != nil {
			selected.Close()
		}
		selected = client
		client.target = target
	}
	if len(responders) == 0 {
		return nil, TargetInfo{}, &CodedError{
			Code:    "no_target",
			Message: "no Betaflight-compatible USB serial port responded to MSP_API_VERSION",
		}
	}
	if len(responders) > 1 {
		if selected != nil {
			selected.Close()
		}
		return nil, TargetInfo{}, &CodedError{
			Code:       "multiple_targets",
			Message:    "multiple Betaflight-compatible USB serial ports responded; pass --port explicitly",
			Candidates: responders,
		}
	}
	return selected, responders[0], nil
}

func classifyPort(name string) (bool, string) {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "bluetooth") || strings.Contains(lower, "debug-console") {
		return false, "ignored non-USB or debug port"
	}
	if strings.Contains(lower, "usb") {
		return true, "USB serial candidate"
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(lower, "/dev/cu.") {
			return false, "macOS incoming tty device; prefer /dev/cu.* for USB serial"
		}
		if strings.Contains(lower, "/dev/cu.usb") || strings.Contains(lower, "usbmodem") || strings.Contains(lower, "usbserial") {
			return true, "macOS USB serial candidate"
		}
	case "linux":
		if strings.Contains(lower, "ttyacm") || strings.Contains(lower, "ttyusb") {
			return true, "Linux USB serial candidate"
		}
	case "windows":
		if strings.HasPrefix(strings.ToUpper(name), "COM") {
			return true, "Windows COM port candidate"
		}
	}
	return false, "not a typical Betaflight USB serial port"
}

func open(name string, baud int, timeout time.Duration) (*Client, error) {
	port, err := serial.Open(name, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, err
	}
	return NewClient(port, timeout)
}

func NewClient(port Port, timeout time.Duration) (*Client, error) {
	client := &Client{port: port, timeout: timeout}
	if err := port.SetReadTimeout(timeout); err != nil {
		port.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Close() error {
	if c == nil || c.port == nil {
		return nil
	}
	return c.port.Close()
}

func (c *Client) Target() TargetInfo {
	return c.target
}

func (c *Client) Request(ctx context.Context, code uint16, payload []byte) (msp.Frame, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return msp.Frame{}, mapContextError(err)
	}
	_ = c.port.ResetInputBuffer()
	_ = c.port.ResetOutputBuffer()
	if err := c.setReadTimeout(ctx); err != nil {
		return msp.Frame{}, err
	}
	if _, err := c.port.Write(msp.EncodeRequest(code, payload)); err != nil {
		return msp.Frame{}, &CodedError{Code: "transport_error", Message: err.Error()}
	}
	for {
		if err := ctx.Err(); err != nil {
			return msp.Frame{}, mapContextError(err)
		}
		if err := c.setReadTimeout(ctx); err != nil {
			return msp.Frame{}, err
		}
		frame, err := msp.ReadFrame(c.port)
		if err != nil {
			if err == io.ErrNoProgress && ctx.Err() != nil {
				return msp.Frame{}, mapContextError(ctx.Err())
			}
			code := "transport_error"
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrNoProgress) {
				code = "msp_timeout"
			}
			return msp.Frame{}, &CodedError{Code: code, Message: err.Error()}
		}
		if frame.Code != code {
			continue
		}
		if frame.Unsupported {
			return msp.Frame{}, &CodedError{Code: "unsupported_msp", Message: fmt.Sprintf("MSP command %d is unsupported by target", code)}
		}
		return frame, nil
	}
}

func (c *Client) setReadTimeout(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return mapContextError(ctx.Err())
		}
		timeout := c.timeout
		if remaining < timeout {
			timeout = remaining
		}
		if timeout <= 0 {
			timeout = time.Millisecond
		}
		if err := c.port.SetReadTimeout(timeout); err != nil {
			return &CodedError{Code: "transport_error", Message: err.Error()}
		}
		return nil
	}
	if err := c.port.SetReadTimeout(c.timeout); err != nil {
		return &CodedError{Code: "transport_error", Message: err.Error()}
	}
	return nil
}

func mapContextError(err error) *CodedError {
	switch err {
	case context.Canceled:
		return &CodedError{Code: "context_canceled", Message: err.Error()}
	default:
		if errors.Is(err, context.DeadlineExceeded) {
			return &CodedError{Code: "context_deadline_exceeded", Message: err.Error()}
		}
		return &CodedError{Code: "context_canceled", Message: err.Error()}
	}
}

func (c *Client) Handshake(ctx context.Context) (TargetInfo, error) {
	api, err := c.APIVersion(ctx)
	if err != nil {
		return TargetInfo{}, err
	}
	if api.Major != 1 {
		return TargetInfo{}, &CodedError{
			Code:    "unsupported_msp_api",
			Message: fmt.Sprintf("unsupported MSP API major version %d", api.Major),
		}
	}
	variant, _ := c.FCVariant(ctx)
	firmware, _ := c.FCVersion(ctx)
	return TargetInfo{
		Variant:         variant,
		FirmwareVersion: firmware,
		MSPAPIVersion:   fmt.Sprintf("%d.%d", api.Major, api.Minor),
		MSPProtocol:     api.MSPProtocol,
	}, nil
}

func (c *Client) APIVersion(ctx context.Context) (APIVersion, error) {
	frame, err := c.Request(ctx, msp.MSPAPIVersion, nil)
	if err != nil {
		return APIVersion{}, err
	}
	r := msp.NewPayloadReader(frame.Payload)
	proto, err := r.U8()
	if err != nil {
		return APIVersion{}, &CodedError{Code: "payload_decode_error", Message: err.Error()}
	}
	major, err := r.U8()
	if err != nil {
		return APIVersion{}, &CodedError{Code: "payload_decode_error", Message: err.Error()}
	}
	minor, err := r.U8()
	if err != nil {
		return APIVersion{}, &CodedError{Code: "payload_decode_error", Message: err.Error()}
	}
	return APIVersion{MSPProtocol: proto, Major: major, Minor: minor}, nil
}

func (c *Client) FCVariant(ctx context.Context) (string, error) {
	frame, err := c.Request(ctx, msp.MSPFCVariant, nil)
	if err != nil {
		return "", err
	}
	if len(frame.Payload) < 4 {
		return "", &CodedError{Code: "payload_decode_error", Message: "MSP_FC_VARIANT returned fewer than 4 bytes"}
	}
	return string(frame.Payload[:4]), nil
}

func (c *Client) FCVersion(ctx context.Context) (string, error) {
	frame, err := c.Request(ctx, msp.MSPFCVersion, nil)
	if err != nil {
		return "", err
	}
	if len(frame.Payload) < 3 {
		return "", &CodedError{Code: "payload_decode_error", Message: "MSP_FC_VERSION returned fewer than 3 bytes"}
	}
	major := frame.Payload[0]
	minor := frame.Payload[1]
	patch := frame.Payload[2]
	r := msp.NewPayloadReader(frame.Payload[3:])
	if s, err := r.PString(); err == nil && s != "" {
		return s, nil
	}
	if major >= 20 {
		return fmt.Sprintf("20%d.%d.%d", major, minor, patch), nil
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

func (c *Client) ExecCLI(ctx context.Context, command string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, mapContextError(err)
	}
	_ = c.port.ResetInputBuffer()
	_ = c.port.ResetOutputBuffer()
	if err := c.setReadTimeout(ctx); err != nil {
		return nil, err
	}
	wire := append([]byte{0x02}, []byte(command)...)
	wire = append(wire, '\n', 0x03)
	if _, err := c.port.Write(wire); err != nil {
		return nil, &CodedError{Code: "transport_error", Message: err.Error()}
	}
	if err := c.readUntil(ctx, 0x02); err != nil {
		var coded *CodedError
		if errors.As(err, &coded) && (coded.Code == "context_canceled" || coded.Code == "context_deadline_exceeded") {
			return nil, coded
		}
		return nil, &CodedError{Code: "cli_timeout", Message: err.Error()}
	}
	var lines []string
	var line []byte
	for {
		b, err := c.readByteWithContext(ctx)
		if err != nil {
			var coded *CodedError
			if errors.As(err, &coded) {
				return nil, coded
			}
			return nil, &CodedError{Code: "cli_timeout", Message: err.Error()}
		}
		switch b {
		case 0x03:
			if len(line) > 0 {
				lines = append(lines, string(line))
			}
			return lines, nil
		case '\r':
			continue
		case '\n':
			lines = append(lines, string(line))
			line = line[:0]
		default:
			line = append(line, b)
		}
	}
}

func (c *Client) EnterInteractive() error {
	_, err := c.port.Write([]byte{'#'})
	if err != nil {
		return &CodedError{Code: "transport_error", Message: err.Error()}
	}
	return nil
}

func (c *Client) RunInteractive(stdin io.Reader, stdout io.Writer) error {
	if err := c.EnterInteractive(); err != nil {
		return err
	}
	_ = c.port.SetReadTimeout(0)
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(c.port, stdin)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(stdout, c.port)
		errCh <- err
	}()
	return <-errCh
}

func (c *Client) readUntil(ctx context.Context, want byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var deadline time.Time
	contextDeadline := false
	if d, ok := ctx.Deadline(); ok {
		deadline = d
		contextDeadline = true
	} else {
		deadline = time.Now().Add(c.timeout)
	}
	for time.Now().Before(deadline) {
		if err := c.setReadTimeout(ctx); err != nil {
			return err
		}
		b, err := c.readByteWithContext(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return err
			}
			if errors.Is(err, io.ErrNoProgress) {
				if ctx.Err() != nil {
					return mapContextError(ctx.Err())
				}
				continue
			}
			var coded *CodedError
			if errors.As(err, &coded) {
				return coded
			}
			return err
		}
		if b == want {
			return nil
		}
	}
	if ctx.Err() != nil {
		return mapContextError(ctx.Err())
	}
	if contextDeadline {
		return &CodedError{Code: "context_deadline_exceeded", Message: context.DeadlineExceeded.Error()}
	}
	return fmt.Errorf("timed out waiting for byte 0x%02x", want)
}

func (c *Client) readByte() (byte, error) {
	var b [1]byte
	n, err := c.port.Read(b[:])
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.ErrNoProgress
	}
	return b[0], err
}

func (c *Client) readByteWithContext(ctx context.Context) (byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, mapContextError(err)
	}
	if err := c.setReadTimeout(ctx); err != nil {
		return 0, err
	}
	b, err := c.readByte()
	if err != nil {
		return 0, err
	}
	return b, nil
}
