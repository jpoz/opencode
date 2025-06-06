package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/sst/opencode/internal/remote"
)

func TestCreateServerConfig(t *testing.T) {
	// Set test values
	viper.Set("server.host", "test-host")
	viper.Set("server.port", 9090)
	viper.Set("server.max_connections", 50)
	viper.Set("server.tls.enabled", true)
	viper.Set("server.tls.cert_file", "/test/cert.pem")
	viper.Set("server.tls.key_file", "/test/key.pem")

	config := createServerConfig()

	assert.Equal(t, "test-host", config.Host)
	assert.Equal(t, 9090, config.Port)
	assert.Equal(t, 50, config.MaxConnections)
	assert.NotNil(t, config.TLS)
	assert.True(t, config.TLS.Enabled)
	assert.Equal(t, "/test/cert.pem", config.TLS.CertFile)
	assert.Equal(t, "/test/key.pem", config.TLS.KeyFile)

	// Clean up
	viper.Reset()
}

func TestCreateServerConfig_NoTLS(t *testing.T) {
	// Set test values without TLS
	viper.Set("server.host", "localhost")
	viper.Set("server.port", 8080)
	viper.Set("server.tls.enabled", false)

	config := createServerConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8080, config.Port)
	assert.Nil(t, config.TLS)

	// Clean up
	viper.Reset()
}

func TestCreateAuthService(t *testing.T) {
	authService := createAuthService()

	assert.NotNil(t, authService)

	// Check that default users were created
	users, err := authService.ListUsers()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), 2) // admin and demo users

	// Verify admin user exists
	adminFound := false
	demoFound := false
	for _, user := range users {
		if user.Username == "admin" {
			adminFound = true
			// Admin should have admin permissions
			assert.Contains(t, user.Permissions, remote.Permission{
				Action:   "*",
				Resource: "*",
				Allow:    true,
			})
		}
		if user.Username == "demo" {
			demoFound = true
			// Demo should have read-only permissions
			hasReadOnlyPerms := false
			for _, perm := range user.Permissions {
				if perm.Action == "tool.read" && perm.Allow {
					hasReadOnlyPerms = true
					break
				}
			}
			assert.True(t, hasReadOnlyPerms)
		}
	}

	assert.True(t, adminFound, "Admin user should be created")
	assert.True(t, demoFound, "Demo user should be created")
}

func TestCreateFileSystemBridge(t *testing.T) {
	// Test with custom paths
	viper.Set("security.file_access.allowed_paths", []string{"/test/allowed"})
	viper.Set("security.file_access.denied_paths", []string{"/test/denied"})

	bridge := createFileSystemBridge()
	assert.NotNil(t, bridge)

	// Create a test user
	user := &remote.User{ID: "test_user"}

	// Test allowed path
	err := bridge.ValidatePath("/test/allowed/file.txt", user)
	assert.NoError(t, err)

	// Test denied path
	err = bridge.ValidatePath("/test/denied/file.txt", user)
	assert.Error(t, err)

	// Clean up
	viper.Reset()
}

func TestCreateFileSystemBridge_Defaults(t *testing.T) {
	// Test with no configuration (should use defaults)
	viper.Reset()

	bridge := createFileSystemBridge()
	assert.NotNil(t, bridge)

	user := &remote.User{ID: "test_user"}

	// Test that common system paths are denied
	err := bridge.ValidatePath("/etc/passwd", user)
	assert.Error(t, err)

	err = bridge.ValidatePath("/usr/bin/ls", user)
	assert.Error(t, err)
}

func TestGenerateExampleConfig(t *testing.T) {
	config := generateExampleConfig()
	
	assert.NotEmpty(t, config)
	assert.Contains(t, config, "server:")
	assert.Contains(t, config, "security:")
	assert.Contains(t, config, "allowed_tools:")
	assert.Contains(t, config, "file_access:")
	assert.Contains(t, config, "rate_limits:")
}

func TestServerCommand_Flags(t *testing.T) {
	// Test that the server command has the expected flags
	cmd := serverCmd

	assert.NotNil(t, cmd.Flags().Lookup("host"))
	assert.NotNil(t, cmd.Flags().Lookup("port"))
	assert.NotNil(t, cmd.Flags().Lookup("config"))
	assert.NotNil(t, cmd.Flags().Lookup("tls"))
	assert.NotNil(t, cmd.Flags().Lookup("cert"))
	assert.NotNil(t, cmd.Flags().Lookup("key"))
	assert.NotNil(t, cmd.Flags().Lookup("token"))
	assert.NotNil(t, cmd.Flags().Lookup("max-connections"))

	// Test default values
	hostFlag := cmd.Flags().Lookup("host")
	assert.Equal(t, "localhost", hostFlag.DefValue)

	portFlag := cmd.Flags().Lookup("port")
	assert.Equal(t, "8080", portFlag.DefValue)

	maxConnsFlag := cmd.Flags().Lookup("max-connections")
	assert.Equal(t, "100", maxConnsFlag.DefValue)
}

func TestLoadServerConfig_Defaults(t *testing.T) {
	// Reset viper to clean state
	viper.Reset()

	err := loadServerConfig()
	assert.NoError(t, err)

	// Check that defaults are set
	assert.Equal(t, "localhost", viper.GetString("server.host"))
	assert.Equal(t, 8080, viper.GetInt("server.port"))
	assert.Equal(t, 100, viper.GetInt("server.max_connections"))
	assert.Equal(t, true, viper.GetBool("server.enable_compression"))

	// Clean up
	viper.Reset()
}