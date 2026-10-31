package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var hardenCmd = &cobra.Command{
	Use:   "harden",
	Short: "Apply security hardening (sane defaults)",
	Long:  "Apply mainframe's security hardening with sane defaults. Optional extras via env vars.",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")

		hardenPath := findAsset("lib/harden.sh")
		if hardenPath == "" {
			return fmt.Errorf("harden.sh not found")
		}

		c := exec.Command("bash", hardenPath)
		c.Env = os.Environ()

		if jsonMode || jsonFlag {
			// JSON mode: wrap each line as a progress event
			stdout, _ := c.StdoutPipe()
			stderr, _ := c.StderrPipe()
			if err := c.Start(); err != nil {
				return err
			}
			go func() {
				scanner := bufio.NewScanner(stderr)
				for scanner.Scan() {
					fmt.Fprintf(os.Stderr, "%s\n", scanner.Text())
				}
			}()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				event := map[string]interface{}{
					"type": "log",
					"ts":   time.Now().UTC().Format(time.RFC3339),
					"msg":  line,
				}
				data, _ := json.Marshal(event)
				fmt.Println(string(data))
			}
			return c.Wait()
		}

		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	hardenCmd.Flags().BoolVar(&jsonMode, "json", false, "Output as NDJSON (machine-parseable)")
}
