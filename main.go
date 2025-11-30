package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"gemini-audio/internal/pactl"
	"gemini-audio/internal/profiles"
)

func usage() {
	fmt.Println("Usage: gemini-audio [options] [command]")
	fmt.Println()
	fmt.Println("Daemon Mode (default):")
	fmt.Println("  gemini-audio [--dry-run]          Run as foreground daemon, deploy all profiles")
	fmt.Println("                                     and cleanup on shutdown")
	fmt.Println()
	fmt.Println("CLI Commands:")
	fmt.Println("  help                              Show this help message")
	fmt.Println("  list-profiles                     List available profiles")
	fmt.Println("  status                            Show current audio sinks")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --dry-run                         Print commands without executing them")
	fmt.Println("  --profiles-dir <path>             Profile directory (default: ./profiles)")
}

func main() {
	dry := flag.Bool("dry-run", false, "Print commands without executing them")
	profilesDir := flag.String("profiles-dir", "./profiles", "Directory containing profile YAML files")
	flag.Parse()

	args := flag.Args()

	// If a command is provided, execute it
	if len(args) > 0 {
		cmd := args[0]
		client := pactl.New(*dry)
		handleCommand(cmd, args[1:], client, *profilesDir)
		return
	}

	// Otherwise, run in daemon mode
	runDaemon(*dry, *profilesDir)
}

func handleCommand(cmd string, args []string, client *pactl.Client, profilesDir string) {
	switch cmd {
	case "help":
		usage()
	case "status":
		out, err := client.Pactl("list", "short", "sinks")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		fmt.Println(string(out))
	case "list-profiles":
		profileList, err := profiles.ListProfiles(profilesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading profiles directory: %v\n", err)
			os.Exit(4)
		}
		if len(profileList) == 0 {
			fmt.Println("No profiles found.")
			return
		}
		fmt.Println("Available profiles:")
		for _, p := range profileList {
			fmt.Printf(" - %s\n", p)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(2)

	}
}

func runDaemon(dry bool, profilesDir string) {
	client := pactl.New(dry)
	manager := profiles.NewManager(client)

	// Track deployed profiles for cleanup
	deployedProfiles := make([]*profiles.Profile, 0)
	var mu sync.Mutex

	// Load all profiles from the directory
	profileList, err := profiles.ListProfiles(profilesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading profiles directory: %v\n", err)
		os.Exit(4)
	}

	if len(profileList) == 0 {
		fmt.Fprintf(os.Stderr, "no profiles found in %s\n", profilesDir)
		os.Exit(3)
	}

	fmt.Printf("🎵 PubTalk Daemon starting...\n")
	fmt.Printf("📁 Profiles directory: %s\n", profilesDir)
	fmt.Printf("📋 Found %d profile(s)\n", len(profileList))

	// Deploy all profiles
	for _, profileName := range profileList {
		profilePath := fmt.Sprintf("%s/%s", profilesDir, profileName)
		profile, err := profiles.LoadProfile(profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading profile %s: %v\n", profileName, err)
			continue
		}

		fmt.Printf("\n📝 Deploying profile: %s\n", profile.Name)
		fmt.Printf("   Description: %s\n", profile.Description)

		if err := manager.Deploy(profile); err != nil {
			fmt.Fprintf(os.Stderr, "error deploying profile %s: %v\n", profile.Name, err)
		} else {
			fmt.Printf("   ✅ Profile deployed successfully\n")
			mu.Lock()
			deployedProfiles = append(deployedProfiles, profile)
			mu.Unlock()
		}
	}

	fmt.Printf("\n✨ All profiles deployed. Daemon running...\n")
	fmt.Println("Press Ctrl+C to shutdown and cleanup.")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	fmt.Printf("\n\n🛑 Received signal: %v\n", sig)
	fmt.Println("🧹 Cleaning up deployed profiles...")

	// Reset profiles in reverse order
	mu.Lock()
	for i := len(deployedProfiles) - 1; i >= 0; i-- {
		profile := deployedProfiles[i]
		fmt.Printf("\n🔄 Resetting profile: %s\n", profile.Name)
		if err := manager.Reset(profile); err != nil {
			fmt.Fprintf(os.Stderr, "error resetting profile %s: %v\n", profile.Name, err)
		} else {
			fmt.Printf("   ✅ Profile reset successfully\n")
		}
	}
	mu.Unlock()

	fmt.Println("\n👋 Daemon shutdown complete.")
	os.Exit(0)
}
