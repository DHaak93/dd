package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// HomeAssistantButtonConfigTopicTemplate is the discovery topic for a
// partial-open button: homeassistant/button/<deviceID>_<key>/config.
const HomeAssistantButtonConfigTopicTemplate = "homeassistant/button/%s_%s/config"

// PartOpenButton is one of the door's partial-open presets (set up in the
// SmartDoor/ATA app, e.g. Pet, Parcel, Ventilation). The door goes to the
// height saved for the preset; the bridge doesn't know what that height is.
type PartOpenButton struct {
	Key     string // command payload, e.g. "part_open_1"
	Command int    // device command, 5-7
	Title   string // name shown in Home Assistant
}

var partOpenPresets = []PartOpenButton{
	{"part_open_1", AvailableCommands.PartOpen1, "Partial open 1"},
	{"part_open_2", AvailableCommands.PartOpen2, "Partial open 2"},
	{"part_open_3", AvailableCommands.PartOpen3, "Partial open 3"},
}

// PartOpenCommand returns the device command for a partial-open payload
// ("part_open_1" to "part_open_3", any case).
func PartOpenCommand(payload string) (int, bool) {
	key := strings.ToLower(strings.TrimSpace(payload))
	for _, p := range partOpenPresets {
		if p.Key == key {
			return p.Command, true
		}
	}
	return 0, false
}

// PartOpenButtons returns the partial-open presets to offer in Home
// Assistant. When the door's own button list includes them, their titles
// (e.g. "Pet") are used and hidden ones are left out. Otherwise all three
// are offered with generic names.
func PartOpenButtons(buttons []DoorStatusButton) []PartOpenButton {
	var out []PartOpenButton
	for _, p := range partOpenPresets {
		for _, b := range buttons {
			if b.Action.Command != p.Command {
				continue
			}
			if b.Hide == 0 {
				if t := strings.TrimSpace(b.Title); t != "" {
					p.Title = t
				}
				out = append(out, p)
			}
			break
		}
	}
	if len(out) == 0 {
		return partOpenPresets
	}
	return out
}

// publishPartOpenButtons publishes Home Assistant button entities for the
// door's partial-open presets. They belong to the same device as the cover.
func publishPartOpenButtons(handler *MQTTHandler, mqttPrefix string, device DoorStatusDevice, basicInfo BasicInfo) {
	var listed []string
	for _, b := range device.Buttons {
		listed = append(listed, fmt.Sprintf("%s=%d", b.Title, b.Action.Command))
	}
	logger.WithFields(logrus.Fields{"deviceID": device.ID, "buttons": strings.Join(listed, ", ")}).Info("Door buttons")

	for _, p := range PartOpenButtons(device.Buttons) {
		payload := map[string]interface{}{
			"name":          p.Title,
			"command_topic": fmt.Sprintf(CommandTopicTemplate, mqttPrefix, device.ID),
			"payload_press": p.Key,
			"unique_id":     fmt.Sprintf("button_%s_%s", device.ID, p.Key),
			"icon":          "mdi:garage-open-variant",
			"device": map[string]interface{}{
				"identifiers":  []string{fmt.Sprintf("garage_door_%s", device.ID)},
				"name":         basicInfo.Name,
				"manufacturer": "dd",
			},
		}
		for k, v := range availabilityConfig(mqttPrefix, device.ID) {
			payload[k] = v
		}
		bytes, err := json.Marshal(payload)
		if err != nil {
			logger.WithError(err).Error("Couldn't encode button config")
			continue
		}
		topic := fmt.Sprintf(HomeAssistantButtonConfigTopicTemplate, device.ID, p.Key)
		if err := handler.publishToMQTT(topic, 0, true, bytes); err != nil {
			logger.WithError(err).WithField("topic", topic).Warn("Couldn't publish button config")
		}
	}
}
