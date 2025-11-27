#!/bin/bash

# A debug script to inspect the audio routing setup for gemini-audio.

echo "================================================="
echo "        gemini-audio Debugging Script"
echo "================================================="

# --- SINKS ---
echo ""
echo "--- SINKS (Output Devices) ---"
echo "You should see two virtual sinks created by the tool:"
echo "1. 'virtual-out-to-chrome': Where Firefox's audio is sent."
echo "2. 'virtual-out-to-chrome-mic': The input side of the virtual microphone."
echo ""
pactl list short sinks | grep 'virtual-out-to-chrome' --color=always
if ! pactl list short sinks | grep -q 'virtual-out-to-chrome'; then
    echo "-> WARNING: Virtual sinks not found. Did you run 'go run ./main.go setup'?"
fi


# --- SOURCES ---
echo ""
echo ""
echo "--- SOURCES (Input Devices) ---"
echo "You should see monitor sources for the sinks above."
echo "The one you want to select in Chrome is 'virtual-out-to-chrome.monitor'."
echo ""
pactl list short sources | grep 'virtual-out-to-chrome' --color=always


# --- MODULES ---
echo ""
echo ""
echo "--- LOADED MODULES ---"
echo "You should see three modules loaded by the tool:"
echo "1. A 'module-null-sink' for 'virtual-out-to-chrome'."
echo "2. A 'module-null-sink' for 'virtual-out-to-chrome-mic'."
echo "3. A 'module-loopback' connecting the two."
echo ""
pactl list short modules | grep 'virtual-out-to-chrome' --color=always


# --- FINAL ADVICE ---
echo ""
echo ""
echo "================================================="
echo "                 HOW TO USE"
echo "================================================="
echo "Based on your feedback, the correct microphone to select in"
echo "Chrome/Meet is:"
echo ""
echo "    Monitor of virtual-out-to-chrome"
echo ""
echo "The device named 'Virtual Mic (firefox-to-chrome)' is a fallback and"
echo "might not be necessary on your system."
echo "================================================="
