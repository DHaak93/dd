# SmartDoor MQTT Bridge Integration

This Home Assistant add-on bridges local SmartDoor garage door control systems with Home Assistant via MQTT, including support for precise position control, status monitoring, and prometheus metrics.

**Compatible systems:** the add-on works with anything on the SmartDoor cloud ecosystem, including the **ATA Smart Phone Hub** and **B&D Smart Door** garage door systems, which use the same ecosystem.

---

## How to Use

### 1. Installation

1. Add this repository URL to your Home Assistant Add-on Store: `https://github.com/DHaak93/dd`
2. Search for **SmartDoor MQTT Bridge** and click **Install**.

### 2. Configuration & One-Time Registration

To talk to the SmartDoor device, the add-on needs cryptographic credentials which are acquired during a **one-time registration** process with the SmartDoor cloud servers.

1. Generate a **Registration Share Code** from your official SmartDoor mobile app.
2. In the add-on **Configuration** tab, enter:
   - **Registration Share Code**: The code you just generated.
   - **User Password**: Your SmartDoor account password.
   - **SmartDoor Device IP/Host**: The local IP address of your SmartDoor garage door (e.g., `192.168.1.81`).
3. Click **Save** and start the add-on.
4. Check the **Log** tab. You should see a message:
   `Registration successful. Credentials saved to /config/dd-credentials.json.`

> [!TIP]
> After registration is successful, the generated credentials are secure and saved locally. You can safely **delete the share code and password** from your add-on configuration tab if you wish.

---

## Configuration Settings

| Setting | Type | Description |
| :--- | :--- | :--- |
| **Registration Share Code** | `string` | One-time share code from your mobile app (only needed for first run). |
| **User Password** | `password` | Your account password (only needed for first run). |
| **SmartDoor Device IP/Host** | `string` | The local IP address of the SmartDoor hardware unit. |
| **MQTT Topic Prefix** | `string` | Topic prefix used for Home Assistant discovery and status (default: `dd-door`). |
| **Status Poll Interval (seconds)** | `integer` | How often to poll the physical door for status changes (default: `60` seconds). |
| **Enable Debug Logging** | `boolean` | Set to `true` to print verbose API and cryptographic messages. Passwords and session secrets are redacted, but the log still contains device and session IDs, so don't post it publicly unedited. |

---

## MQTT Topics & Discovery

This add-on supports Home Assistant MQTT Discovery automatically. Once started, a new **Cover** entity will be registered in Home Assistant.

If you are using custom MQTT scripts or automations, the following topics are exposed per door (`<id>` is the SmartDoor device ID, shown in the add-on log; default prefix `dd-door`):

| Topic | Direction | Payload |
| :--- | :--- | :--- |
| `homeassistant/cover/<id>/config` | out (retained) | Discovery config for the cover entity |
| `dd-door/<id>/state` | out | `open`, `closed`, `opening` or `closing` (a door stopped part-way is `open`) |
| `dd-door/<id>/position` | out | Current position `0`-`100` |
| `dd-door/<id>/availability` | out (retained) | `online` / `offline` |
| `dd-door/<id>/activity` | out | Text of every new entry in the door's cloud activity log, e.g. `Opening by Remote control`, `Daniel sent Close` |
| `dd-door/<id>/alert` | out | Text of safety/fault log entries only, e.g. `Close Prevented as beam is blocked`, `Reversing`, `Open after Obstruction` |
| `dd-door/<id>/command` | in | `go_open`, `go_close`, `STOP`, or `part_open_1`/`part_open_2`/`part_open_3` (the door's partial-open presets) |
| `dd-door/<id>/set_position` | in | Target position `0`-`100`, for scripts only (no slider in Home Assistant). Not every door accepts in-between positions; see below |
| `homeassistant/button/<id>_part_open_N/config` | out (retained) | Discovery config for the partial-open buttons |

### Opening / closing detection

The SmartDoor cloud only updates a door's position once it has finished travelling, so on its own Home Assistant would not see a door move until it stopped. The bridge therefore also watches:

1. **`device.rate`** (when the cloud sends it): positive while opening, negative while closing, `0` when stopped. `opening`/`closing` is published as soon as the door moves, and `open` (with the position it stopped at) as soon as it halts part-way. The app reports `0` while a move it started is travelling, so a new log entry about opening/closing takes priority over a `0` rate.
2. **The door's activity log**: a new entry such as `Opening by Remote control` or `Daniel sent Close` publishes `opening`/`closing` straight away. This also covers remotes and wall buttons, not just commands from Home Assistant.
3. **Safety events**: an entry with the cloud's alert flag set, or that mentions the beam/obstruction/reversing, is published on the `alert` topic. If a safety device reverses the door (`Reversing`, `Opening by Safety Beam`), the state changes to `opening`.

Once the door reaches an end position the real `open`/`closed` state is published again. If no end position is reached within 45 seconds the door is reported as stopped part-way, i.e. `open` at its position. Home Assistant's MQTT cover has no stopped state of its own: on `stopped` it guesses open or closed from the last position it had, which can be wrong, so the bridge sends `open` instead. The current state is also re-published with every status update, so Home Assistant catches up after a restart.

The activity log does not say which remote was used.

### Example automation

```yaml
automation:
  - alias: Garage safety alert
    trigger:
      - platform: mqtt
        topic: dd-door/+/alert
    action:
      - service: notify.notify
        data:
          message: "Garage door: {{ trigger.payload }}"
```

### Partial open (Pet, Parcel, Ventilation)

Each door gets up to three buttons in Home Assistant, on the same device as the cover, for the partial-open presets set up in the SmartDoor/ATA app (Settings → Partial open control 1–3). They are named as in the app when the door reports the names; otherwise they are **Partial open 1–3**, in the app's order (by default Pet, Parcel, Ventilation). You can rename them in Home Assistant. Pressing one sends the door to the height saved for that preset; Home Assistant shows it opening, then open at the position it stopped.

The doors (e.g. the GDO6V4) reject in-between positions with "Action rejected: Not Supported", so the cover shows its position but has no position slider. Use the partial-open buttons instead. To put them on the door's card, use a card like this:

```yaml
type: vertical-stack
cards:
  - type: tile
    entity: cover.garage_front_roller_door
    features:
      - type: cover-open-close
  - type: grid
    columns: 3
    square: false
    cards:
      - type: button
        entity: button.garage_front_roller_door_partial_open_1
        name: Pet
        tap_action: {action: perform-action, perform_action: button.press, target: {entity_id: button.garage_front_roller_door_partial_open_1}}
      - type: button
        entity: button.garage_front_roller_door_partial_open_2
        name: Parcel
        tap_action: {action: perform-action, perform_action: button.press, target: {entity_id: button.garage_front_roller_door_partial_open_2}}
      - type: button
        entity: button.garage_front_roller_door_partial_open_3
        name: Ventilation
        tap_action: {action: perform-action, perform_action: button.press, target: {entity_id: button.garage_front_roller_door_partial_open_3}}
```

Replace the entity IDs with your own (Settings → Devices → the door).
