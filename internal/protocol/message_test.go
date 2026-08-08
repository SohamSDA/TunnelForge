package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	want := HTTPRequest{
		Method:  http.MethodPost,
		Path:    "/hooks",
		Query:   "source=test",
		Headers: map[string][]string{"Content-Type": {"application/octet-stream"}},
		Body:    []byte{0, 1, 2, 255},
	}
	message, err := NewMessage(TypeHTTPRequest, "request-1", want)
	if err != nil {
		t.Fatalf("NewMessage() error: %v", err)
	}

	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}
	var got HTTPRequest
	if err := decoded.PayloadAs(&got); err != nil {
		t.Fatalf("PayloadAs() error: %v", err)
	}
	if got.Method != want.Method || got.Path != want.Path || !bytes.Equal(got.Body, want.Body) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestMessageValidation(t *testing.T) {
	validRequest, err := json.Marshal(HTTPRequest{Method: http.MethodGet, Path: "/"})
	if err != nil {
		t.Fatal(err)
	}
	largeBody, err := json.Marshal(HTTPRequest{Method: http.MethodPost, Path: "/", Body: make([]byte, MaxBodyBytes+1)})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		message Message
		wantErr error
	}{
		{name: "valid", message: Message{Version: Version, Type: TypeHTTPRequest, RequestID: "1", Payload: validRequest}},
		{name: "unsupported version", message: Message{Version: 99, Type: TypeHTTPRequest, RequestID: "1", Payload: validRequest}, wantErr: ErrUnsupportedVersion},
		{name: "unknown type", message: Message{Version: Version, Type: "mystery", Payload: []byte(`{}`)}, wantErr: ErrInvalidMessage},
		{name: "missing request ID", message: Message{Version: Version, Type: TypeHTTPRequest, Payload: validRequest}, wantErr: ErrInvalidMessage},
		{name: "unexpected request ID", message: Message{Version: Version, Type: TypeHeartbeat, RequestID: "1", Payload: []byte(`{"unix_milliseconds":1}`)}, wantErr: ErrInvalidMessage},
		{name: "missing payload", message: Message{Version: Version, Type: TypeHeartbeat}, wantErr: ErrInvalidMessage},
		{name: "large body", message: Message{Version: Version, Type: TypeHTTPRequest, RequestID: "1", Payload: largeBody}, wantErr: ErrInvalidMessage},
		{name: "invalid status", message: Message{Version: Version, Type: TypeHTTPResponse, RequestID: "1", Payload: []byte(`{"status_code":99}`)}, wantErr: ErrInvalidMessage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.message.Validate()
			if test.wantErr == nil && err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestDecodeRejectsMalformedInput(t *testing.T) {
	tests := []string{
		`{"version":1,"type":"heartbeat","payload":{},"extra":true}`,
		`{"version":1,"type":"heartbeat","payload":{}} {}`,
		`not-json`,
	}
	for _, input := range tests {
		if _, err := Decode(strings.NewReader(input)); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("Decode(%q) error = %v, want ErrInvalidMessage", input, err)
		}
	}
}

func TestHeadersMakesDeepCopy(t *testing.T) {
	original := http.Header{"X-Test": {"one", "two"}}
	got := Headers(original)
	original["X-Test"][0] = "changed"
	if got["X-Test"][0] != "one" {
		t.Fatalf("Headers() did not copy values: %v", got)
	}
}
