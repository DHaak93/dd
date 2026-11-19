package dd

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

// testKey is the phone secret key shared by the test Conn and the fake hub.
var testKey = md5hash("secret")

// encryptedMessage builds a Message as the hub would send it.
func encryptedMessage(t *testing.T, key []byte, processID string, payload string) *Message {
	t.Helper()
	const ts = 12345
	c, err := NewEncCipher(key, ts)
	if err != nil {
		t.Fatalf("NewEncCipher: %v", err)
	}
	return &Message{
		ProcessID: processID,
		dataPayload: dataPayload{
			IsEncrypted: true,
			Time:        ts,
			Data:        base64.StdEncoding.EncodeToString(c.Encrypt([]byte(payload))),
		},
	}
}

// testConn starts a fake hub and returns a Conn pointed at it. The handler receives
// each decoded request and returns the messages to include in the response.
func testConn(t *testing.T, handler func(req genericRequest) []*Message) *Conn {
	t.Helper()

	// SimpleRequest always targets DefaultPort, so the fake hub must listen there.
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(DefaultPort))
	if err != nil {
		t.Skipf("port %d unavailable: %v", DefaultPort, err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req genericRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		raw, _ := json.Marshal(handler(req))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"messages": string(raw)})
	}))
	srv.Listener.Close()
	srv.Listener = l
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return &Conn{
		Host:           "127.0.0.1",
		RequestMode:    true,
		phoneSecret:    testKey,
		phoneSecretRaw: []byte("secret"),
		sessionSecret:  []byte("session"),
		processID:      "test",
		unresolvedRPC:  make(map[string]chan *Message),
	}
}

func TestMessages_NotDuplicated(t *testing.T) {
	dc := testConn(t, func(req genericRequest) []*Message {
		return []*Message{encryptedMessage(t, testKey, "", `{"devices":[]}`)}
	})

	messages, err := dc.Messages()
	if err != nil {
		t.Fatalf("Messages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Messages() returned %d messages, want 1", len(messages))
	}
	if string(messages[0].DecodedMessage) != `{"devices":[]}` {
		t.Errorf("DecodedMessage = %q", messages[0].DecodedMessage)
	}
}

func TestMessages_BadMessageSkipped(t *testing.T) {
	dc := testConn(t, func(req genericRequest) []*Message {
		bad := &Message{dataPayload: dataPayload{IsEncrypted: true, Time: 1, Data: "AAAA"}} // 3 bytes: not a block
		return []*Message{bad, encryptedMessage(t, testKey, "", `{}`)}
	})

	messages, err := dc.Messages()
	if err != nil {
		t.Fatalf("Messages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Messages() returned %d messages, want 1 (bad one skipped)", len(messages))
	}
}

func TestRPC_InlineResponse(t *testing.T) {
	dc := testConn(t, func(req genericRequest) []*Message {
		return []*Message{encryptedMessage(t, testKey, req.ProcessID, `{"code":0,"value":"ok"}`)}
	})

	var out struct {
		Value string `json:"value"`
	}
	if err := dc.RPC(RPC{Path: "/app/res/action", Output: &out}); err != nil {
		t.Fatalf("RPC() error = %v", err)
	}
	if out.Value != "ok" {
		t.Errorf("Value = %q, want ok", out.Value)
	}
	if n := len(dc.unresolvedRPC); n != 0 {
		t.Errorf("unresolvedRPC has %d leftover entries, want 0", n)
	}
}

func TestRPC_DeferredResponse(t *testing.T) {
	var mu sync.Mutex
	var firstPID string
	dc := testConn(t, func(req genericRequest) []*Message {
		mu.Lock()
		defer mu.Unlock()
		if firstPID == "" {
			// The RPC itself: no reply yet.
			firstPID = req.ProcessID
			return nil
		}
		// A later poll carries the reply for the original RPC.
		return []*Message{encryptedMessage(t, testKey, firstPID, `{"code":0,"value":"later"}`)}
	})

	var out struct {
		Value string `json:"value"`
	}
	if err := dc.RPC(RPC{Path: "/app/res/action", Output: &out}); err != nil {
		t.Fatalf("RPC() error = %v", err)
	}
	if out.Value != "later" {
		t.Errorf("Value = %q, want later", out.Value)
	}
	if n := len(dc.unresolvedRPC); n != 0 {
		t.Errorf("unresolvedRPC has %d leftover entries, want 0", n)
	}
}

func TestRPC_ErrorCode(t *testing.T) {
	dc := testConn(t, func(req genericRequest) []*Message {
		return []*Message{encryptedMessage(t, testKey, req.ProcessID, `{"code":7,"description":"nope"}`)}
	})

	if err := dc.RPC(RPC{Path: "/app/res/action"}); err == nil {
		t.Fatal("RPC() with non-zero code should return an error")
	}
}

func TestDecrypt_BadLength(t *testing.T) {
	key := make([]byte, 16)
	for _, n := range []int{0, 1, 15, 17} {
		c, err := NewDecCipher(key, 1)
		if err != nil {
			t.Fatalf("NewDecCipher: %v", err)
		}
		if _, err := c.Decrypt(make([]byte, n)); err == nil {
			t.Errorf("Decrypt(%d bytes) should return an error", n)
		}
	}
}

func TestPKCS5Trimming_Empty(t *testing.T) {
	if got := PKCS5Trimming(nil); len(got) != 0 {
		t.Errorf("PKCS5Trimming(nil) = %v, want empty", got)
	}
}
