package updater

import (
	"context"
	"errors"
	"regexp"
	"testing"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

type fakeRelease struct {
	version   string
	greaterFn func(string) bool
}

func (r fakeRelease) Version() string {
	return r.version
}

func (r fakeRelease) GreaterThan(other string) bool {
	if r.greaterFn != nil {
		return r.greaterFn(other)
	}
	return false
}

type fakeClient struct {
	release        releaseView
	found          bool
	detectErr      error
	updateErr      error
	updateCalls    int
	lastUpdatePath string
}

func (c *fakeClient) DetectLatest(context.Context, selfupdate.Repository) (releaseView, bool, error) {
	return c.release, c.found, c.detectErr
}

func (c *fakeClient) UpdateTo(_ context.Context, rel releaseView, cmdPath string) error {
	_ = rel
	c.updateCalls++
	c.lastUpdatePath = cmdPath
	return c.updateErr
}

func TestResolveAssetTarget(t *testing.T) {
	tests := []struct {
		name         string
		goos         string
		goarch       string
		wantOS       string
		wantArch     string
		wantExt      string
		wantAsset    string
		expectErrMsg string
	}{
		{
			name:      "linux amd64",
			goos:      "linux",
			goarch:    "amd64",
			wantOS:    "Linux",
			wantArch:  "x86_64",
			wantExt:   "tar.gz",
			wantAsset: "neocode_Linux_x86_64.tar.gz",
		},
		{
			name:      "darwin arm64",
			goos:      "darwin",
			goarch:    "arm64",
			wantOS:    "Darwin",
			wantArch:  "arm64",
			wantExt:   "tar.gz",
			wantAsset: "neocode_Darwin_arm64.tar.gz",
		},
		{
			name:      "windows amd64",
			goos:      "windows",
			goarch:    "amd64",
			wantOS:    "Windows",
			wantArch:  "x86_64",
			wantExt:   "zip",
			wantAsset: "neocode_Windows_x86_64.zip",
		},
		{
			name:         "unsupported os",
			goos:         "freebsd",
			goarch:       "amd64",
			expectErrMsg: "unsupported os",
		},
		{
			name:         "unsupported arch",
			goos:         "linux",
			goarch:       "386",
			expectErrMsg: "unsupported arch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := resolveAssetTarget(tt.goos, tt.goarch)
			if tt.expectErrMsg != "" {
				if err == nil || !regexp.MustCompile(tt.expectErrMsg).MatchString(err.Error()) {
					t.Fatalf("resolveAssetTarget() error = %v, want contains %q", err, tt.expectErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveAssetTarget() error = %v", err)
			}
			if target.OSToken != tt.wantOS {
				t.Fatalf("OSToken = %q, want %q", target.OSToken, tt.wantOS)
			}
			if target.ArchToken != tt.wantArch {
				t.Fatalf("ArchToken = %q, want %q", target.ArchToken, tt.wantArch)
			}
			if target.Ext != tt.wantExt {
				t.Fatalf("Ext = %q, want %q", target.Ext, tt.wantExt)
			}
			if target.AssetName != tt.wantAsset {
				t.Fatalf("AssetName = %q, want %q", target.AssetName, tt.wantAsset)
			}
		})
	}
}

func TestBuildSelfupdateConfigUsesExactFilterAndChecksum(t *testing.T) {
	target := assetTarget{
		OSToken:   "Darwin",
		ArchToken: "x86_64",
		Ext:       "tar.gz",
		AssetName: "neocode_Darwin_x86_64.tar.gz",
	}
	config := buildSelfupdateConfig(target, true)
	if config.OS != "Darwin" || config.Arch != "x86_64" {
		t.Fatalf("OS/Arch = %q/%q, want %q/%q", config.OS, config.Arch, "Darwin", "x86_64")
	}
	if !config.Prerelease {
		t.Fatal("expected prerelease to be enabled")
	}
	if len(config.Filters) != 1 {
		t.Fatalf("len(Filters) = %d, want 1", len(config.Filters))
	}
	exactFilter := config.Filters[0]
	re := regexp.MustCompile(exactFilter)
	if !re.MatchString("neocode_Darwin_x86_64.tar.gz") {
		t.Fatal("exact filter should match target asset")
	}
	if re.MatchString("neocode_Darwin_x86_64.tar.gz.sig") {
		t.Fatal("exact filter should not match similar asset names")
	}
	validator, ok := config.Validator.(*selfupdate.ChecksumValidator)
	if !ok {
		t.Fatalf("validator type = %T, want *selfupdate.ChecksumValidator", config.Validator)
	}
	if validator.UniqueFilename != checksumFilename {
		t.Fatalf("UniqueFilename = %q, want %q", validator.UniqueFilename, checksumFilename)
	}
}

func TestCheckLatest(t *testing.T) {
	originalNewClient := newClient
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "linux"
	runtimeGOARCH = "amd64"

	client := &fakeClient{
		release: fakeRelease{
			version: "v1.2.0",
			greaterFn: func(other string) bool {
				return other == "v1.1.0"
			},
		},
		found: true,
	}
	newClient = func(config selfupdate.Config) (updateClient, error) {
		return client, nil
	}

	result, err := CheckLatest(context.Background(), CheckOptions{
		CurrentVersion:    "v1.1.0",
		IncludePrerelease: false,
	})
	if err != nil {
		t.Fatalf("CheckLatest() error = %v", err)
	}
	if !result.HasUpdate {
		t.Fatal("expected HasUpdate to be true")
	}
	if result.LatestVersion != "v1.2.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v1.2.0")
	}
}

func TestDoUpdateSkipsWhenAlreadyLatestForSemver(t *testing.T) {
	originalNewClient := newClient
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "linux"
	runtimeGOARCH = "amd64"

	client := &fakeClient{
		release: fakeRelease{
			version: "v1.2.0",
			greaterFn: func(other string) bool {
				return false
			},
		},
		found: true,
	}
	newClient = func(config selfupdate.Config) (updateClient, error) {
		return client, nil
	}

	result, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.2.0"})
	if err != nil {
		t.Fatalf("DoUpdate() error = %v", err)
	}
	if result.Updated {
		t.Fatal("expected Updated to be false")
	}
	if client.updateCalls != 0 {
		t.Fatalf("update calls = %d, want 0", client.updateCalls)
	}
}

func TestDoUpdateUsesUpdaterLibraryPathForWindows(t *testing.T) {
	originalNewClient := newClient
	originalExePath := resolveExecutablePath
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		resolveExecutablePath = originalExePath
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "windows"
	runtimeGOARCH = "amd64"

	client := &fakeClient{
		release: fakeRelease{
			version: "v1.3.0",
			greaterFn: func(other string) bool {
				return false
			},
		},
		found: true,
	}

	var capturedConfig selfupdate.Config
	newClient = func(config selfupdate.Config) (updateClient, error) {
		capturedConfig = config
		return client, nil
	}
	resolveExecutablePath = func() (string, error) {
		return `C:\Tools\neocode.exe`, nil
	}

	result, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "dev"})
	if err != nil {
		t.Fatalf("DoUpdate() error = %v", err)
	}
	if !result.Updated {
		t.Fatal("expected Updated to be true")
	}
	if client.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", client.updateCalls)
	}
	if client.lastUpdatePath != `C:\Tools\neocode.exe` {
		t.Fatalf("last update path = %q, want %q", client.lastUpdatePath, `C:\Tools\neocode.exe`)
	}
	if capturedConfig.OS != "Windows" || capturedConfig.Arch != "x86_64" {
		t.Fatalf("config OS/Arch = %q/%q, want %q/%q", capturedConfig.OS, capturedConfig.Arch, "Windows", "x86_64")
	}
}

func TestDoUpdatePropagatesUpdateError(t *testing.T) {
	originalNewClient := newClient
	originalExePath := resolveExecutablePath
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		resolveExecutablePath = originalExePath
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "linux"
	runtimeGOARCH = "amd64"

	expected := errors.New("apply update failed")
	client := &fakeClient{
		release: fakeRelease{
			version: "v1.3.0",
			greaterFn: func(other string) bool {
				return true
			},
		},
		found:     true,
		updateErr: expected,
	}

	newClient = func(config selfupdate.Config) (updateClient, error) {
		return client, nil
	}
	resolveExecutablePath = func() (string, error) {
		return "/usr/local/bin/neocode", nil
	}

	_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.2.0"})
	if !errors.Is(err, expected) {
		t.Fatalf("DoUpdate() error = %v, want %v", err, expected)
	}
}
