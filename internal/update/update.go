package update

import (
	"context"
	"fmt"
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
		Source:    source,
		Filters:  []string{"revise"},
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

	newer := latest.GreaterThan(currentVersion)

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latest.Version(),
		IsNewer:        newer,
		IsPreRelease:   latest.Prerelease,
		ReleaseNotes:   latest.ReleaseNotes,
	}, nil
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
		Source:  source,
		Filters: []string{"revise"},
	})
	if err != nil {
		return fmt.Errorf("creating updater: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = updater.UpdateTo(ctx, detectedRelease, "")
	if err != nil {
		return fmt.Errorf("applying update: %w", err)
	}

	return nil
}
