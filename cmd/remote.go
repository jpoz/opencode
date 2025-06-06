package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

var (
	remoteHost  string
	remotePort  int
	remoteToken string
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Connect to a remote OpenCode server",
	Long: `Connect to a remote OpenCode server and interact with the coding agent.

This command connects to an OpenCode server running on another machine and provides
an interactive session. You need the server's host, port, and an authentication token.

Get the authentication token from the server when it starts up - it will display
admin and demo tokens that you can use to authenticate.`,
	RunE: runRemoteClient,
}

func init() {
	// Add remote command to root
	rootCmd.AddCommand(remoteCmd)

	// Remote client configuration flags
	remoteCmd.Flags().StringVar(&remoteHost, "host", "localhost", "Remote server host to connect to")
	remoteCmd.Flags().IntVar(&remotePort, "port", 8080, "Remote server port to connect to")
	remoteCmd.Flags().StringVar(&remoteToken, "token", "", "Authentication token for remote server (required)")

	// Mark token as required
	remoteCmd.MarkFlagRequired("token")
}

func runRemoteClient(cmd *cobra.Command, args []string) error {
	slog.Info("Connecting to remote OpenCode server", 
		"host", remoteHost, 
		"port", remotePort)
	
	// Validate inputs
	if remoteHost == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if remotePort <= 0 || remotePort > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if remoteToken == "" {
		return fmt.Errorf("authentication token is required")
	}

	// Show connection details
	fmt.Printf("Connecting to: ws://%s:%d/ws\n", remoteHost, remotePort)
	fmt.Printf("Using token: %s...\n", remoteToken[:min(8, len(remoteToken))])
	
	// TODO: Implement actual WebSocket client connection
	fmt.Println("\n⚠️  Remote client connection not yet implemented!")
	fmt.Println("This would connect to the remote server and provide an interactive session.")
	fmt.Println("\nTo complete the remote functionality:")
	fmt.Println("1. WebSocket client implementation needed")
	fmt.Println("2. Interactive session handling")
	fmt.Println("3. Real-time bidirectional communication")
	
	fmt.Println("\n📋 Current command structure is ready:")
	fmt.Printf("   Server: ./opencode server --host=0.0.0.0 --port=%d\n", remotePort)
	fmt.Printf("   Client: ./opencode remote --host=%s --port=%d --token=YOUR_TOKEN\n", remoteHost, remotePort)
	
	return fmt.Errorf("remote client connection not yet implemented - Phase 2 needed")
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}