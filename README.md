Ubuntu 24.04 (PipeWire) compatibility notes for PubTalk
=====================================================

This project (pubtalk) uses PulseAudio-compatible `pactl` commands to create virtual sinks
and loopbacks so that one application's output can be presented as another application's
input (virtual microphone). On Ubuntu 24.04 the audio server is PipeWire with a
PulseAudio compatibility layer. That changes available modules and behavior compared to
older PulseAudio-only setups.

Key points
----------
- PipeWire does not provide `module-null-source` in many installs. Instead, the reliable
  approach is to create a `module-null-sink` and use its `.monitor` source as the
  recording device seen by apps.
- We create exactly two objects per profile:
  - `virtual-out-<profile>`: a null sink which receives playback streams (virtual output)
  - `virtual-out-<profile>-mic`: another null sink that serves as the "virtual mic" sink
    — a loopback feeds the first sink's monitor into this mic sink, and apps choose the
    mic sink's `.monitor` as their recording source.
- This avoids creating ambiguous remapped sources and keeps devices visible and
  selectable in GUIs like pavucontrol.

How to use
----------
1. Start the daemon:

```bash
cd /path/to/pubtalk
go run main.go
```

2. Open PulseAudio Volume Control (`pavucontrol`) and:
   - In the Playback tab: ensure the app's output (e.g. Chrome/Firefox) is routed to the
     corresponding `virtual-out-<profile>` sink (the daemon attempts to move it automatically).
   - In the Recording tab: choose the `virtual-out-<profile>-mic.monitor` source as the
     microphone for the application that should receive the audio.

3. Verify via CLI:

```bash
pactl list short sinks    # shows virtual-out-<profile> and virtual-out-<profile>-mic
pactl list short sources  # shows virtual-out-<profile>-mic.monitor entries
pactl list short modules  # loopback modules appear with 'source=<monitor>' and 'sink=<micSinkName>'
```

Cleanup
-------
The daemon will attempt to unload only the modules it created on shutdown. If you need
to clean up manually, be careful to avoid unloading unrelated modules. Inspect module
arguments first:

```bash
pactl list short modules
```

Notes for contributors
----------------------
- Module matching is done by substring matching against the module arguments. Exact
  matching against multiple substrings is used to avoid accidental unloads.
- If you change naming conventions for sinks/sources, update `internal/profiles/manager.go`
  and the module-matching helper in `internal/pactl/pactl.go`.
