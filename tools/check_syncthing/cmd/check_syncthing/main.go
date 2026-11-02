// Command check_syncthing is a Nagios/Gogios plugin that reports
// out-of-sync Syncthing folders through the REST API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/snonux/conf/tools/check_syncthing/internal"
)

func main() {
	var cfg internal.Config
	// ContinueOnError: a bad flag must report UNKNOWN on stdout, not exit 2
	// (CRITICAL) with the usage on stderr as the default flag set does.
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.URL, "url", "http://127.0.0.1:8384", "Syncthing GUI/REST base URL")
	fs.StringVar(&cfg.APIKeyFile, "api-key-file", "", "file holding the API key (preferred over -api-key-env)")
	fs.StringVar(&cfg.APIKeyEnv, "api-key-env", "SYNCTHING_API_KEY", "environment variable holding the API key")
	fs.StringVar(&cfg.IgnoreFile, "ignore-file", "", "file of known out-of-sync folders to ignore")
	fs.StringVar(&cfg.StateDir, "state-dir", "", "writable directory for out-of-sync durations; empty disables escalation")
	fs.DurationVar(&cfg.EscalateAfter, "escalate-after", time.Hour, "escalate a lasting WARNING (folders) or stale transfer (freshness) to CRITICAL after this long (needs -state-dir; 0 disables)")
	fs.DurationVar(&cfg.Timeout, "timeout", 30*time.Second, "timeout for all API calls")
	fs.StringVar(&cfg.Mode, "mode", internal.ModeFolders, "folders: out-of-sync folders; freshness: data received from a peer within -period and -device peers seen")
	fs.DurationVar(&cfg.Period, "period", 24*time.Hour, "freshness: warn when no peer sent data for this long")
	fs.Func("device", "freshness: warn when <device ID or name>=<duration> was not seen for that long (repeatable)", func(s string) error {
		rule, err := internal.ParseDeviceRule(s)
		if err == nil {
			cfg.Devices = append(cfg.Devices, rule)
		}
		return err
	})
	version := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(os.Stdout)
			fs.PrintDefaults()
			return
		}
		fmt.Printf("SYNCTHING UNKNOWN: %v\n", err)
		os.Exit(int(internal.Unknown))
	}
	if *version {
		fmt.Println(internal.Version)
		return
	}
	if fs.NArg() > 0 {
		fmt.Printf("SYNCTHING UNKNOWN: unexpected arguments %q\n", fs.Args())
		os.Exit(int(internal.Unknown))
	}
	result := internal.Run(context.Background(), cfg, time.Now())
	fmt.Println(result.Line)
	os.Exit(int(result.Level))
}
