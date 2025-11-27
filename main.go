package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gemini-audio/internal/pactl"
	"gopkg.in/yaml.v3"
)

// Profile represents a routing configuration
type Profile struct {
	Name         string        `yaml:"name"`
	Description  string        `yaml:"description"`
	VirtualSink  string        `yaml:"virtual_sink"`
	Applications []Application `yaml:"applications"`
}

// Application within a profile
type Application struct {
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

func usage() {
	fmt.Println("Usage: gemini-audio <command> [args]")
	fmt.Println("Commands: help | list-profiles | reset <profile-file> | setup <profile-file> | status")
}

func main() {
	dry := flag.Bool("dry-run", false, "Print commands without executing them")
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	client := pactl.New(*dry)

	switch cmd {
	case "status":
		out, err := client.Pactl("list", "short", "sinks")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		fmt.Println(string(out))
	case "list-profiles":
		files, err := ioutil.ReadDir("./profiles")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading profiles directory: %v\n", err)
			os.Exit(4)
		}

		fmt.Println("Available profiles:")
		for _, file := range files {
			if !file.IsDir() && (strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml")) {
				fmt.Printf(" - %s\n", file.Name())
			}
		}
	case "setup":
		if flag.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "setup requires a profile file path")
			os.Exit(2)
		}
		profilePath := flag.Arg(1)
		abs, _ := filepath.Abs(profilePath)
		fmt.Printf("Applying profile: %s (dry-run=%v)\n", abs, *dry)

		p, err := loadProfile(profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading profile: %v\n", err)
			os.Exit(3)
		}

		fmt.Printf("Loaded profile: %s\n", p.Name)
		fmt.Printf("  Description: %s\n", p.Description)

		// 1. Create the main virtual sink, if it doesn't exist
		exists, err := client.SinkExists(p.VirtualSink)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not check for sink '%s': %v\n", p.VirtualSink, err)
		}
		if !exists {
			_, err = client.Pactl("load-module", "module-null-sink", fmt.Sprintf("sink_name=%s", p.VirtualSink))
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create virtual sink '%s': %v\n", p.VirtualSink, err)
			} else {
				fmt.Printf("  Created virtual sink: %s\n", p.VirtualSink)
			}
		} else {
			fmt.Printf("  Virtual sink '%s' already exists, skipping creation.\n", p.VirtualSink)
		}

		// 2. Create a second virtual sink to act as a visible microphone
		micSinkName := p.VirtualSink + "-mic"
		micDescription := "Virtual Mic (" + p.Name + ")"
		exists, err = client.SinkExists(micSinkName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not check for sink '%s': %v\n", micSinkName, err)
		}
		if !exists {
			props := fmt.Sprintf(`device.description="%s" device.class="audio" media.class="Audio/Source/Virtual" device.icon_name="audio-input-microphone"`, micDescription)
			_, err = client.Pactl("load-module", "module-null-sink", "sink_name="+micSinkName, "sink_properties="+props)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create virtual mic sink '%s': %v\n", micSinkName, err)
			} else {
				fmt.Printf("  Created virtual mic: %s\n", micDescription)
			}
		} else {
			fmt.Printf("  Virtual mic sink '%s' already exists, skipping creation.\n", micSinkName)
		}

		// 3. Loopback the monitor of the first sink to the second sink
		monitorSource, err := client.GetSinkMonitor(p.VirtualSink)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  failed to get monitor source for '%s': %v\n", p.VirtualSink, err)
		} else {
			loopbackArg := "source=" + monitorSource
			_, err := client.FindModules(loopbackArg)
			if err != nil {
				// We expect an error if it's not found, so we create it.
				_, err = client.Pactl("load-module", "module-loopback", "source="+monitorSource, "sink="+micSinkName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  failed to create loopback: %v\n", err)
				} else {
					fmt.Printf("  Loopback enabled: %s -> %s\n", monitorSource, micSinkName)
				}
			} else {
				fmt.Printf("  Loopback for '%s' already exists, skipping creation.\n", monitorSource)
			}
		}

		// 4. Route applications
		for _, app := range p.Applications {
			if app.Role == "playback" {
				fmt.Printf("  Routing playback for: %s\n", app.Name)
				inputs, err := client.ListSinkInputs()
				if err != nil {
					fmt.Fprintf(os.Stderr, "  failed to list sink inputs: %v\n", err)
					continue
				}

				found := false
				for _, input := range inputs {
					if strings.Contains(strings.ToLower(input.ApplicationName), strings.ToLower(app.Name)) {
						fmt.Printf("    Found sink input #%d (%s)\n", input.ID, input.ApplicationName)
						err := client.MoveSinkInput(input.ID, p.VirtualSink)
						if err != nil {
							fmt.Fprintf(os.Stderr, "    failed to move sink input: %v\n", err)
						} else {
							fmt.Printf("    Moved sink input #%d to %s\n", input.ID, p.VirtualSink)
							found = true
						}
					}
				}
				if !found {
					fmt.Printf("    Warning: could not find a sink input for application '%s'\n", app.Name)
				}
			} else if app.Role == "input_target" {
				fmt.Printf("  Configuring input for: %s\n", app.Name)
				fmt.Printf("    SUCCESS: A virtual microphone has been created.\n")
				fmt.Printf("    In %s, select this as your microphone:\n", app.Name)
				fmt.Printf("    ==> %s\n", micDescription)
			}
		}

	case "reset":
		if flag.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "reset requires a profile file path")
			os.Exit(2)
		}
		profilePath := flag.Arg(1)
		abs, _ := filepath.Abs(profilePath)
		fmt.Printf("Resetting profile: %s (dry-run=%v)\n", abs, *dry)

		p, err := loadProfile(profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading profile: %v\n", err)
			os.Exit(3)
		}

		// Unload all modules associated with this profile, in reverse order of creation.
		micSinkName := p.VirtualSink + "-mic"

		// 1. Unload loopback modules
		// We have to find the monitor source name to find the loopback.
		// This is tricky because there might be multiple sinks with the same name.
		// We will just assume the first one is the one we want to get the monitor name.
		monitorSource, err := client.GetSinkMonitor(p.VirtualSink)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not determine monitor source, loopback modules may not be found: %v\n", err)
		} else {
			loopbackArg := "source=" + monitorSource
			modIDs, err := client.FindModules(loopbackArg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not find any loopback modules: %v\n", err)
			} else {
				fmt.Printf("Found %d loopback modules to unload.\n", len(modIDs))
				for _, modID := range modIDs {
					if err := client.UnloadModule(modID); err != nil {
						fmt.Fprintf(os.Stderr, "failed to unload loopback module %d: %v\n", modID, err)
					} else {
						fmt.Printf("Unloaded loopback module %d\n", modID)
					}
				}
			}
		}

		// 2. Unload mic sink modules
		micModIDs, err := client.FindModules("sink_name=" + micSinkName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not find any modules for mic sink '%s': %v\n", micSinkName, err)
		} else {
			fmt.Printf("Found %d mic sink modules to unload.\n", len(micModIDs))
			for _, modID := range micModIDs {
				if err := client.UnloadModule(modID); err != nil {
					fmt.Fprintf(os.Stderr, "failed to unload mic sink module %d: %v\n", modID, err)
				} else {
					fmt.Printf("Unloaded mic sink module %d\n", modID)
				}
			}
		}

		// 3. Unload main sink modules
		mainModIDs, err := client.FindModules("sink_name=" + p.VirtualSink)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not find any modules for main sink '%s': %v\n", p.VirtualSink, err)
		} else {
			fmt.Printf("Found %d main sink modules to unload.\n", len(mainModIDs))
			for _, modID := range mainModIDs {
				if err := client.UnloadModule(modID); err != nil {
					fmt.Fprintf(os.Stderr, "failed to unload main sink module %d: %v\n", modID, err)
				} else {
					fmt.Printf("Unloaded main sink module %d\n", modID)
				}
			}
		}
		fmt.Println("Reset complete.")

	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(2)
	}
}

func loadProfile(path string) (*Profile, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile: %w", err)
	}

	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}
	return &p, nil
}