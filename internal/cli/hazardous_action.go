package cli

type hazardousReadOnlyEvidence struct {
	Source   string `json:"source"`
	ReadOnly bool   `json:"read_only"`
}

type hazardousCaptureSummary struct {
	Source             string `json:"source"`
	ReadOnly           bool   `json:"read_only"`
	PreflightCaptured  bool   `json:"preflight_captured"`
	PostActionCaptured bool   `json:"post_stop_captured"`
}

type hazardousActionAudit struct {
	Source              string   `json:"source"`
	ReadOnlyEvidence    bool     `json:"read_only_evidence"`
	RequestedDurationMS int64    `json:"requested_duration_ms"`
	ElapsedDurationMS   int64    `json:"elapsed_duration_ms"`
	StartCommand        string   `json:"start_command"`
	StopCommand         string   `json:"stop_command"`
	Confirmations       []string `json:"confirmations"`
	SafetyPassed        bool     `json:"safety_passed"`
	PreflightCaptured   bool     `json:"preflight_captured"`
	PostActionCaptured  bool     `json:"post_stop_captured"`
	StopAttempted       bool     `json:"stop_attempted"`
	StopSucceeded       bool     `json:"stop_succeeded"`
	WarningMessages     []string `json:"warning_messages,omitempty"`
}
