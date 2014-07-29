package apns

import (
    "encoding/json"
    "errors"
    "fmt"
)

type Payload struct {
    ActionLocKey         string                 
    AlertText            string                 //alert text, may be truncated if bigger than max payload size
    Badge                int              
    ContentAvailable     int
    CustomFields         map[string]interface{} //any custom fields to be added to the apns payload
    ExpirationTime       uint32                 //unix time in seconds when the payload is invalid
    LaunchImage          string
    LocArgs              []string
    LocKey               string
    Priority             uint8                  //must be either 5 or 10, if not one of these two values will default to 5
    Sound                string
    Token                string                 //push token, should contain no spaces
    ExtraData            interface{}            //any extra data to be associated with this payload, 
                                                //will not be sent to apple but will be held onto for error cases
}

type apsAlertBody struct {
    Body                 string       `json:"body,omitempty"`
    ActionLocKey         string       `json:"action-loc-key,omitempty"`
    LocKey               string       `json:"loc-key,omitempty"`
    LocArgs              []string     `json:"loc-args,omitempty"`
    LaunchImage          string       `json:"launch-image,omitempty"`
}

type alertBodyAps struct {
    Alert                apsAlertBody `json:"alert,omitempty"`
    Badge                int          `json:"badge,omitempty"`
    Sound                string       `json:"sound,omitempty"`
    ContentAvailable     int          `json:"content-available,omitempty"`
}

type simpleAps struct {
    Alert                string       `json:"alert,omitempty"`
    Badge                int          `json:"badge,omitempty"`
    Sound                string       `json:"sound,omitempty"`
    ContentAvailable     int          `json:"content-available,omitempty"`
}

//Convert a Payload into a json object and then converted to a byte array
func (p *Payload) Marshal(maxPayloadSize int) ([]byte, error) {
    // found in https://github.com/anachronistic/apns/blob/master/push_notification.go#L93
    // This deserves some explanation.
    //
    // Setting an exported field of type int to 0
    // triggers the omitempty behavior if you've set it.
    // Since the badge is optional, we should omit it if
    // it's not set. However, we want to include it if the
    // value is 0, so there's a hack in payload.go
    // that exploits the fact that Apple treats -1 for a
    // badge value as though it were 0 (i.e. it clears the
    // badge but doesn't stop the notification from going
    // through successfully.)
    //
    // Still a hack though :)
    if p.Badge == 0 {
        p.Badge = -1
    }
    if p.isSimple() {
        return p.marshalSimplePayload(maxPayloadSize)
    } else {
        return p.marshalAlertBodyPayload(maxPayloadSize)
    }
}

//Whether or not to use simple aps format or not
func (p *Payload) isSimple() bool {
    return p.ActionLocKey == "" && p.LocKey == "" &&
           p.LocArgs == nil && p.LaunchImage == ""
}

//Helper method to generate a json compatible map with aps key + custom fields
//will return error if custom field named aps supplied
func constructFullPayload(aps interface{}, customFields map[string]interface{}) (map[string]interface{}, error) {
    var fullPayload map[string]interface{} = make(map[string]interface{})
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
    var jsonStr []byte = nil

    //use simple payload
    aps := simpleAps{
        Alert: p.AlertText,
        Badge: p.Badge,
        Sound: p.Sound,
        ContentAvailable: p.ContentAvailable,
    }

    fullPayload, err := constructFullPayload(aps, p.CustomFields)
    if (err != nil) {
        return nil, err
    }

    jsonStr, err = json.Marshal(fullPayload)
    if (err != nil) {
        return nil, err
    }

    payloadLen := len(jsonStr)

    if payloadLen > maxPayloadSize {
        clipSize := payloadLen - (maxPayloadSize) + 3 //need extra characters for ellipse
        if clipSize > len(p.AlertText) {
            return nil, errors.New(fmt.Sprintf("Payload was too long to successfully marshall to less than %v", maxPayloadSize))
        }
        aps.Alert = aps.Alert[:len(aps.Alert) - clipSize] + "..."
        fullPayload["aps"] = aps
        if (err != nil) {
            return nil, err
        }

        jsonStr, err = json.Marshal(fullPayload)
        if (err != nil) {
            return nil, err
        }
    }

    return jsonStr, nil
}

//Handle complet payload case with alert object
//Handle truncating of alert text if too long for maxPayloadSize
func (p *Payload) marshalAlertBodyPayload(maxPayloadSize int) ([]byte, error) {
    var jsonStr []byte = nil

    //use alertBody payload
    alertBody := apsAlertBody{
        Body: p.AlertText,
        ActionLocKey: p.ActionLocKey,
        LocKey: p.LocKey,
        LocArgs: p.LocArgs,
        LaunchImage: p.LaunchImage,
    }

    aps := alertBodyAps{
        Alert: alertBody,
        Badge: p.Badge,
        Sound: p.Sound,
        ContentAvailable: p.ContentAvailable,
    }

    fullPayload, err := constructFullPayload(aps, p.CustomFields)
    if (err != nil) {
        return nil, err
    }

    jsonStr, err = json.Marshal(fullPayload)
    if (err != nil) {
        return nil, err
    }

    payloadLen := len(jsonStr)

    if payloadLen > maxPayloadSize {
        clipSize := payloadLen - (maxPayloadSize) + 3 //need extra characters for ellipse
        if clipSize > len(p.AlertText) {
            return nil, errors.New(fmt.Sprintf("Payload was too long to successfully marshall to less than %v", maxPayloadSize))
        }
        aps.Alert.Body = aps.Alert.Body[:len(aps.Alert.Body) - clipSize] + "..."
        fullPayload["aps"] = aps
        if (err != nil) {
            return nil, err
        }

        jsonStr, err = json.Marshal(fullPayload)
        if (err != nil) {
            return nil, err
        }
    }

    return jsonStr, nil
}
