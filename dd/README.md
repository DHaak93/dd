# Home Assistant Add-on: SmartDoor MQTT Bridge

[![Supports aarch64 Architecture][aarch64-shield]][addon-store]
[![Supports amd64 Architecture][amd64-shield]][addon-store]

A secure and resilient integration that bridges **SmartDoor** smart garage door systems with Home Assistant via MQTT. Works with the **ATA Smart Phone Hub** and **B&D Smart Door** systems too, which use the same SmartDoor ecosystem.

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

Every change needs a version bump in `config.yaml`.

When the version changes on `master`, the **Release** GitHub Actions workflow
tags the commit `v<version>` and publishes
`ghcr.io/dhaak93/{amd64,aarch64}-addon-dd:<version>`, built from that tag.

Without `image:` in `config.yaml`, Home Assistant builds the add-on on your
machine instead: the Dockerfile clones `DHaak93/dd` `master` (whatever it is
at that moment) and compiles the Go binaries. With `image:` set, Home
Assistant just pulls the published image, so wait for the Release workflow
to finish before installing an update.

[addon-store]: https://github.com/DHaak93/dd
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
