package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/wk-j/yaam/internal/config"
)

// yaam serve runs as a launchd agent started at login (ADR 0009).
const launchdLabel = "com.github.wk-j.yaam"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func launchdDomain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

func xmlText(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func launchctl(args ...string) error {
	out, err := exec.Command("/bin/launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %v: %s", args[0], err, bytes.TrimSpace(out))
	}
	return nil
}

func loaded() bool {
	return exec.Command("/bin/launchctl", "print", launchdDomain()+"/"+launchdLabel).Run() == nil
}

func cmdInstall(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: yaam install")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	p, err := homePaths()
	if err != nil {
		return err
	}
	if err := config.EnsureHome(p.home); err != nil {
		return err
	}
	env := ""
	if os.Getenv("YAAM_HOME") != "" {
		env = fmt.Sprintf(`
	<key>EnvironmentVariables</key>
	<dict>
		<key>YAAM_HOME</key>
		<string>%s</string>
	</dict>`, xmlText(p.home))
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>serve</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>%s
</dict>
</plist>
`, launchdLabel, xmlText(exe), xmlText(p.serveLog), xmlText(p.serveLog), env)

	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return err
	}
	if loaded() {
		if err := launchctl("bootout", launchdDomain()+"/"+launchdLabel); err != nil {
			return err
		}
		// bootout returns while the old yaam serve is still shutting down,
		// and bootstrap fails with "5: Input/output error" until it is gone.
		if err := waitUnloaded(25 * time.Second); err != nil {
			return err
		}
	}
	if err := launchctl("bootstrap", launchdDomain(), path); err != nil {
		return err
	}
	fmt.Printf("yaam serve now runs at login (%s); open the web UI with `yaam open`\n", path)
	return nil
}

// waitUnloaded waits until launchd no longer knows the service; launchd
// SIGKILLs a job 20 s after SIGTERM, so timeout must exceed that.
func waitUnloaded(timeout time.Duration) error {
	for deadline := time.Now().Add(timeout); loaded(); time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			return errors.New("the running yaam serve has not stopped yet; run yaam install again in a moment")
		}
	}
	return nil
}

func cmdUninstall(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: yaam uninstall")
	}
	path, err := plistPath()
	if err != nil {
		return err
	}
	if loaded() {
		if err := launchctl("bootout", launchdDomain()+"/"+launchdLabel); err != nil {
			return err
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Println("yaam serve no longer runs at login")
	return nil
}
