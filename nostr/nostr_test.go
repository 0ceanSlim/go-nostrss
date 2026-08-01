package nostr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-nostrss/types"

	"github.com/gorilla/websocket"
)

// fakeRelay serves a websocket endpoint that reads one EVENT and replies with
// the frames supplied by reply, which receives the incoming event's ID.
func fakeRelay(t *testing.T, reply func(eventID string) []string) string {
	t.Helper()

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		_, message, err := ws.ReadMessage()
		if err != nil {
			return
		}

		var frame []json.RawMessage
		if err := json.Unmarshal(message, &frame); err != nil || len(frame) < 2 {
			return
		}
		var event types.NostrEvent
		if err := json.Unmarshal(frame[1], &event); err != nil {
			return
		}

		for _, out := range reply(event.ID) {
			if err := ws.WriteMessage(websocket.TextMessage, []byte(out)); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func testEvent(t *testing.T) *types.NostrEvent {
	t.Helper()

	event, err := CreateNostrEvent("hello", strings.Repeat("ab", 32), 1700000000, nil)
	if err != nil {
		t.Fatalf("CreateNostrEvent: %v", err)
	}
	return event
}

func TestSendEventAccepted(t *testing.T) {
	event := testEvent(t)
	url := fakeRelay(t, func(id string) []string {
		return []string{`["OK","` + id + `",true,""]`}
	})

	accepted, msg, err := SendEvent(url, *event)
	if err != nil {
		t.Fatalf("SendEvent: %v", err)
	}
	if !accepted {
		t.Errorf("accepted = false, msg = %q", msg)
	}
}

func TestSendEventRejected(t *testing.T) {
	event := testEvent(t)
	url := fakeRelay(t, func(id string) []string {
		return []string{`["OK","` + id + `",false,"blocked: pubkey not allowed"]`}
	})

	accepted, msg, err := SendEvent(url, *event)
	if err != nil {
		t.Fatalf("SendEvent: %v", err)
	}
	if accepted {
		t.Error("a relay rejection must not be reported as success")
	}
	if msg != "blocked: pubkey not allowed" {
		t.Errorf("msg = %q, want the relay's reason", msg)
	}
}

func TestSendEventSkipsNoticeAndForeignOK(t *testing.T) {
	event := testEvent(t)
	url := fakeRelay(t, func(id string) []string {
		return []string{
			`["NOTICE","restricted: pow required"]`,
			`["OK","0000000000000000000000000000000000000000000000000000000000000000",false,"other event"]`,
			`["OK","` + id + `",true,""]`,
		}
	})

	accepted, _, err := SendEvent(url, *event)
	if err != nil {
		t.Fatalf("SendEvent: %v", err)
	}
	if !accepted {
		t.Error("should have skipped the NOTICE and the OK for a different event")
	}
}

func TestSendEventUnreachableRelay(t *testing.T) {
	event := testEvent(t)
	if _, _, err := SendEvent("ws://127.0.0.1:1", *event); err == nil {
		t.Error("expected an error connecting to a dead relay")
	}
}

func TestSignAndPublishFanOut(t *testing.T) {
	event := testEvent(t)

	good := fakeRelay(t, func(id string) []string { return []string{`["OK","` + id + `",true,""]`} })
	bad := fakeRelay(t, func(id string) []string { return []string{`["OK","` + id + `",false,"rate-limited"]`} })
	dead := "ws://127.0.0.1:1"

	privKey := strings.Repeat("11", 32)
	results, err := SignAndPublish(event, privKey, []string{good, bad, dead})
	if err != nil {
		t.Fatalf("SignAndPublish: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// Results must stay aligned with the relay order despite running concurrently.
	if results[0].Relay != good || !results[0].Accepted {
		t.Errorf("result[0] = %+v, want accepted for %s", results[0], good)
	}
	if results[1].Relay != bad || results[1].Accepted || results[1].Message != "rate-limited" {
		t.Errorf("result[1] = %+v, want a rejection", results[1])
	}
	if results[2].Relay != dead || results[2].Err == nil {
		t.Errorf("result[2] = %+v, want a connection error", results[2])
	}

	if event.Sig == "" {
		t.Error("event should have been signed once before fan-out")
	}
}

func TestSignAndPublishNoRelays(t *testing.T) {
	if _, err := SignAndPublish(testEvent(t), strings.Repeat("11", 32), nil); err == nil {
		t.Error("expected an error when no relays are configured")
	}
}

func TestCreateNostrEventNilTags(t *testing.T) {
	event, err := CreateNostrEvent("hi", strings.Repeat("ab", 32), 1700000000, nil)
	if err != nil {
		t.Fatalf("CreateNostrEvent: %v", err)
	}
	// Must serialize as [] rather than null, or the ID will not match what a
	// relay computes.
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"tags":[]`) {
		t.Errorf("nil tags should marshal as []: %s", data)
	}
}
