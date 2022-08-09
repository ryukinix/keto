package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/ory/keto/embedx"

	"github.com/ory/x/configx"

	"github.com/ory/x/logrusx"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/keto/internal/namespace"
)

// configFile writes the content to a temporary file, returning the path.
// Good for testing config files.
func configFile(t *testing.T, content string) (path string) {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Remove(f.Name()) })

	n, err := f.WriteString(content)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(content) {
		t.Fatal("failed to write the complete content")
	}

	return f.Name()
}

func TestKoanfNamespaceManager(t *testing.T) {
	setup := func(t *testing.T, configFile string) (*test.Hook, *Config) {
		hook := test.Hook{}
		l := logrusx.New("test", "today", logrusx.WithHook(&hook))

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		config, err := NewDefault(
			ctx,
			pflag.NewFlagSet("test", pflag.ContinueOnError),
			l,
			// configx.SkipValidation(),
			configx.WithConfigFiles(configFile),
		)
		require.NoError(t, err)

		return &hook, config
	}

	assertNamespaces := func(t *testing.T, p *Config, nn ...*namespace.Namespace) {
		nm, err := p.NamespaceManager()
		require.NoError(t, err)

		actualNamespaces, err := nm.Namespaces(context.Background())
		require.NoError(t, err)
		assert.Equal(t, len(nn), len(actualNamespaces))

		for _, n := range nn {
			assert.Contains(t, actualNamespaces, n)
		}
	}

	t.Run("case=creates memory namespace manager when namespaces are set", func(t *testing.T) {
		config := configFile(t, `
dsn: memory
namespaces:
  - name: n0
  - name: n1
  - name: n2`)

		run := func(namespaces []*namespace.Namespace) func(*testing.T) {
			return func(t *testing.T) {
				_, p := setup(t, config)

				assertNamespaces(t, p, namespaces...)

				nm, err := p.NamespaceManager()
				require.NoError(t, err)
				_, ok := nm.(*memoryNamespaceManager)
				assert.True(t, ok)
			}

		}

		nn := []*namespace.Namespace{
			{Name: "n0"},
			{Name: "n1"},
			{Name: "n2"},
		}
		nnJson, err := json.Marshal(nn)
		require.NoError(t, err)
		nnValue := make([]interface{}, 0)
		require.NoError(t, json.Unmarshal(nnJson, &nnValue))

		t.Run(
			"type=[]*namespace.Namespace",
			run(nn),
		)
	})

	t.Run("case=reloads namespace manager when namespaces are updated using Set()", func(t *testing.T) {
		_, p := setup(t, configFile(t, "dsn: memory"))

		n0 := &namespace.Namespace{
			Name: "n0",
		}
		n1 := &namespace.Namespace{
			Name: "n1",
		}

		require.NoError(t, p.Set(KeyNamespaces, []*namespace.Namespace{n0}))
		assertNamespaces(t, p, n0)

		require.NoError(t, p.Set(KeyNamespaces, []*namespace.Namespace{n1}))
		assertNamespaces(t, p, n1)
	})

	t.Run("case=creates watcher manager when namespaces is string URL", func(t *testing.T) {
		_, p := setup(t, configFile(t, fmt.Sprintf(`
dsn: memory
namespaces: file://%s`,
			t.TempDir())))

		nm, err := p.NamespaceManager()
		require.NoError(t, err)
		_, ok := nm.(*NamespaceWatcher)
		assert.True(t, ok)
	})

	t.Run("case=uses passed configx provider", func(t *testing.T) {
		ctx := context.Background()
		cp, err := configx.New(ctx, embedx.ConfigSchema, configx.WithValue(KeyDSN, "foobar"))
		require.NoError(t, err)

		p := New(ctx, logrusx.New("test", "today"), cp)
		assert.Equal(t, "foobar", p.DSN())
		assert.Same(t, cp, p.p)
	})
}
