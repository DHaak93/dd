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
| `dd-door/<id>/state` | out | `open`, `closed`, `opening`, `closing` or `stopped` |
| `dd-door/<id>/position` | out | Current position `0`-`100` |
| `dd-door/<id>/availability` | out (retained) | `online` / `offline` |
| `dd-door/<id>/activity` | out | Text of every new entry in the door's cloud activity log, e.g. `Opening by Remote control`, `Daniel sent Close` |
| `dd-door/<id>/alert` | out | Text of safety/fault log entries only, e.g. `Close Prevented as beam is blocked`, `Reversing`, `Open after Obstruction` |
| `dd-door/<id>/command` | in | `go_open`, `go_close` or `STOP` |
| `dd-door/<id>/set_position` | in | Target position `0`-`100` |

### Opening / closing detection

The SmartDoor cloud only updates a door's position once it has finished travelling, so on its own Home Assistant would not see a door move until it stopped. The bridge therefore also watches:

1. **`device.rate`** (when the cloud sends it): positive while opening, negative while closing, `0` when stopped. `opening`/`closing` is published as soon as the door moves, and `stopped` as soon as it halts part-way.
2. **The door's activity log**: a new entry such as `Opening by Remote control` or `Daniel sent Close` publishes `opening`/`closing` straight away. This also covers remotes and wall buttons, not just commands from Home Assistant.
3. **Safety events**: an entry with the cloud's alert flag set, or that mentions the beam/obstruction/reversing, is published on the `alert` topic. If a safety device reverses the door (`Reversing`, `Opening by Safety Beam`), the state changes to `opening`.

Once the door reaches an end position the real `open`/`closed` state is published again. If no end position is reached within 45 seconds the door is reported as `stopped`. The current state is also re-published with every status update, so Home Assistant catches up after a restart.

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