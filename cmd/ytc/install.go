package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	hostName    = "com.mogest.ytc"
	extensionID = "bidbhfllonlbjabgdemkdibgdillegmd"
)

var chromeDirs = map[string]string{
	"darwin": "Library/Application Support/Google/Chrome",
	"linux":  ".config/google-chrome",
}

func install() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	chromeDir, ok := chromeDirs[runtime.GOOS]
	if !ok {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	chromeDir = filepath.Join(home, chromeDir)
	if _, err := os.Stat(chromeDir); err != nil {
		return fmt.Errorf("Chrome not found: %w", err)
	}

	manifest, _ := json.MarshalIndent(map[string]any{
		"name":            hostName,
		"description":     "ytchauffeur",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": []string{"chrome-extension://" + extensionID + "/"},
	}, "", "  ")

	dir := filepath.Join(chromeDir, "NativeMessagingHosts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, hostName+".json")
	if err := os.WriteFile(path, manifest, 0o644); err != nil {
		return err
	}
	fmt.Println("installed", path)
	return nil
}
