# Home Assistant Add-on: SmartDoor MQTT Bridge

[![Supports aarch64 Architecture][aarch64-shield]][addon-store]
[![Supports amd64 Architecture][amd64-shield]][addon-store]

A secure and resilient integration that bridges **SmartDoor** smart garage door systems with Home Assistant via MQTT.

## Features

- **MQTT Discovery**: Automatic entity registration under Home Assistant (Cover entity).
- **Position Control**: Full support for opening/closing to precise percentages (5% - 95%) or standard Open/Close/Stop commands.
- **Live Opening/Closing**: Reports `opening`/`closing` as soon as a door starts moving — including from remotes and wall buttons — instead of only after it finishes travelling.
- **Activity & Safety Alerts**: Publishes every cloud activity log entry on `<prefix>/<id>/activity` and safety events (beam blocked, obstruction, reversing) on `<prefix>/<id>/alert` for use in automations.
- **At-Rest Encrypted Credentials**: Auto-migrates and encrypts your login credentials locally under `/config/dd-credentials.json` via AES-256-GCM.
- **Metrics Integration**: Built-in optional Prometheus metrics endpoint.
- **Robust FSM & Reconnection**: Handles network dropouts gracefully and recovers without crashing.
- **AppArmor Protection**: Strict kernel-level security isolation profile enforced by default.

## Installation & Setup

For step-by-step setup and configuration guidance, please see the [Add-on Documentation](DOCS.md).

## Building

This fork has no prebuilt image: Home Assistant builds the add-on on your
machine, and the Dockerfile clones `DHaak93/dd` `master` to compile the Go
binaries. The build is only redone when the add-on version changes, so every
change needs a version bump in `config.yaml`.

[addon-store]: https://github.com/DHaak93/dd
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
