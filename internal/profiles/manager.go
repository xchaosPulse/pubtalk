package profiles

import (
	"fmt"
	"strings"

	"gemini-audio/internal/pactl"
)

// Manager implements profile deployment for Ubuntu 24.04 / PipeWire.
// Implementation notes:
// - We create a virtual null-sink for the profile's output (`virtual-out-...`).
// - We create a second null-sink to act as the visible microphone (`...-mic`).
// - A loopback module feeds the first sink's monitor into the mic sink, and apps
//   select the mic sink's `.monitor` as their recording source. This works reliably
//   under PipeWire's PulseAudio compatibility layer and avoids creating ambiguous
//   remapped sources which sometimes appear as duplicates.

// Manager handles deployment and reset of audio profiles
type Manager struct {
	client *pactl.Client
}

// NewManager creates a new profile manager
func NewManager(client *pactl.Client) *Manager {
	return &Manager{client: client}
}

// Deploy applies a profile configuration
func (m *Manager) Deploy(p *Profile) error {
	// 1. Create the main virtual sink, if it doesn't exist
	exists, err := m.client.SinkExists(p.VirtualSink)
	if err != nil {
		return fmt.Errorf("could not check for sink '%s': %w", p.VirtualSink, err)
	}
	if !exists {
		_, err = m.client.Pactl("load-module", "module-null-sink", fmt.Sprintf("sink_name=%s", p.VirtualSink))
		if err != nil {
			return fmt.Errorf("failed to create virtual sink '%s': %w", p.VirtualSink, err)
		}
		fmt.Printf("     - Created virtual sink: %s\n", p.VirtualSink)
	} else {
		fmt.Printf("     - Virtual sink '%s' already exists, skipping creation\n", p.VirtualSink)
	}

	// 2. Create a virtual recording device (mic sink) for the microphone input
	// Applications will select its monitor as their microphone
	micSinkName := p.VirtualSink + "-mic"
	micDescription := "Virtual Mic (" + p.Name + ")"

	exists, err = m.client.SinkExists(micSinkName)
	if err != nil {
		return fmt.Errorf("could not check for mic sink '%s': %w", micSinkName, err)
	}

	if !exists {
		// Create a null sink for the microphone
		_, err = m.client.Pactl("load-module", "module-null-sink", "sink_name="+micSinkName, fmt.Sprintf(`sink_properties="device.description='%s' device.icon_name=audio-input-microphone"`, micDescription))
		if err != nil {
			return fmt.Errorf("failed to create virtual mic sink '%s': %w", micSinkName, err)
		}
		fmt.Printf("     - Created virtual mic: %s\n", micDescription)
	}

	// Create a loopback that feeds the main sink's audio into this mic sink
	monitorSource, err := m.client.GetSinkMonitor(p.VirtualSink)
	if err != nil {
		return fmt.Errorf("failed to get monitor source for '%s': %w", p.VirtualSink, err)
	}

	// Create loopback with low latency for real-time monitoring.
	// Avoid creating duplicates by checking for an existing module with both source and sink args.
	loopArgs := []string{"source=" + monitorSource, "sink=" + micSinkName}
	if _, err := m.client.FindModulesMatching(loopArgs); err != nil {
		_, err = m.client.Pactl("load-module", "module-loopback", "source="+monitorSource, "sink="+micSinkName, "latency_msec=1")
		if err != nil {
			return fmt.Errorf("failed to create loopback: %w", err)
		}
		fmt.Printf("     - Created loopback: %s -> %s\n", monitorSource, micSinkName)
	} else {
		fmt.Printf("     - Loopback already exists for %s -> %s, skipping\n", monitorSource, micSinkName)
	}

	// 4. Route applications
	for _, app := range p.Applications {
		if app.Role == "playback" {
			fmt.Printf("     - Routing playback for: %s\n", app.Name)
			inputs, err := m.client.ListSinkInputs()
			if err != nil {
				fmt.Printf("       Warning: failed to list sink inputs: %v\n", err)
				continue
			}

			found := false
			for _, input := range inputs {
				if strings.Contains(strings.ToLower(input.ApplicationName), strings.ToLower(app.Name)) {
					fmt.Printf("       Found sink input #%d (%s)\n", input.ID, input.ApplicationName)
					err := m.client.MoveSinkInput(input.ID, p.VirtualSink)
					if err != nil {
						fmt.Printf("       Warning: failed to move sink input: %v\n", err)
					} else {
						fmt.Printf("       Moved sink input #%d to %s\n", input.ID, p.VirtualSink)
						found = true
					}
				}
			}
			if !found {
				fmt.Printf("       Warning: could not find active sink input for application '%s'\n", app.Name)
			}
		} else if app.Role == "input_target" {
			fmt.Printf("     - Configuring input for: %s\n", app.Name)
			fmt.Printf("       ✓ Virtual microphone ready in %s\n", app.Name)
			fmt.Printf("       ✓ Select as microphone: %s\n", micDescription)
		}
	}

	return nil
}

// Reset removes all modules associated with a profile
func (m *Manager) Reset(p *Profile) error {
	micSinkName := p.VirtualSink + "-mic"

	// 1. Unload loopback modules that feed audio into the mic sink
	// Find loopback modules matching both the monitor source and mic sink to avoid unloading unrelated modules
	monitorSource, _ := m.client.GetSinkMonitor(p.VirtualSink)
	loopArgs := []string{"source=" + monitorSource, "sink=" + micSinkName}
	loopbackModIDs, err := m.client.FindModulesMatching(loopArgs)
	if err != nil {
		fmt.Printf("     - No loopback modules found for %s -> %s\n", monitorSource, micSinkName)
	} else {
		fmt.Printf("     - Unloading %d loopback module(s)\n", len(loopbackModIDs))
		for _, modID := range loopbackModIDs {
			if err := m.client.UnloadModule(modID); err != nil {
				fmt.Printf("       Warning: failed to unload loopback module %d: %v\n", modID, err)
			} else {
				fmt.Printf("       Unloaded loopback module %d\n", modID)
			}
		}
	}

	// 2. Unload the mic sink modules
	nullSinkArg := "sink_name=" + micSinkName
	nullSinkModIDs, err := m.client.FindModules(nullSinkArg)
	if err != nil {
		fmt.Printf("     - No mic sink modules found\n")
	} else {
		fmt.Printf("     - Unloading %d mic sink module(s)\n", len(nullSinkModIDs))
		for _, modID := range nullSinkModIDs {
			if err := m.client.UnloadModule(modID); err != nil {
				fmt.Printf("       Warning: failed to unload mic sink module %d: %v\n", modID, err)
			} else {
				fmt.Printf("       Unloaded mic sink module %d\n", modID)
			}
		}
	}

	// 3. Unload main sink modules
	mainModIDs, err := m.client.FindModules("sink_name=" + p.VirtualSink)
	if err != nil {
		fmt.Printf("     - No main sink modules found\n")
	} else {
		fmt.Printf("     - Unloading %d main sink module(s)\n", len(mainModIDs))
		for _, modID := range mainModIDs {
			if err := m.client.UnloadModule(modID); err != nil {
				fmt.Printf("       Warning: failed to unload main sink module %d: %v\n", modID, err)
			} else {
				fmt.Printf("       Unloaded main sink module %d\n", modID)
			}
		}
	}

	return nil
}
