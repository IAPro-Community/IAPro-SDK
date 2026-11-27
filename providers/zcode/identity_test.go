package zcode

import "testing"

func TestCredentialMetadata(t *testing.T) {
	if !HasCredentialMetadata([]byte(`{"oauth:zai:access_token":"x"}`)) || HasCredentialMetadata([]byte(`{"oauth:active_provider":"zai"}`)) {
		t.Fatal("provider selector alone is not a credential")
	}
}
