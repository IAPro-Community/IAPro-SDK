package spec

import (
	"reflect"
	"testing"
)

func TestLoginCommands(t *testing.T) {
	for id, want := range map[string][]string{
		"codex": {"login"}, "claude": {"auth", "login"},
		"agy": nil, "opencode": {"providers", "login"},
		"cursor": {"login"}, "zcode": {"login", "zai"},
	} {
		s := MustGet(id)
		if s.LoginCommand == nil || !reflect.DeepEqual(s.LoginCommand(), want) {
			t.Errorf("login contract drift for %s", id)
		}
	}
	if s := MustGet("cursor"); s.LogoutCommand == nil || !reflect.DeepEqual(s.LogoutCommand(), []string{"logout"}) {
		t.Fatal("Cursor logout contract drift")
	}
}
