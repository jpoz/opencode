package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/config"
	"github.com/sst/opencode/internal/db"
	"github.com/sst/opencode/internal/llm/agent"
	"github.com/sst/opencode/internal/remote"
)

var (
	serverHost     string
	serverPort     int
	serverConfig   string
	serverTLS      bool
	serverCertFile string
	serverKeyFile  string
	serverTokens   []string
	serverMaxConns int
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the OpenCode remote server",
	Long: `Start the OpenCode remote server to allow remote clients to connect and control the coding agent.

The server provides a WebSocket-based API that allows remote clients (web browsers, mobile apps, IDEs)
to interact with the OpenCode agent. All tool execution and file operations are securely sandboxed
and require proper authentication and authorization.

Configuration can be provided via command line flags, environment variables, or a YAML config file.`,
	RunE: runServer,
}

func init() {
	// Add server command to root
	rootCmd.AddCommand(serverCmd)

	// Server configuration flags
	serverCmd.Flags().StringVar(&serverHost, "host", "localhost", "Host to bind the server to")
	serverCmd.Flags().IntVar(&serverPort, "port", 8080, "Port to bind the server to")
	serverCmd.Flags().StringVar(&serverConfig, "config", "", "Path to server configuration file")
	serverCmd.Flags().BoolVar(&serverTLS, "tls", false, "Enable TLS/SSL")
	serverCmd.Flags().StringVar(&serverCertFile, "cert", "", "TLS certificate file")
	serverCmd.Flags().StringVar(&serverKeyFile, "key", "", "TLS private key file")
	serverCmd.Flags().StringSliceVar(&serverTokens, "token", []string{}, "Authentication tokens (can be specified multiple times)")
	serverCmd.Flags().IntVar(&serverMaxConns, "max-connections", 100, "Maximum number of concurrent connections")

	// Bind flags to viper for configuration file support
	viper.BindPFlag("server.host", serverCmd.Flags().Lookup("host"))
	viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.tls.enabled", serverCmd.Flags().Lookup("tls"))
	viper.BindPFlag("server.tls.cert_file", serverCmd.Flags().Lookup("cert"))
	viper.BindPFlag("server.tls.key_file", serverCmd.Flags().Lookup("key"))
	viper.BindPFlag("server.max_connections", serverCmd.Flags().Lookup("max-connections"))
}

func runServer(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling EARLY - before any potentially blocking operations
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	slog.Debug("Signal handler registered for server shutdown")

	// Load configuration
	if err := loadServerConfig(); err != nil {
		return fmt.Errorf("failed to load server configuration: %w", err)
	}

	// Load the config
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	
	// Setup logging level variable like in root.go
	lvl := new(slog.LevelVar)
	_, err = config.Load(cwd, false, lvl)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	conn, err := db.Connect()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer conn.Close()

	// Create server configuration
	serverConfig := createServerConfig()

	// Create WebSocket server
	wsServer := remote.NewWebSocketServer(serverConfig)

	// Create authentication service
	authService := createAuthService()

	// Create file system bridge
	fileSystemBridge := createFileSystemBridge()

	// Create tool bridge
	toolBridge := remote.NewRemoteToolBridge(authService, fileSystemBridge)

	// Create the app with backward compatibility function
	app, err := app.NewWithoutUI(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	// Register tools with the bridge
	if err := registerToolsWithBridge(toolBridge, app); err != nil {
		return fmt.Errorf("failed to register tools: %w", err)
	}

	slog.Info("Starting OpenCode remote server",
		"host", serverConfig.Host,
		"port", serverConfig.Port,
		"tls_enabled", serverConfig.TLS != nil && serverConfig.TLS.Enabled,
		"max_connections", serverConfig.MaxConnections)

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := wsServer.Start(); err != nil {
			serverErr <- fmt.Errorf("server failed: %w", err)
		} else {
			// Server stopped gracefully
			serverErr <- nil
		}
	}()

	// Signal handler was already set up early in the function

	select {
	case sig := <-sigChan:
		slog.Info("Received signal, shutting down server", "signal", sig)
	case err := <-serverErr:
		slog.Error("Server error", "error", err)
		return err
	}

	// Graceful shutdown
	slog.Info("Shutting down server...")
	if err := wsServer.Stop(); err != nil {
		slog.Error("Error stopping server", "error", err)
		return err
	}

	slog.Info("Server shutdown complete")
	return nil
}

func loadServerConfig() error {
	if serverConfig != "" {
		// Load configuration from file
		viper.SetConfigFile(serverConfig)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		slog.Info("Loaded server configuration", "config_file", serverConfig)
	}

	// Set defaults
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.max_connections", 100)
	viper.SetDefault("server.heartbeat_interval", "30s")
	viper.SetDefault("server.connection_timeout", "60s")
	viper.SetDefault("server.read_buffer_size", 1024)
	viper.SetDefault("server.write_buffer_size", 1024)
	viper.SetDefault("server.enable_compression", true)
	viper.SetDefault("server.allowed_origins", []string{"*"})

	return nil
}

func createServerConfig() *remote.ServerConfig {
	config := &remote.ServerConfig{
		Host:              viper.GetString("server.host"),
		Port:              viper.GetInt("server.port"),
		MaxConnections:    viper.GetInt("server.max_connections"),
		HeartbeatInterval: viper.GetDuration("server.heartbeat_interval"),
		ConnectionTimeout: viper.GetDuration("server.connection_timeout"),
		ReadBufferSize:    viper.GetInt("server.read_buffer_size"),
		WriteBufferSize:   viper.GetInt("server.write_buffer_size"),
		EnableCompression: viper.GetBool("server.enable_compression"),
		AllowedOrigins:    viper.GetStringSlice("server.allowed_origins"),
	}

	// TLS configuration
	if viper.GetBool("server.tls.enabled") {
		config.TLS = &remote.TLSConfig{
			Enabled:  true,
			CertFile: viper.GetString("server.tls.cert_file"),
			KeyFile:  viper.GetString("server.tls.key_file"),
		}
	}

	return config
}

func createAuthService() remote.AuthService {
	authService := remote.NewTokenAuthService()

	// Create default admin user if no users exist
	users, _ := authService.ListUsers()
	if len(users) == 0 {
		// Create admin user
		adminUser, err := authService.CreateUser("admin", remote.AdminPermissions())
		if err != nil {
			slog.Error("Failed to create admin user", "error", err)
		} else {
			// Create token for admin user
			token, err := authService.CreateToken(adminUser.ID, nil)
			if err != nil {
				slog.Error("Failed to create admin token", "error", err)
			} else {
				slog.Info("Created admin user", "user_id", adminUser.ID, "token", token)
				fmt.Printf("\n🔑 Admin token created: %s\n", token)
				fmt.Printf("Use this token to authenticate with the server.\n\n")
			}
		}

		// Create read-only demo user
		demoUser, err := authService.CreateUser("demo", remote.ReadOnlyPermissions())
		if err != nil {
			slog.Error("Failed to create demo user", "error", err)
		} else {
			// Create token for demo user
			token, err := authService.CreateToken(demoUser.ID, nil)
			if err != nil {
				slog.Error("Failed to create demo token", "error", err)
			} else {
				slog.Info("Created demo user", "user_id", demoUser.ID, "token", token)
				fmt.Printf("📖 Demo (read-only) token created: %s\n\n", token)
			}
		}
	}

	// Add tokens from command line
	for _, tokenStr := range serverTokens {
		// For command line tokens, create a generic user
		user, err := authService.CreateUser("cli_user_"+tokenStr[:8], remote.DeveloperPermissions())
		if err != nil {
			slog.Error("Failed to create CLI user", "error", err)
			continue
		}

		// This is a simplified approach - in production you'd want proper token management
		slog.Info("Created CLI user", "user_id", user.ID, "token_prefix", tokenStr[:8])
	}

	return authService
}

func createFileSystemBridge() remote.FileSystemBridge {
	// Get allowed and denied paths from configuration
	allowedPaths := viper.GetStringSlice("security.file_access.allowed_paths")
	deniedPaths := viper.GetStringSlice("security.file_access.denied_paths")

	// Set defaults if not configured
	if len(allowedPaths) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		allowedPaths = []string{cwd}
		slog.Info("Using current directory as allowed path", "path", cwd)
	}

	if len(deniedPaths) == 0 {
		deniedPaths = []string{
			"/etc",
			"/usr/bin",
			"/usr/sbin",
			"/bin",
			"/sbin",
			"/var",
			"/root",
		}
	}

	slog.Info("File system access configured",
		"allowed_paths", allowedPaths,
		"denied_paths", deniedPaths)

	return remote.NewSandboxFileSystemBridge(allowedPaths, deniedPaths)
}

func registerToolsWithBridge(bridge *remote.RemoteToolBridge, app *app.App) error {
	// Get the agent tools
	tools := agent.PrimaryAgentTools(
		app.Permissions,
		app.Sessions,
		app.Messages,
		app.History,
		app.LSPClients,
	)

	// Register tools with the bridge
	if err := bridge.RegisterTools(tools); err != nil {
		return fmt.Errorf("failed to register agent tools: %w", err)
	}

	// Set allowed tools from configuration
	allowedTools := viper.GetStringSlice("security.allowed_tools")
	if len(allowedTools) > 0 {
		bridge.SetAllowedTools(allowedTools)
		slog.Info("Tool access restricted", "allowed_tools", allowedTools)
	} else {
		slog.Info("All tools are allowed (no restrictions configured)")
	}

	slog.Info("Registered tools with bridge", "tool_count", len(tools))
	return nil
}

// Example server configuration file that can be created
func generateExampleConfig() string {
	return `# OpenCode Remote Server Configuration

server:
  host: "0.0.0.0"
  port: 8080
  max_connections: 100
  heartbeat_interval: "30s"
  connection_timeout: "60s"
  
  # TLS/SSL Configuration
  tls:
    enabled: false
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"

# Security Configuration
security:
  # Allowed tools (empty means all tools are allowed)
  allowed_tools: 
    - "bash"
    - "edit"
    - "view"
    - "glob"
    - "grep"
    - "ls"
  
  # File system access control
  file_access:
    allowed_paths:
      - "/workspace"
      - "/home/user/projects"
    denied_paths:
      - "/etc"
      - "/usr"
      - "/var"
      - "/root"

# Rate limiting
rate_limits:
  requests_per_minute: 60
  concurrent_sessions: 10

# Logging
logging:
  level: "info"
  format: "json"
`
}