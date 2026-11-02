package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds the plugin options (see cmd/check_syncthing for the flags).
type Config struct {
	URL        string
	APIKeyFile string
	APIKeyEnv  string
	IgnoreFile string
	// StateDir enables duration tracking and escalation when not empty.
	StateDir      string
	EscalateAfter time.Duration
	Timeout       time.Duration
}

// Result is the plugin outcome: one summary line and its exit level.
type Result struct {
	Level Level
	Line  string
}

// Run performs one check. A missing or rejected API key is CRITICAL, so a
// broken key pages at once instead of waiting for a renotify; other API,
// configuration and decode failures are UNKNOWN. The state is only touched
// once the API answered completely.
func Run(ctx context.Context, cfg Config, now time.Time) Result {
	key, err := readAPIKey(cfg)
	if err != nil {
		return critical(err)
	}
	rules, err := LoadIgnoreFile(cfg.IgnoreFile)
	if err != nil {
		return unknown(err)
	}
	if cfg.Timeout <= 0 {
		return unknown(fmt.Errorf("timeout must be positive, got %s", cfg.Timeout))
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	// Never follow redirects: Go forwards custom headers such as X-API-Key
	// to the redirect target, even on another host or over plain http.
	httpClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	entries, err := fetchEntries(ctx, NewClient(cfg.URL, key, httpClient))
	if errors.Is(err, ErrKeyRejected) {
		return critical(err)
	}
	if err != nil {
		return unknown(err)
	}
	report := Classify(entries, rules)
	if cfg.StateDir == "" {
		return Result{Level: report.Level(), Line: report.Summary()}
	}
	report, stateErr := applyState(report, cfg, now)
	result := Result{Level: report.Level(), Line: report.Summary()}
	switch {
	case stateErr == nil:
	case result.Level == Critical:
		// Keep a CRITICAL that is already known.
		result.Line += "; " + stateErr.Error()
	default:
		// Escalation cannot work without state: never let that pass as OK
		// or WARNING.
		result = Result{Level: Unknown, Line: "SYNCTHING UNKNOWN: " + stateErr.Error() +
			"; was " + strings.TrimPrefix(result.Line, "SYNCTHING ")}
	}
	return result
}

func fetchEntries(ctx context.Context, client Client) ([]Entry, error) {
	folders, err := client.Folders(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(folders))
	for _, folder := range folders {
		entry := Entry{Folder: folder}
		if !folder.Paused {
			if entry.Status, err = client.FolderStatus(ctx, folder.ID); err != nil {
				return nil, fmt.Errorf("folder %s: %w", folder.ID, err)
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// applyState updates the first-seen times in cfg.StateDir and applies
// durations and escalation. On a state error the report is returned
// without history.
func applyState(report Report, cfg Config, now time.Time) (Report, error) {
	st, err := LoadState(cfg.StateDir)
	if err != nil {
		return report, fmt.Errorf("state error: %w", err)
	}
	st = st.Track(report.OutOfSyncIDs(), now)
	if err := st.Save(cfg.StateDir); err != nil {
		return report, fmt.Errorf("state error: %w", err)
	}
	return report.ApplyHistory(st, now, cfg.EscalateAfter), nil
}

// readAPIKey reads the key from the key file, else from the environment
// variable. Errors name the source, never the value.
func readAPIKey(cfg Config) (string, error) {
	if cfg.APIKeyFile != "" {
		data, err := os.ReadFile(cfg.APIKeyFile)
		if err != nil {
			return "", fmt.Errorf("read API key file: %w", err)
		}
		if key := strings.TrimSpace(string(data)); key != "" {
			return key, nil
		}
		return "", fmt.Errorf("API key file %s is empty", cfg.APIKeyFile)
	}
	if key := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv)); key != "" {
		return key, nil
	}
	return "", fmt.Errorf("no API key: pass -api-key-file or set $%s", cfg.APIKeyEnv)
}

func unknown(err error) Result {
	return Result{Level: Unknown, Line: "SYNCTHING UNKNOWN: " + err.Error()}
}

func critical(err error) Result {
	return Result{Level: Critical, Line: "SYNCTHING CRITICAL: " + err.Error()}
}
