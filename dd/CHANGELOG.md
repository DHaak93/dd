# Changelog

All notable changes to this add-on will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.3.5-dh9] - DHaak93 fork

Reliability fixes ported from upstream 0.3.6 (gravypower/dd).

### Added
- The bridge reconnects to the SmartDoor cloud with backoff (5–60 s) when
  the session is lost, instead of stopping. The doors show as unavailable in
  Home Assistant meanwhile and catch up straight after reconnecting.
- `<prefix>/bridge/availability` (retained, with an MQTT last will). The
  doors and their buttons are only available while both the door and the
  bridge are online, so they show as unavailable if the add-on dies.

### Fixed
- Every polled message was processed twice, so each status update was
  handled twice.
- A reply to a command could be lost if a status poll picked it up first;
  the bridge now registers for the reply before sending.
- An empty or truncated encrypted message no longer crashes the bridge.
- Cloud requests time out after 30 s instead of hanging.
- A failed registration no longer prints the password and secrets.

## [0.3.5-dh8] - DHaak93 fork

### Changed
- The cover no longer has a position slider in Home Assistant. Its position
  is still shown, but the doors reject in-between positions, so the slider
  only ever worked for fully open or closed. Use the open/close buttons and
  the partial-open buttons instead. `set_position` still works over MQTT for
  scripts. DOCS.md has an example card with the partial-open buttons under
  the cover.

## [0.3.5-dh7] - DHaak93 fork

### Added
- Home Assistant buttons for the door's partial-open presets, named as set
  up in the SmartDoor/ATA app (e.g. **Pet**, **Parcel**, **Ventilation**).
  They appear on the same device as the cover. The door goes to the height
  saved for each preset in the app. They can also be sent as
  `part_open_1`, `part_open_2` or `part_open_3` on the command topic.

### Changed
- Some doors (e.g. the GDO6V4) reject the 5–95 % position commands used by
  the position slider ("Action rejected: Not Supported"). The log now says
  to use the partial-open buttons instead. Fully open/closed from the
  slider still works.

## [0.3.5-dh6] - DHaak93 fork

Fixes from testing 0.3.5-dh5 on a real door.

### Fixed
- A door stopped part-way could show as **closed** in Home Assistant (seen:
  "Closed" at 48%). On a `stopped` state, HA's MQTT cover picks open or
  closed from the last position it received, which was still 0. A door
  halted part-way is now published as `open`, and the position is published
  before the state.
- A move started from the SmartDoor app reports `rate` 0 while the door
  travels. A new "… sent Open/Close" log entry now wins over rate 0, so the
  move is not mistaken for a stop.

## [0.3.5-dh5] - DHaak93 fork

### Changed
- Home Assistant now pulls the prebuilt image
  `ghcr.io/dhaak93/{arch}-addon-dd` (published by the Release workflow)
  instead of compiling the add-on locally. No code changes.

## [0.3.5-dh4] - DHaak93 fork

Everything under "Unreleased" below, plus fork packaging:

### Changed
- `build.yaml` removed (deprecated); the base image now defaults in the
  Dockerfile to the multi-arch `ghcr.io/home-assistant/base:3.22` (same
  Alpine release as before). The unused tempio download is gone.
- `map: config` replaced with `homeassistant_config`, still mounted at
  `/config` so the existing credentials file is found.
- Only `aarch64` and `amd64` are listed; Home Assistant has deprecated
  `armhf`, `armv7` and `i386`.
- Real maintainer in `repository.yaml`.
- GitHub Actions: the inherited Builder workflow (retired builder action,
  image publishing) is replaced by a Go CI workflow; the add-on lint no
  longer runs on a daily schedule.
- New Release workflow: tags `v<version>` and publishes per-arch images to
  ghcr.io when the version changes. This version still builds locally.

## [Unreleased]

### Added
- Live `opening`/`closing` state as soon as a door starts moving, including
  from remotes and wall buttons (from `device.rate`, falling back to the
  cloud activity log).
- `<prefix>/<id>/activity` topic with the text of each new cloud activity
  log entry.
- `<prefix>/<id>/alert` topic for safety events (beam blocked, obstruction,
  reversing); a reversal shows the door as `opening`.

### Fixed
- `STOP` from Home Assistant left the cover on `stopping`, which Home
  Assistant doesn't support; it now reports `stopped`.
- A command from Home Assistant that the door didn't complete (e.g. close
  prevented by the beam) left the cover on `opening`/`closing` until the door
  next moved.
- State is re-published with every status update, so Home Assistant no longer
  shows an unknown state after it restarts.
- One undecodable cloud message no longer stops the bridge.
- Debug logs no longer contain passwords or the session secret.
- Removed `expire_after` and `scan_interval` from the discovery config.

## [0.3.3] - 2025-12-02

### Fixed
- Removed incorrect watchdog configuration that was causing add-on to restart every ~4 minutes
- Watchdog was checking MQTT broker TCP port instead of add-on health (add-on is a client, not a server)

## [0.3.2] - 2025-12-02

### Fixed
- Removed MQTT section from schema validation to fix "Missing option 'mqtt?'" error
- MQTT configuration is now truly optional - add manually only if using custom broker
- Schema validation no longer blocks add-on installation when MQTT section is omitted

## [0.3.1] - 2025-12-02

### Changed
- Made MQTT configuration completely optional - now defaults to Home Assistant MQTT integration
- Removed requirement to specify MQTT broker settings in configuration
- MQTT service availability check only runs when using HA MQTT (not custom brokers)

### Fixed
- AppArmor profile name now matches add-on slug (was causing loading errors)
- Removed nested AppArmor profile structure
- Simplified configuration schema for better user experience

## [0.3.0] - 2025-12-02

### Added
- P **Position Control**: Full slider support for setting door position (0-100%)
  - New MQTT topics: `position` and `set_position`
  - Real-time position reporting with 5% granularity
  - Support for common presets (Pet Mode 20%, Delivery Mode 68%, Ventilation 5-10%)
- MQTT prefix configuration option (`mqtt_prefix`)
- Custom MQTT broker support (optional, defaults to HA MQTT service)
- Watchdog monitoring on MQTT TCP port
- Comprehensive unit tests for position mapping
- Complete architecture documentation

### Changed
- Updated add-on name to "SmartDoor MQTT Bridge"
- Improved add-on description
- Enhanced MQTT configuration with nested schema
- Set `startup` to `application` and `boot` to `auto` per HA best practices
- Updated AppArmor profile with correct service names and network permissions
- Improved error handling throughout codebase

### Fixed
- Critical race condition in MQTT command handler
- Aggressive Fatal() calls replaced with graceful error handling
- Thread-safe device FSM map access with RWMutex
- Better error context in crypto operations
- Documentation gaps filled with comprehensive guides

### Documentation
- Added `POSITION_CONTROL.md` with usage examples
- Added `IMPROVEMENTS.md` summarizing all enhancements
- Updated `README.md` with complete architecture documentation
- Created `ICON_README.md` for visual assets guidance

## [0.2.0] - Previous Release

### Added
- Initial MQTT integration
- Basic FSM state management
- Multi-architecture support

### Fixed
- MQTT connection resilience improvements
- Device state tracking

## [0.1.9] - Initial Release

### Added
- SmartDoor device integration
- Home Assistant MQTT discovery
- Basic door control (open/close/stop)
