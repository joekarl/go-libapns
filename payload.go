package apns

import (
	"encoding/json"
	"errors"
	"fmt"
)

//Object describing a push notification payload
type Payload struct {
	// Basic alert structure
	AlertText        string
	Badge            BadgeNumber
	Sound            string
	ContentAvailable int
	Category         string

	// If this is an enhanced message, use
	// an APSAlertBody instead of .Alert
	AlertBody APSAlertBody

	// Any custom fields to be added to the apns payload
	// These exist outside of the `aps` namespace
	CustomFields map[string]interface{}

	// Payload server fields
	// UNIX time in seconds when the payload is invalid
	ExpirationTime uint32
	// Must be either 5 or 10, if not one of these two values will default to 5
	Priority uint8

	// Device push token, should contain no spaces
	Token string

	// Any extra data to be associated with this payload,
	// Will not be sent to apple but will be held onto for error cases
	ExtraData interface{}
}

type APSAlertBody struct {
	// Text of the alert
	Body string `json:"body,omitempty"`

	// Other alert options
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`

	// New Title fields and localizations. >= iOS 8.2
	Title        string   `json:"title,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
}

type alertBodyAps struct {
	Alert            APSAlertBody
	Badge            BadgeNumber
	Sound            string
	Category         string
	ContentAvailable int
}

type simpleAps struct {
	Alert            string
	Badge            BadgeNumber
	Sound            string
	Category         string
	ContentAvailable int
}

// Convert a Payload into a json object and then converted to a byte array
// If the number of converted bytes is greater than the maxPayloadSize
// an attempt will be made to truncate the AlertText
// If this cannot be done, then an error will be returned
func (p *Payload) Marshal(maxPayloadSize int) ([]byte, error) {
	if p.isSimple() {
		return p.marshalSimplePayload(maxPayloadSize)
	} else {
		return p.marshalAlertBodyPayload(maxPayloadSize)
	}
}

//Whether or not to use simple aps format or not
func (p *Payload) isSimple() bool {
	return p.AlertBody.Body == ""
}

//Helper method to generate a json compatible map with aps key + custom fields
//will return error if custom field named aps supplied
func constructFullPayload(aps interface{}, customFields map[string]interface{}) (map[string]interface{}, error) {
	var fullPayload = make(map[string]interface{})
	fullPayload["aps"] = aps
	for key, value := range customFields {
		if key == "aps" {
			return nil, errors.New("Cannot have a custom field named aps")
		}
		fullPayload[key] = value
	}
	return fullPayload, nil
}

//Handle simple payload case with just text alert
//Handle truncating of alert text if too long for maxPayloadSize
func (p *Payload) marshalSimplePayload(maxPayloadSize int) ([]byte, error) {
	var jsonStr []byte

	//use simple payload
	aps := simpleAps{
		Alert:            p.AlertText,
		Badge:            p.Badge,
		Sound:            p.Sound,
		Category:         p.Category,
		ContentAvailable: p.ContentAvailable,
	}

	fullPayload, err := constructFullPayload(aps, p.CustomFields)
	if err != nil {
		return nil, err
	}

	jsonStr, err = json.Marshal(fullPayload)
	if err != nil {
		return nil, err
	}

	payloadLen := len(jsonStr)

	if payloadLen > maxPayloadSize {
		clipSize := payloadLen - (maxPayloadSize) + 3 //need extra characters for ellipse
		if clipSize > len(p.AlertText) {
			return nil, errors.New(fmt.Sprintf("Payload was too long to successfully marshall to less than %v", maxPayloadSize))
		}
		aps.Alert = aps.Alert[:len(aps.Alert)-clipSize] + "..."
		fullPayload["aps"] = aps
		if err != nil {
			return nil, err
		}

		jsonStr, err = json.Marshal(fullPayload)
		if err != nil {
			return nil, err
		}
	}

	return jsonStr, nil
}

//Handle complet payload case with alert object
//Handle truncating of alert text if too long for maxPayloadSize
func (p *Payload) marshalAlertBodyPayload(maxPayloadSize int) ([]byte, error) {
	var jsonStr []byte

	// Use APSAlertBody payload
	aps := alertBodyAps{
		Alert:            p.AlertBody,
		Badge:            p.Badge,
		Sound:            p.Sound,
		Category:         p.Category,
		ContentAvailable: p.ContentAvailable,
	}

	fullPayload, err := constructFullPayload(aps, p.CustomFields)
	if err != nil {
		return nil, err
	}

	jsonStr, err = json.Marshal(fullPayload)
	if err != nil {
		return nil, err
	}

	payloadLen := len(jsonStr)

	if payloadLen > maxPayloadSize {
		clipSize := payloadLen - (maxPayloadSize) + 3 //need extra characters for ellipse
		if clipSize > len(p.AlertBody.Body) {
			return nil, errors.New(fmt.Sprintf("Payload was too long to successfully marshall %v or less bytes", maxPayloadSize))
		}
		aps.Alert.Body = aps.Alert.Body[:len(aps.Alert.Body)-clipSize] + "..."
		fullPayload["aps"] = aps
		if err != nil {
			return nil, err
		}

		jsonStr, err = json.Marshal(fullPayload)
		if err != nil {
			return nil, err
		}
	}

	return jsonStr, nil
}

func (s simpleAps) MarshalJSON() ([]byte, error) {
	toMarshal := make(map[string]interface{})

	if s.Alert != "" {
		toMarshal["alert"] = s.Alert
	}
	if s.Badge.IsSet() {
		toMarshal["badge"] = s.Badge
	}
	if s.Sound != "" {
		toMarshal["sound"] = s.Sound
	}
	if s.Category != "" {
		toMarshal["category"] = s.Category
	}
	if s.ContentAvailable != 0 {
		toMarshal["content-available"] = s.ContentAvailable
	}

	return json.Marshal(toMarshal)
}

func (a alertBodyAps) MarshalJSON() ([]byte, error) {
	toMarshal := make(map[string]interface{})
	toMarshal["alert"] = a.Alert

	if a.Badge.IsSet() {
		toMarshal["badge"] = a.Badge
	}
	if a.Sound != "" {
		toMarshal["sound"] = a.Sound
	}
	if a.Category != "" {
		toMarshal["category"] = a.Category
	}
	if a.ContentAvailable != 0 {
		toMarshal["content-available"] = a.ContentAvailable
	}

	return json.Marshal(toMarshal)
}
