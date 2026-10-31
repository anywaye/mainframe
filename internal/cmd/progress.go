package cmd

import (
	"encoding/json"
	"fmt"
	"time"
)

// ProgressEvent represents a single progress update
type ProgressEvent struct {
	Type      string `json:"type"`                // step_start, step_done, step_failed, complete, error, info
	Step      int    `json:"step,omitempty"`      // current step number
	Total     int    `json:"total,omitempty"`     // total steps
	Name      string `json:"name,omitempty"`      // step name
	Status    string `json:"status,omitempty"`    // ok, failed, skipped
	Progress  int    `json:"progress,omitempty"`  // 0-100 percentage
	ElapsedMs int64  `json:"elapsed_ms,omitempty"` // milliseconds
	Error     string `json:"error,omitempty"`     // error message
	Message   string `json:"message,omitempty"`   // human-readable message
	Timestamp string `json:"ts"`                 // ISO 8601 timestamp
}

// jsonMode controls whether output is NDJSON
var jsonMode bool

// emitProgress outputs a progress event in JSON or human-readable format
func emitProgress(e ProgressEvent) {
	e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	if jsonMode {
		data, _ := json.Marshal(e)
		fmt.Println(string(data))
		if logFile != nil {
			fmt.Fprintln(logFile, string(data))
		}
	} else {
		// Human-readable
		switch e.Type {
		case "step_start":
			fmt.Printf("[%d/%d] %s...\n", e.Step, e.Total, e.Name)
		case "step_done":
			fmt.Printf("  OK (%dms)\n", e.ElapsedMs)
		case "step_failed":
			fmt.Printf("  FAILED: %s\n", e.Error)
		case "step_skipped":
			fmt.Printf("  (skipped)\n")
		case "info":
			fmt.Printf("  %s\n", e.Message)
		case "complete":
			fmt.Printf("\n=== Complete: %s (%dms) ===\n", e.Status, e.ElapsedMs)
		case "error":
			fmt.Printf("ERROR: %s\n", e.Error)
		}
	}
}
