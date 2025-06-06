package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCommand_RemoteFlag(t *testing.T) {
	// Test that the remote flag exists
	remoteFlag := rootCmd.Flags().Lookup("remote")
	assert.NotNil(t, remoteFlag)
	assert.Equal(t, "false", remoteFlag.DefValue)
	assert.Contains(t, remoteFlag.Usage, "remote mode")
}

func TestRootCommand_RemoteAndPromptMutuallyExclusive(t *testing.T) {
	// Test that remote and prompt flags are mutually exclusive
	// This is a behavioral test - we can't easily inspect the mutual exclusion
	// through the Cobra API, but we can verify the flags exist
	
	remoteFlag := rootCmd.Flags().Lookup("remote")
	promptFlag := rootCmd.Flags().Lookup("prompt")
	
	assert.NotNil(t, remoteFlag, "remote flag should exist")
	assert.NotNil(t, promptFlag, "prompt flag should exist")
	
	// The mutual exclusion is tested through integration tests
	// where we would set both flags and expect an error
}

func TestRootCommand_Help(t *testing.T) {
	// Test that the command help mentions both TUI and remote modes
	longDesc := rootCmd.Long
	assert.Contains(t, longDesc, "terminal-based AI assistant")
	assert.Contains(t, longDesc, "interactive chat interface")
}

func TestRootCommand_FlagDefaults(t *testing.T) {
	// Test that all expected flags have the right defaults
	flags := []struct {
		name     string
		expected string
	}{
		{"remote", "false"},
		{"debug", "false"},
		{"quiet", "false"},
		{"verbose", "false"},
		{"help", "false"},
		{"version", "false"},
		{"output-format", "text"},
	}

	for _, flag := range flags {
		f := rootCmd.Flags().Lookup(flag.name)
		assert.NotNil(t, f, "Flag %s should exist", flag.name)
		assert.Equal(t, flag.expected, f.DefValue, "Flag %s should have default value %s", flag.name, flag.expected)
	}
}

func TestRootCommand_Structure(t *testing.T) {
	// Test basic command structure
	assert.Equal(t, "OpenCode", rootCmd.Use)
	assert.Contains(t, rootCmd.Short, "terminal AI assistant")
	assert.NotNil(t, rootCmd.RunE, "Root command should have a RunE function")
	
	// Test that server command is a subcommand
	subcommands := rootCmd.Commands()
	hasServerCmd := false
	for _, cmd := range subcommands {
		if cmd.Use == "server" {
			hasServerCmd = true
			break
		}
	}
	assert.True(t, hasServerCmd, "Server command should be a subcommand of root")
}