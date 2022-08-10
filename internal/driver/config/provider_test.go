package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/ory/x/configx"
	"github.com/ory/x/logrusx"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/keto/embedx"
	"github.com/ory/keto/internal/namespace"
)

// createFile writes the content to a temporary file, returning the path.
// Good for testing config files.
func createFile(t *testing.T, content string) (path string) {
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

// createFileF writes the content format string with the applied args to a
// temporary file, returning the path. Good for testing config files.
func createFileF(t *testing.T, contentF string, args ...any) (path string) {
	return createFile(t, fmt.Sprintf(contentF, args...))
}

func setup(t *testing.T, configFile string) (*test.Hook, *Config) {
	t.Helper()
	hook := test.Hook{}
	l := logrusx.New("test", "today", logrusx.WithHook(&hook))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	config, err := NewDefault(
		ctx,
		pflag.NewFlagSet("test", pflag.ContinueOnError),
		l,
		configx.WithConfigFiles(configFile),
	)
	require.NoError(t, err)

	return &hook, config
}

func assertNamespaces(t *testing.T, p *Config, nn ...*namespace.Namespace) {
	t.Helper()

	nm, err := p.NamespaceManager()
	require.NoError(t, err)
	actualNamespaces, err := nm.Namespaces(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(nn), len(actualNamespaces))
	assert.ElementsMatch(t, nn, actualNamespaces)
}

// The new way to configure namespaces is through the Ory Permissions Language.
// We check here that we still support enumerating the namespaces directly in
// the config or through a file reference, in which case there should be no
// rewrites configured.
func TestLegacyNamespaceConfig(t *testing.T) {
	t.Run("case=creates memory namespace manager when namespaces are set", func(t *testing.T) {
		config := createFile(t, `
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
		_, p := setup(t, createFile(t, "dsn: memory"))

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
		_, p := setup(t, createFileF(t, `
dsn: memory
namespaces: file://%s`,
			t.TempDir()))

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

// Test that the namespaces can be configured through the Ory Permission
// Language.
func TestRewritesNamespaceConfig(t *testing.T) {
	t.Run("case=one file", func(t *testing.T) {
		oplConfig := createFile(t, `
class User implements Namespace {
  related: {
    manager: User[]
  }
}
  
class Group implements Namespace {
  related: {
    members: (User | Group)[]
  }
}
		`)
		config := createFileF(t, `
dsn: memory
namespaces:
  config: file://%s`, oplConfig)

		_, p := setup(t, config)
		nm, err := p.NamespaceManager()
		require.NoError(t, err)
		namespaces, err := nm.Namespaces(context.Background())
		require.NoError(t, err)
		assert.Len(t, namespaces, 2)

		users, groups := namespaces[0], namespaces[1]

		assert.Equal(t, "User", users.Name)
		assert.Equal(t, "manager", users.Relations[0].Name)

		assert.Equal(t, "Group", groups.Name)
		assert.Equal(t, "members", groups.Relations[0].Name)
	})

}
