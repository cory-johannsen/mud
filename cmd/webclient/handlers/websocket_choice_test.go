package handlers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// REQ-BUG69-6: serverEventEncodedChoice MUST detect the "\x00choice\x00" sentinel in a
// MessageEvent and return the JSON payload along with the type name "FeatureChoicePrompt".
func TestServerEventEncodedChoice_DetectsSentinel(t *testing.T) {
	payload := map[string]interface{}{
		"featureId": "feat_path",
		"prompt":    "Choose your path:",
		"options":   []string{"Option Alpha", "Option Beta"},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	content := "\x00choice\x00" + string(data)
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: content},
		},
	}

	raw, msgType := handlers.ServerEventEncodedChoiceForTest(event)
	require.NotNil(t, raw, "must detect the sentinel and return non-nil payload")
	assert.Equal(t, "FeatureChoicePrompt", msgType)

	// Verify the returned JSON round-trips correctly.
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, "feat_path", decoded["featureId"])
	assert.Equal(t, "Choose your path:", decoded["prompt"])
}

// REQ-BUG69-7: serverEventEncodedChoice MUST return nil when the MessageEvent does NOT
// contain the "\x00choice\x00" sentinel.
func TestServerEventEncodedChoice_NoSentinel_ReturnsNil(t *testing.T) {
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: "Regular message text"},
		},
	}

	raw, msgType := handlers.ServerEventEncodedChoiceForTest(event)
	assert.Nil(t, raw)
	assert.Equal(t, "", msgType)
}

// REQ-BUG69-8: serverEventEncodedChoice MUST return nil when the event has no MessageEvent.
func TestServerEventEncodedChoice_NonMessageEvent_ReturnsNil(t *testing.T) {
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{
			RoomView: &gamev1.RoomView{Title: "A room"},
		},
	}

	raw, msgType := handlers.ServerEventEncodedChoiceForTest(event)
	assert.Nil(t, raw)
	assert.Equal(t, "", msgType)
}

// REQ-BUG69-9: serverEventEncodedChoice MUST NOT interfere with the loadout sentinel.
func TestServerEventEncodedChoice_DoesNotMatchLoadoutSentinel(t *testing.T) {
	content := "\x00loadout\x00{}"
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: content},
		},
	}

	raw, _ := handlers.ServerEventEncodedChoiceForTest(event)
	assert.Nil(t, raw, "loadout sentinel must not be matched by choice detector")
	_ = strings.Contains(content, "loadout") // suppress unused import warning
}
