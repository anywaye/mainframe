package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ==================== LOCAL-FIRST SECRETS (age-encrypted, peer-synced) ====================
// Each machine has an age keypair. Secrets are encrypted with all peer public keys.
// Sync happens via SSH. No hub, no single point of failure.

var secretsMode string

func isPersonalMode() bool {
	return secretsMode == "personal"
}

// --- Paths ---

func localSecretsDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "mainframe")
	os.MkdirAll(dir, 0700)
	os.MkdirAll(filepath.Join(dir, "peers"), 0700)
	return dir
}

func ageKeyPath() string  { return filepath.Join(localSecretsDir(), "age-key.txt") }
func agePubPath() string  { return filepath.Join(localSecretsDir(), "age-key.pub") }
func secretsAgePath() string { return filepath.Join(localSecretsDir(), "secrets.age") }
func peersPath() string   { return filepath.Join(localSecretsDir(), "peers.json") }

// --- Peer config ---

type Peer struct {
	Name   string `json:"name"`
	PubKey string `json:"pubkey"`
	SSH    string `json:"ssh"`
}

type PeersConfig struct {
	Peers map[string]Peer `json:"peers"`
}

func loadPeers() PeersConfig {
	var cfg PeersConfig
	cfg.Peers = make(map[string]Peer)
	data, err := os.ReadFile(peersPath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	if cfg.Peers == nil {
		cfg.Peers = make(map[string]Peer)
	}
	return cfg
}

func savePeers(cfg PeersConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(peersPath(), data, 0600)
}

// --- Crypto helpers ---

func getAllRecipients() []string {
	var recipients []string
	if pubData, err := os.ReadFile(agePubPath()); err == nil {
		pubKey := strings.TrimSpace(string(pubData))
		if pubKey != "" {
			recipients = append(recipients, pubKey)
		}
	}
	cfg := loadPeers()
	for _, peer := range cfg.Peers {
		if peer.PubKey != "" {
			recipients = append(recipients, peer.PubKey)
		}
	}
	return recipients
}

func decryptLocalSecrets() (map[string]string, error) {
	secrets := make(map[string]string)
	data, err := os.ReadFile(secretsAgePath())
	if err != nil {
		if os.IsNotExist(err) {
			return secrets, nil
		}
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	args := []string{"-d", "-i", ageKeyPath()}
	cmd := exec.Command("age", args...)
	cmd.Stdin = strings.NewReader(string(data))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	if err := json.Unmarshal(out, &secrets); err != nil {
		return nil, fmt.Errorf("failed to parse secrets: %w", err)
	}
	return secrets, nil
}

func encryptLocalSecrets(secrets map[string]string) error {
	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}

	args := []string{"-o", secretsAgePath()}
	for _, r := range getAllRecipients() {
		args = append(args, "-r", r)
	}

	cmd := exec.Command("age", args...)
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to encrypt: %w", err)
	}
	os.Chmod(secretsAgePath(), 0600)
	return nil
}

func getLocalSecret(name string) (string, error) {
	secrets, err := decryptLocalSecrets()
	if err != nil {
		return "", err
	}
	val, ok := secrets[name]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", name)
	}
	return val, nil
}

// ==================== COMMANDS ====================

// --- init ---

var secretsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize local secrets store (generate age keypair)",
	RunE: func(cmd *cobra.Command, args []string) error {
		keyPath := ageKeyPath()
		pubPath := agePubPath()

		if _, err := os.Stat(keyPath); err == nil {
			fmt.Println("Age keypair already exists.")
			pubData, _ := os.ReadFile(pubPath)
			fmt.Printf("Public key: %s\n", strings.TrimSpace(string(pubData)))
			return nil
		}

		cmd2 := exec.Command("age-keygen", "-o", keyPath)
		out, err := cmd2.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to generate keypair: %w\n%s", err, string(out))
		}

		keyData, _ := os.ReadFile(keyPath)
		for _, line := range strings.Split(string(keyData), "\n") {
			if strings.HasPrefix(line, "# public key:") {
				pubKey := strings.TrimSpace(strings.TrimPrefix(line, "# public key:"))
				os.WriteFile(pubPath, []byte(pubKey+"\n"), 0644)
				break
			}
		}

		pubData, _ := os.ReadFile(pubPath)
		fmt.Println("Age keypair generated.")
		fmt.Printf("Private key: %s\n", keyPath)
		fmt.Printf("Public key: %s\n", strings.TrimSpace(string(pubData)))
		fmt.Println("\nShare your public key with other machines:")
		fmt.Printf("  mf secrets peers add <name> %s\n", strings.TrimSpace(string(pubData)))
		return nil
	},
}

// --- set ---

var secretsSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE",
	Short: "Set a secret in local store",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets, err := decryptLocalSecrets()
		if err != nil {
			return err
		}
		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("Invalid format: %s (expected KEY=VALUE)\n", arg)
				continue
			}
			secrets[parts[0]] = parts[1]
			fmt.Printf("Set: %s\n", parts[0])
		}
		if err := encryptLocalSecrets(secrets); err != nil {
			return err
		}
		fmt.Printf("Secrets saved (%d total)\n", len(secrets))
		return nil
	},
}

// --- get ---

var secretsGetCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Get a secret from local store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		val, err := getLocalSecret(args[0])
		if err != nil {
			return err
		}
		// CLI convention: always end with a newline so redirected output is a
		// well-formed file (e.g. `mf secrets get SSH_KEY_DEV_MF > ~/.ssh/dev-mf`).
		if !strings.HasSuffix(val, "\n") {
			val += "\n"
		}
		fmt.Print(val)
		return nil
	},
}

// --- export-for ---

var secretsExportForCmd = &cobra.Command{
	Use:   "export-for PUBKEY",
	Short: "Stream this machine's secrets re-encrypted for another age recipient",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets, err := decryptLocalSecrets()
		if err != nil {
			return err
		}
		if len(secrets) == 0 {
			return fmt.Errorf("no secrets to export")
		}
		data, err := json.MarshalIndent(secrets, "", "  ")
		if err != nil {
			return err
		}
		enc := exec.Command("age", "-r", args[0], "-a")
		enc.Stdin = strings.NewReader(string(data))
		enc.Stderr = os.Stderr
		out, err := enc.Output()
		if err != nil {
			return fmt.Errorf("failed to encrypt for recipient: %w", err)
		}
		os.Stdout.Write(out)
		return nil
	},
}

// --- merge-from ---

var secretsMergeFromCmd = &cobra.Command{
	Use:   "merge-from FILE",
	Short: "Merge secrets from an age-encrypted file into the local store (existing local values win)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		dec := exec.Command("age", "-d", "-i", ageKeyPath())
		dec.Stdin = strings.NewReader(string(data))
		dec.Stderr = os.Stderr
		out, err := dec.Output()
		if err != nil {
			return fmt.Errorf("cannot decrypt %s (are you a recipient?): %w", args[0], err)
		}
		incoming := map[string]string{}
		if err := json.Unmarshal(out, &incoming); err != nil {
			return fmt.Errorf("decrypted file is not a secrets store: %w", err)
		}
		secrets, err := decryptLocalSecrets()
		if err != nil {
			secrets = map[string]string{}
		}
		added := 0
		for k, v := range incoming {
			if _, exists := secrets[k]; !exists {
				secrets[k] = v
				added++
			}
		}
		if err := encryptLocalSecrets(secrets); err != nil {
			return err
		}
		fmt.Printf("Merged %d new secrets from %s (%d total)\n", added, args[0], len(secrets))
		return nil
	},
}

// --- list ---

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret names in local store",
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets, err := decryptLocalSecrets()
		if err != nil {
			return err
		}
		if len(secrets) == 0 {
			fmt.Println("No secrets stored.")
			return nil
		}
		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Println(k)
		}
		fmt.Printf("\nTotal: %d secrets\n", len(keys))
		return nil
	},
}

// --- delete ---

var secretsDeleteCmd = &cobra.Command{
	Use:   "delete KEY",
	Short: "Delete a secret from local store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets, err := decryptLocalSecrets()
		if err != nil {
			return err
		}
		if _, ok := secrets[args[0]]; !ok {
			return fmt.Errorf("secret not found: %s", args[0])
		}
		delete(secrets, args[0])
		if err := encryptLocalSecrets(secrets); err != nil {
			return err
		}
		fmt.Printf("Deleted: %s\n", args[0])
		return nil
	},
}

// --- sync ---

var secretsSyncFrom string

var secretsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync secrets between machines (push to peers, or pull with --from)",
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		if from != "" {
			return syncFromPeer(from)
		}
		return syncToPeers()
	},
}

func syncFromPeer(peerNameOrAddr string) error {
	cfg := loadPeers()
	// Check if it's a peer name
	peer, ok := cfg.Peers[peerNameOrAddr]
	sshAddr := peerNameOrAddr
	if ok && peer.SSH != "" {
		sshAddr = peer.SSH
	}

	fmt.Printf("Pulling secrets from %s...\n", sshAddr)

	tmp, err := os.CreateTemp("", "mainframe-secrets-*.age")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	// Preferred: ask the peer to re-encrypt its store for THIS machine's key.
	// This works even when this machine has a brand-new keypair (e.g. after a
	// wipe/reinstall) and was never a recipient of the peer's store.
	myPub := ""
	if pubData, err := os.ReadFile(agePubPath()); err == nil {
		myPub = strings.TrimSpace(string(pubData))
	}
	merged := false
	if myPub != "" {
		exportCmd := exec.Command("ssh", sshAddr, "mf secrets export-for "+myPub)
		out, err := exportCmd.Output()
		if err == nil && len(out) > 0 {
			if werr := os.WriteFile(tmpPath, out, 0o600); werr == nil {
				if merr := mergeFrom(tempFileArg(tmpPath)); merr == nil {
					merged = true
				} else {
					fmt.Printf("  export-for succeeded but merge failed: %v\n", merr)
				}
			}
		}
	}

	// Fallback: copy the peer's store and merge if this machine is a recipient.
	if !merged {
		remoteFile := "~/.config/mainframe/secrets.age"
		scpCmd := exec.Command("scp", sshAddr+":"+remoteFile, tmpPath)
		scpCmd.Stderr = os.Stderr
		if err := scpCmd.Run(); err != nil {
			return fmt.Errorf("failed to pull from %s: %w", sshAddr, err)
		}
		if err := mergeFrom(tempFileArg(tmpPath)); err != nil {
			return fmt.Errorf("pulled file but can't merge (ask the peer to upgrade mf so it supports 'export-for', or re-add this machine as a recipient): %w", err)
		}
	}
	return nil
}

// tempFileArg keeps merge-from call sites readable.
func tempFileArg(path string) string { return path }

func mergeFrom(path string) error {
	return secretsMergeFromCmd.RunE(secretsMergeFromCmd, []string{path})
}

func syncToPeers() error {
	cfg := loadPeers()
	if len(cfg.Peers) == 0 {
		fmt.Println("No peers configured.")
		fmt.Println("Add peers with: mf secrets peers add <name> <pubkey> [ssh-addr]")
		return nil
	}

	localFile := secretsAgePath()
	if _, err := os.Stat(localFile); err != nil {
		return fmt.Errorf("no secrets file to sync. Run 'mf secrets set' first")
	}

	for name, peer := range cfg.Peers {
		if peer.SSH == "" {
			fmt.Printf("  Skipping %s (no SSH address)\n", name)
			continue
		}
		fmt.Printf("Pushing to %s (%s)...\n", name, peer.SSH)
		// Copy to a temp path on the peer, then MERGE there — never overwrite
		// the peer's own store (which may hold secrets we don't have).
		tmpRemote := "~/.config/mainframe/.secrets-incoming.age"
		exec.Command("ssh", peer.SSH, "mkdir -p ~/.config/mainframe").Run()
		scpCmd := exec.Command("scp", localFile, peer.SSH+":"+tmpRemote)
		scpCmd.Stderr = os.Stderr
		if err := scpCmd.Run(); err != nil {
			fmt.Printf("  Failed: %v\n", err)
			continue
		}
		mergeCmd := exec.Command("ssh", peer.SSH, "mf secrets merge-from "+tmpRemote+" && rm -f "+tmpRemote)
		mergeCmd.Stderr = os.Stderr
		if err := mergeCmd.Run(); err != nil {
			fmt.Printf("  Failed to merge on peer (upgrade mf there): %v\n", err)
			continue
		}
		fmt.Printf("  Synced to %s\n", name)
	}
	return nil
}

// --- peers ---

var secretsPeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "Manage peer machines for secret sync",
}

var secretsPeersAddCmd = &cobra.Command{
	Use:   "add NAME PUBKEY_OR_FILE [SSH_ADDR]",
	Short: "Add a peer machine",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		pubkeyOrFile := args[1]
		sshAddr := ""
		if len(args) > 2 {
			sshAddr = args[2]
		}

		pubkey := pubkeyOrFile
		if _, err := os.Stat(pubkeyOrFile); err == nil {
			data, _ := os.ReadFile(pubkeyOrFile)
			pubkey = strings.TrimSpace(string(data))
		}

		if !strings.HasPrefix(pubkey, "age1") {
			return fmt.Errorf("invalid age public key (must start with age1)")
		}

		cfg := loadPeers()
		cfg.Peers[name] = Peer{Name: name, PubKey: pubkey, SSH: sshAddr}
		if err := savePeers(cfg); err != nil {
			return err
		}

		fmt.Printf("Added peer: %s\n", name)
		// Re-encrypt secrets with new recipient
		secrets, err := decryptLocalSecrets()
		if err == nil && len(secrets) > 0 {
			if err := encryptLocalSecrets(secrets); err != nil {
				fmt.Printf("  Warning: %v\n", err)
			} else {
				fmt.Println("  Secrets re-encrypted with new peer key")
			}
		}
		return nil
	},
}

var secretsPeersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List peer machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadPeers()
		if len(cfg.Peers) == 0 {
			fmt.Println("No peers configured.")
			return nil
		}
		for name, peer := range cfg.Peers {
			fmt.Printf("  %s: %s", name, peer.PubKey[:20]+"...")
			if peer.SSH != "" {
				fmt.Printf(" (ssh: %s)", peer.SSH)
			}
			fmt.Println()
		}
		return nil
	},
}

var secretsPeersRemoveCmd = &cobra.Command{
	Use:   "remove NAME",
	Short: "Remove a peer machine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadPeers()
		if _, ok := cfg.Peers[args[0]]; !ok {
			return fmt.Errorf("peer not found: %s", args[0])
		}
		delete(cfg.Peers, args[0])
		if err := savePeers(cfg); err != nil {
			return err
		}
		fmt.Printf("Removed peer: %s\n", args[0])

		secrets, err := decryptLocalSecrets()
		if err == nil && len(secrets) > 0 {
			if err := encryptLocalSecrets(secrets); err != nil {
				fmt.Printf("  Warning: %v\n", err)
			} else {
				fmt.Println("  Secrets re-encrypted without removed peer")
			}
		}
		return nil
	},
}

var secretsPubKeyCmd = &cobra.Command{
	Use:   "pubkey",
	Short: "Show this machine's age public key",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(agePubPath())
		if err != nil {
			return fmt.Errorf("no keypair found. Run 'mf secrets init' first")
		}
		fmt.Print(string(data))
		return nil
	},
}

// --- setup Infisical (optional broker) ---

var secretsSetupInfisicalCmd = &cobra.Command{
	Use:   "setup-infisical",
	Short: "Set up Infisical for agent secret brokering (self-hosted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Install Infisical CLI if needed
		if _, err := exec.LookPath("infisical"); err != nil {
			fmt.Println("Installing Infisical CLI...")
			if _, err := exec.LookPath("brew"); err == nil {
				exec.Command("brew", "install", "infisical/get-cli/infisical").Run()
			} else {
				// Linux
				exec.Command("bash", "-c", "curl -1sLf 'https://dl.cloudsmith.io/public/infisical/infisical-cli/setup.deb.sh' | bash").Run()
				exec.Command("apt-get", "install", "-y", "infisical").Run()
			}
		}

		// Create docker-compose for self-hosted Infisical
		home, _ := os.UserHomeDir()
		infisicalDir := filepath.Join(home, "infisical")
		os.MkdirAll(infisicalDir, 0755)

		composePath := filepath.Join(infisicalDir, "docker-compose.yml")
		if _, err := os.Stat(composePath); err != nil {
			encKey := "change-me"
			authSecret := "change-me"
			if out, err := exec.Command("openssl", "rand", "-hex", "16").Output(); err == nil {
				encKey = strings.TrimSpace(string(out))
			}
			if out, err := exec.Command("openssl", "rand", "-hex", "32").Output(); err == nil {
				authSecret = strings.TrimSpace(string(out))
			}

			composeContent := "version: \"3.8\"\nservices:\n  infisical:\n    image: infisical/infisical:latest\n    ports:\n      - \"8080:8080\"\n    environment:\n      - ENCRYPTION_KEY=" + encKey + "\n      - AUTH_SECRET=" + authSecret + "\n      - DB_CONNECTION_URI=postgres://infisical:infisical@db:5432/infisical\n      - REDIS_URL=redis://redis:6379\n    depends_on:\n      - db\n      - redis\n    restart: unless-stopped\n\n  db:\n    image: postgres:15-alpine\n    environment:\n      - POSTGRES_USER=infisical\n      - POSTGRES_PASSWORD=infisical\n      - POSTGRES_DB=infisical\n    volumes:\n      - infisical-db:/var/lib/postgresql/data\n    restart: unless-stopped\n\n  redis:\n    image: redis:7-alpine\n    restart: unless-stopped\n\nvolumes:\n  infisical-db:\n"
			os.WriteFile(composePath, []byte(composeContent), 0644)
			fmt.Printf("Created %s\n", composePath)
		}

		fmt.Println("Infisical setup complete.")
		fmt.Println("  Start: cd ~/infisical && docker compose up -d")
		fmt.Println("  Then: mf secrets push-infisical (to sync local secrets)")
		fmt.Println("  UI: http://localhost:8080")
		return nil
	},
}

// --- push to Infisical (optional broker) ---

var secretsPushInfisicalCmd = &cobra.Command{
	Use:   "push-infisical",
	Short: "Sync local secrets to local Infisical instance (for agent brokering)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if infisical is available
		if _, err := exec.LookPath("infisical"); err != nil {
			fmt.Println("Infisical not installed. Skipping.")
			fmt.Println("Install: brew install infisical/get-cli/infisical")
			return nil
		}

		secrets, err := decryptLocalSecrets()
		if err != nil {
			return err
		}

		host := "http://localhost:8080"
		projectID := os.Getenv("INFISICAL_PROJECT_ID")
		if projectID == "" {
			fmt.Println("INFISICAL_PROJECT_ID not set. Skipping.")
			return nil
		}

		for name, value := range secrets {
			setCmd := exec.Command("infisical", "secrets", "set", name+"="+value,
				"--path=/", "--env=dev", "--projectId="+projectID, "--domain="+host)
			setCmd.Stdout = os.Stdout
			setCmd.Stderr = os.Stderr
			if err := setCmd.Run(); err != nil {
				fmt.Printf("  Failed to push %s: %v\n", name, err)
			} else {
				fmt.Printf("  Pushed: %s\n", name)
			}
		}
		fmt.Printf("Pushed %d secrets to Infisical\n", len(secrets))
		return nil
	},
}

// --- personal mode (Bitwarden) ---

var secretsPersonalMode bool

func isPersonal() bool {
	return secretsPersonalMode
}

func personalSetup() error {
	if _, err := exec.LookPath("bw"); err != nil {
		fmt.Println("Bitwarden CLI not found. Installing...")
		if _, err := exec.LookPath("npm"); err == nil {
			exec.Command("npm", "install", "-g", "@bitwarden/cli").Run()
		} else {
			return fmt.Errorf("bw CLI not found. Install: npm install -g @bitwarden/cli")
		}
	}
	version, _ := exec.Command("bw", "--version").Output()
	fmt.Printf("Bitwarden CLI: %s\n", strings.TrimSpace(string(version)))
	return nil
}

func personalStatus() error {
	if _, err := exec.LookPath("bw"); err != nil {
		fmt.Println("Bitwarden CLI: not installed")
		return nil
	}
	version, _ := exec.Command("bw", "--version").Output()
	fmt.Printf("Bitwarden CLI: %s\n", strings.TrimSpace(string(version)))
	out, _ := exec.Command("bw", "status").Output()
	fmt.Printf("Status: %s\n", strings.TrimSpace(string(out)))
	return nil
}

func personalUnlock(password string) error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("bw CLI not found")
	}
	statusOut, _ := exec.Command("bw", "status").Output()
	if strings.Contains(string(statusOut), "unauthenticated") {
		fmt.Println("Not logged in. Run 'bw login <email>' first.")
		return nil
	}
	unlockArgs := []string{"unlock", "--raw"}
	if password != "" {
		unlockArgs = append(unlockArgs, password)
	}
	unlockCmd := exec.Command("bw", unlockArgs...)
	if password == "" {
		unlockCmd.Stdin = os.Stdin
	}
	unlockCmd.Stderr = os.Stderr
	out, err := unlockCmd.Output()
	if err != nil {
		return fmt.Errorf("unlock failed: %w", err)
	}
	session := strings.TrimSpace(string(out))
	if session == "" {
		return fmt.Errorf("no session token received")
	}
	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".config", "bitwarden")
	os.MkdirAll(sessionDir, 0755)
	os.WriteFile(filepath.Join(sessionDir, "session"), []byte(session+"\n"), 0600)
	os.Setenv("BW_SESSION", session)
	fmt.Println("Vault unlocked.")
	return nil
}

func personalLock() error {
	exec.Command("bw", "lock").Run()
	home, _ := os.UserHomeDir()
	os.Remove(filepath.Join(home, ".config", "bitwarden", "session"))
	fmt.Println("Vault locked.")
	return nil
}

func personalGet(name string) error {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", "bitwarden", "session"))
	if err != nil {
		return fmt.Errorf("vault not unlocked. Run: mf secrets --mode=personal unlock")
	}
	os.Setenv("BW_SESSION", strings.TrimSpace(string(data)))
	out, err := exec.Command("bw", "get", "item", name).Output()
	if err != nil {
		return fmt.Errorf("item not found: %s", name)
	}
	fmt.Println(string(out))
	return nil
}

func personalList() error {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", "bitwarden", "session"))
	if err != nil {
		return fmt.Errorf("vault not unlocked. Run: mf secrets --mode=personal unlock")
	}
	os.Setenv("BW_SESSION", strings.TrimSpace(string(data)))
	out, err := exec.Command("bw", "list", "items").Output()
	if err != nil {
		return fmt.Errorf("failed to list: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// ==================== COMMAND REGISTRATION ====================

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets (local-first, peer-synced, age-encrypted)",
	Long: `Manage secrets using local-first storage (age-encrypted, peer-synced).

Machine secrets are stored locally and synced between machines via SSH.
No hub, no single point of failure. Each machine is standalone.

Commands:
  init          Generate age keypair for this machine
  set           Set a secret (KEY=VALUE)
  get           Get a secret
  list          List secret names
  delete        Delete a secret
  sync          Push secrets to peers (--from to pull)
  peers         Manage peer machines
  pubkey        Show this machine's public key
  push-infisical Sync local secrets to Infisical (for agent brokering)

Personal mode (--mode=personal): Bitwarden for human secrets (web logins, etc.)
`,
}

func init() {
	secretsCmd.PersistentFlags().StringVar(&secretsMode, "mode", "machine", "Secrets mode: machine (local-first) or personal (Bitwarden)")

	// Sync flag
	secretsSyncCmd.Flags().StringVar(&secretsSyncFrom, "from", "", "Pull secrets from a peer (SSH address or peer name)")

	// Peers
	secretsPeersCmd.AddCommand(secretsPeersAddCmd)
	secretsPeersCmd.AddCommand(secretsPeersListCmd)
	secretsPeersCmd.AddCommand(secretsPeersRemoveCmd)

	// Personal mode commands (Bitwarden)
	personalSetupCmd := &cobra.Command{
		Use: "setup", Short: "Set up Bitwarden (personal mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isPersonal() {
				fmt.Println("Setup is for personal mode. Use: mf secrets --mode=personal setup")
				return nil
			}
			return personalSetup()
		},
	}
	personalStatusCmd := &cobra.Command{
		Use: "status", Short: "Show status (personal: Bitwarden)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isPersonal() {
				return personalStatus()
			}
			// Machine mode: show local store status
			secrets, err := decryptLocalSecrets()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			fmt.Printf("Local secrets store: %s\n", secretsAgePath())
			fmt.Printf("Secrets: %d\n", len(secrets))
			cfg := loadPeers()
			fmt.Printf("Peers: %d\n", len(cfg.Peers))
			// Check age keypair
			if _, err := os.Stat(ageKeyPath()); err == nil {
				fmt.Println("Age keypair: exists")
				pubData, _ := os.ReadFile(agePubPath())
				fmt.Printf("Public key: %s\n", strings.TrimSpace(string(pubData)))
			} else {
				fmt.Println("Age keypair: not found (run 'mf secrets init')")
			}
			return nil
		},
	}
	personalUnlockCmd := &cobra.Command{
		Use: "unlock", Short: "Unlock Bitwarden vault (personal mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isPersonal() {
				fmt.Println("Unlock is for personal mode. Use: mf secrets --mode=personal unlock")
				return nil
			}
			password := ""
			if len(args) > 0 {
				password = args[0]
			}
			return personalUnlock(password)
		},
	}
	personalLockCmd := &cobra.Command{
		Use: "lock", Short: "Lock Bitwarden vault (personal mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isPersonal() {
				fmt.Println("Lock is for personal mode. Use: mf secrets --mode=personal lock")
				return nil
			}
			return personalLock()
		},
	}
	personalGetCmd := &cobra.Command{
		Use: "get", Short: "Get secret (machine) or item (personal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isPersonal() {
				if len(args) == 0 {
					return fmt.Errorf("usage: mf secrets --mode=personal get <item>")
				}
				return personalGet(args[0])
			}
			if len(args) == 0 {
				return fmt.Errorf("usage: mf secrets get <KEY>")
			}
			val, err := getLocalSecret(args[0])
			if err != nil {
				return err
			}
			fmt.Print(val)
			return nil
		},
	}
	personalListCmd := &cobra.Command{
		Use: "list", Short: "List secrets (machine) or items (personal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isPersonal() {
				return personalList()
			}
			secrets, err := decryptLocalSecrets()
			if err != nil {
				return err
			}
			if len(secrets) == 0 {
				fmt.Println("No secrets stored.")
				return nil
			}
			keys := make([]string, 0, len(secrets))
			for k := range secrets {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Println(k)
			}
			return nil
		},
	}

	secretsCmd.AddCommand(secretsInitCmd)
	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsSyncCmd)
	secretsCmd.AddCommand(secretsExportForCmd)
	secretsCmd.AddCommand(secretsMergeFromCmd)
	secretsCmd.AddCommand(secretsPeersCmd)
	secretsCmd.AddCommand(secretsPubKeyCmd)
	secretsCmd.AddCommand(secretsSetupInfisicalCmd)
	secretsCmd.AddCommand(secretsPushInfisicalCmd)
	secretsCmd.AddCommand(personalSetupCmd)
	secretsCmd.AddCommand(personalStatusCmd)
	secretsCmd.AddCommand(personalUnlockCmd)
	secretsCmd.AddCommand(personalLockCmd)
	secretsCmd.AddCommand(personalGetCmd)
	secretsCmd.AddCommand(personalListCmd)
	secretsCmd.AddCommand(secretsDeleteCmd)

	// Override get/list with the combined versions
	// Remove the old standalone get/list and use personalGetCmd/personalListCmd
	// (already added above)

	rootCmd.AddCommand(secretsCmd)
}
