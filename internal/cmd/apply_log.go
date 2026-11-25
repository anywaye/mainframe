package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// applyLogger handles logging, timing, and summary for mf apply.
type applyLogger struct {
	verbose   int
	logFile   *os.File
	startTime time.Time
	stepTimes map[string]time.Duration
	summary   []string
}

var appLogger *applyLogger

// initApplyLogger sets up logging to file and returns the logger.
func initApplyLogger(verbose int) *applyLogger {
	l := &applyLogger{
		verbose:   verbose,
		startTime: time.Now(),
		stepTimes: make(map[string]time.Duration),
		summary:   []string{},
	}

	// Create log directory
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".mainframe", "logs")
	os.MkdirAll(logDir, 0755)

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	logPath := filepath.Join(logDir, fmt.Sprintf("apply-%s.log", timestamp))
	f, err := os.Create(logPath)
	if err != nil {
		fmt.Printf("Warning: could not create log file: %v\n", err)
	} else {
		l.logFile = f
		fmt.Printf("Logging to: %s\n", logPath)
	}

	return l
}

// log writes a message to both stdout and the log file.
func (l *applyLogger) log(msg string) {
	fmt.Print(msg)
	if l.logFile != nil {
		l.logFile.WriteString(msg)
	}
}

// logLine writes a message with newline to both stdout and log file.
func (l *applyLogger) logLine(msg string) {
	l.log(msg + "\n")
}

// verboseLog writes a message only if verbose level is high enough.
func (l *applyLogger) verboseLog(level int, msg string) {
	if l.verbose >= level {
		l.logLine(msg)
	}
}

// startStep records the start time of a step.
func (l *applyLogger) startStep(name string) time.Time {
	t := time.Now()
	l.verboseLog(2, fmt.Sprintf("  [timing] %s started at %s", name, t.Format("15:04:05")))
	return t
}

// endStep records the duration of a step and adds to summary.
func (l *applyLogger) endStep(name string, start time.Time, status string) {
	duration := time.Since(start)
	l.stepTimes[name] = duration
	l.summary = append(l.summary, fmt.Sprintf("  %-20s %8s  %s", name, duration.Round(time.Millisecond), status))
	l.verboseLog(1, fmt.Sprintf("  [timing] %s completed in %s", name, duration.Round(time.Millisecond)))
}

// printSummary prints a summary of all steps and their durations.
func (l *applyLogger) printSummary() {
	totalDuration := time.Since(l.startTime)
	fmt.Println("\n=== Apply Summary ===")
	fmt.Printf("Total time: %s\n", totalDuration.Round(time.Millisecond))
	fmt.Println("Step durations:")
	for _, s := range l.summary {
		fmt.Println(s)
	}
	if l.logFile != nil {
		l.logFile.WriteString(fmt.Sprintf("\n=== Apply Summary ===\nTotal time: %s\n", totalDuration.Round(time.Millisecond)))
		for _, s := range l.summary {
			l.logFile.WriteString(s + "\n")
		}
		l.logFile.Close()
	}
}

// close closes the log file.
func (l *applyLogger) close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

// applyState tracks which steps have been completed for resume capability.
type applyState struct {
	CompletedSteps []string  `json:"completed_steps"`
	LastRun        time.Time `json:"last_run"`
	ConfigDir      string    `json:"config_dir"`
}

// stateFilePath returns the path to the state file.
func stateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mainframe", "apply.state")
}

// loadApplyState loads the previous apply state.
func loadApplyState() *applyState {
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return nil
	}
	var state applyState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

// saveApplyState saves the current apply state.
func saveApplyState(state *applyState) {
	data, _ := json.MarshalIndent(state, "", "  ")
	os.MkdirAll(filepath.Dir(stateFilePath()), 0755)
	os.WriteFile(stateFilePath(), data, 0644)
}

// clearApplyState removes the state file.
func clearApplyState() {
	os.Remove(stateFilePath())
}
