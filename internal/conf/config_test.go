package conf

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestDefaultPathsUseDataDirEnv(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "octopus-data")
	t.Setenv("OCTOPUS_DATA_DIR", dataDir)

	if got, want := defaultDataDir(), filepath.Clean(dataDir); got != want {
		t.Fatalf("defaultDataDir() = %q, want %q", got, want)
	}
	if got, want := defaultConfigPath(), filepath.Join(filepath.Clean(dataDir), "config.json"); got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
	if got, want := defaultDatabasePath(), filepath.Join(filepath.Clean(dataDir), "data.db"); got != want {
		t.Fatalf("defaultDatabasePath() = %q, want %q", got, want)
	}
}

func TestDefaultPathsFallbackToDataDir(t *testing.T) {
	t.Setenv("OCTOPUS_DATA_DIR", "")

	if got, want := defaultDataDir(), "data"; got != want {
		t.Fatalf("defaultDataDir() = %q, want %q", got, want)
	}
	if got, want := defaultConfigPath(), filepath.Join("data", "config.json"); got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
	if got, want := defaultDatabasePath(), filepath.Join("data", "data.db"); got != want {
		t.Fatalf("defaultDatabasePath() = %q, want %q", got, want)
	}
}

func TestServerReadTimeoutDefaultsAndEnvironmentOverride(t *testing.T) {
	dataDir := t.TempDir()
	for _, test := range []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "default", want: ServerReadTimeout},
		{name: "duration environment override", value: "5m", want: 5 * time.Minute},
		{name: "explicit zero disables limit", value: "0s", want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			previous := AppConfig
			t.Cleanup(func() { AppConfig = previous })
			t.Setenv("OCTOPUS_DATA_DIR", filepath.Join(dataDir, test.name))
			t.Setenv("OCTOPUS_AUTH_JWT_SECRET", "test-secret")
			if test.value != "" {
				t.Setenv("OCTOPUS_SERVER_READ_TIMEOUT", test.value)
			}

			if err := Load(""); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := AppConfig.Server.ReadTimeout; got != test.want {
				t.Fatalf("Server.ReadTimeout = %s, want %s", got, test.want)
			}
		})
	}
}
