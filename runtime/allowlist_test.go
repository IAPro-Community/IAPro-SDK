// SPDX-License-Identifier: MIT

package runtime

import (
	"reflect"
	"testing"
)

func TestAllowEnvironmentNeverInheritsUnapprovedValues(t *testing.T) {
	parent := []string{"Path=/bin", "HOME=/home/test", "TOKEN=secret", "HTTPS_PROXY=credential", "BAD", "HOME=\x00"}
	want := []string{"Path=/bin", "HOME=/home/test"}
	if got := AllowEnvironment(parent, "PATH", "home"); !reflect.DeepEqual(got, want) {
		t.Fatal("unexpected environment")
	}
	if got := AllowEnvironment(nil); got == nil {
		t.Fatal("nil environment would inherit parent")
	}
}
