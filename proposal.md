# Proposal: Go-based Audio Routing Utility for Ubuntu

## Abstract
This document proposes a concise, maintainable, native Go command-line utility to manage application-to-application audio routing on Ubuntu (PulseAudio or PipeWire with PulseAudio compatibility). The tool will replace fragile shell-script workflows with a safe, configurable, and diagnosable approach for scenarios such as piping Firefox audio into Chrome's microphone (e.g., for Twitter Spaces) and vice-versa.

## Background and Motivation
Existing routing is done with ad-hoc shell scripts (`AudioReset.sh`, `FinalVerWIP.sh`) that work but are brittle: they lack structured configuration, robust error handling, idempotency, and clear diagnostics. System updates, different audio servers, or manual mistakes can break routing and be time-consuming to recover.

## Problem Statement
Users need reliable, repeatable ways to route audio between specific application outputs and application inputs (virtual microphones). Two primary, representative scenarios are:
- Firefox output -> Chrome input (for broadcasting external audio into a Chrome-based web app)
- Chrome output -> Firefox input (for recording or rebroadcasting a meeting into Firefox)

The tool must avoid feedback loops, be resilient across re-runs, and provide clear status and recovery actions.

## Goals
- Provide a robust CLI utility to create and manage virtual sinks and monitor sources.
- Make routing idempotent and safe to run repeatedly.
- Expose an easy configuration/profile mechanism for common routing setups.
- Offer readable status, diagnostics, and a clean reset/cleanup command.
- Keep implementation portable across PulseAudio and PipeWire with the Pulse compatibility layer.

## Scope and Non-Goals
- In scope: creating/removing `null-sink` modules, moving sink-inputs/source-outputs, mapping monitor sources to application microphones, profile management, logging.
- Out of scope: deep integration into browser settings (the tool will attempt to set browser input where possible, otherwise provide clear instructions), building a GUI (CLI only for v1).

## Proposed Solution
Build a Go-based CLI that wraps `pactl` operations (initially via `os/exec`) and provides higher-level primitives:
- Virtual device lifecycle: create `null-sink` modules and discover monitor sources.
- Stream discovery and routing: find sink-inputs and source-outputs for applications (by name, PID, or stream properties) and move them to virtual devices.
- Profiles/config: YAML-based profiles for common scenarios (example below).
- Commands: `setup`, `status`, `reset`, `list-profiles`, `apply-profile`, and `watch` mode for reactive handling.

## Technical Approach
- Language: Go
- Initial interface to audio system: `pactl` called via `os/exec` (simple and widely available). Keep abstraction so a future transition to native bindings is straightforward.
- CLI framework: `cobra` (recommended) or `urfave/cli`.
- Config format: YAML (human-editable) with profile examples.
- Concurrency: use Go routines for non-blocking status/watch features.

## Example CLI and Profile
Example commands:
```
gemini-audio status
gemini-audio setup firefox-to-chrome
gemini-audio reset
```

Example YAML profile (`profiles/firefox-to-chrome.yaml`):
```
name: firefox-to-chrome
description: Route Firefox output into Chrome microphone
virtual_sink: virtual-out-to-chrome
applications:
    - name: firefox
        role: playback
    - name: chrome
        role: input_target
```

## Safety and Error Handling
- Detect existing virtual devices and reuse them where appropriate.
- Prevent simple feedback loops by validating that a virtual sink monitor is not simultaneously routed back into its own sink.
- Provide verbose `--dry-run` and `--force` options for cautious operation.

## Testing and Validation
- Unit tests for config parsing and `pactl` command formation (mock `os/exec`).
- Integration tests that run on a real Ubuntu dev machine with PulseAudio/PipeWire present (documented test cases).

## Roadmap / Milestones
1. CLI skeleton, `pactl` wrapper functions, basic `status` command (week 1)
2. Virtual sink create/remove and simple routing commands (week 2)
3. Profile system, `apply-profile`, and `reset` (week 3)
4. Watch mode, diagnostics, and tests (week 4)

## Deliverables
- `gemini-audio` (binary + source)
- `profiles/` with example YAML profiles
- README with usage and troubleshooting
- Unit and integration tests

## Next Steps
- Confirm the proposal scope and priorities.
- Decide whether to start with `pactl`-based execution (recommended) or native bindings.
- If confirmed, scaffold the Go repo with `cobra`, add `pactl` wrappers, and implement the `status`/`setup`/`reset` commands.

---
If you want, I can now scaffold the Go project (module, CLI skeleton, and a `pactl` wrapper) and create one example profile. Which next step would you like me to take?
