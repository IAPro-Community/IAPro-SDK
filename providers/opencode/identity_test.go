package opencode

import "testing"

func TestCredentialMetadata(t *testing.T) {
	if !HasCredentialMetadata([]byte(`{"openai":{"access":"x"}}`)) || HasCredentialMetadata([]byte(`{"openai":{"type":"oauth"}}`)) {
		t.Fatal("wrong credential metadata detection")
	}
}
