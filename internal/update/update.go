package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creativeprojects/go-selfupdate"
)

// UpdateInfo describes an available update.
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	IsNewer        bool
	IsPreRelease   bool
	ReleaseNotes   string
}

// release holds the detected release so ApplyUpdate can use it.
var detectedRelease *selfupdate.Release

// CheckForUpdate queries GitHub releases for a newer version.
// Returns nil info if already up to date.
func CheckForUpdate(currentVersion string, includePre bool) (*UpdateInfo, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("creating source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:     source,
		Prerelease: includePre,
	})
	if err != nil {
		return nil, fmt.Errorf("creating updater: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latest, found, err := updater.DetectLatest(ctx, selfupdate.NewRepositorySlug("justincampbell", "revise"))
	if err != nil {
		return nil, fmt.Errorf("checking for update: %w", err)
	}
	if !found {
		return nil, nil
	}

	// Store for ApplyUpdate.
	detectedRelease = latest

	// Dev builds and other non-semver versions always report as needing update.
	newer := !IsSemver(currentVersion) || latest.GreaterThan(currentVersion)

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latest.Version(),
		IsNewer:        newer,
		IsPreRelease:   latest.Prerelease,
		ReleaseNotes:   latest.ReleaseNotes,
	}, nil
}

// IsSemver returns true if the version string is valid semver (with or without v prefix).
func IsSemver(v string) bool {
	s := v
	if !strings.HasPrefix(s, "v") {
		s = "v" + s
	}
	// Quick check: must have at least one dot and start with a digit after v.
	if len(s) < 4 || s[1] < '0' || s[1] > '9' || !strings.Contains(s, ".") {
		return false
	}
	return true
}

// ApplyUpdate downloads and installs the previously detected update.
// CheckForUpdate must be called first.
func ApplyUpdate() error {
	if detectedRelease == nil {
		return fmt.Errorf("no update detected; call CheckForUpdate first")
	}

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return fmt.Errorf("creating source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
	if err != nil {
		return fmt.Errorf("creating updater: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}

	// Resolve symlinks so we update the actual binary, not the symlink
	// (e.g. Homebrew installs are symlinked from /opt/homebrew/bin/).
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	err = updater.UpdateTo(ctx, detectedRelease, exe)
	if err != nil {
		return fmt.Errorf("applying update: %w", err)
	}

	return nil
}
