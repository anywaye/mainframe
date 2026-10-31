package cmd

import (
	"fmt"
	"os"
	"time"
)

// logFile is the path to the provision log file
var logFile *os.File

// initLogging opens a log file for the current run
func initLogging() {
	logPath := "/tmp/mainframe-provision.log"
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Warning: could not open log file: %v\n", err)
		return
	}
	logFile = f
	logMsg("=== Mainframe provision started ===")
}

// logMsg writes a timestamped message to both stdout and the log file
func logMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s", ts, msg)
	fmt.Println(line)
	if logFile != nil {
		fmt.Fprintln(logFile, line)
	}
}

// logStep writes a step header with timing
func logStep(step, total int, name string) {
	logMsg("[%d/%d] %s", step, total, name)
}

// logOK writes a success message with duration
func logOK(elapsed string) {
	logMsg("  OK (%s)", elapsed)
}

// logFail writes a failure message with duration and hint
func logFail(elapsed string, err error, hint string) {
	logMsg("  FAILED (%s): %v", elapsed, err)
	if hint != "" {
		logMsg("  Hint: %s", hint)
	}
}

// closeLogging closes the log file
func closeLogging() {
	if logFile != nil {
		logMsg("=== Mainframe provision finished ===")
		logFile.Close()
	}
}
