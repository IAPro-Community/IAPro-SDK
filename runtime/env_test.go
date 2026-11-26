package runtime

import (
	"reflect"
	"testing"
)

func TestWindowsEnvCaseInsensitive(t *testing.T) {
	got := envSetForOS([]string{"Home=old", "path=old", "Secret=old", "KEEP=yes"}, map[string]string{"HOME": "new", "PATH": "new"}, "windows", "SECRET")
	want := []string{"KEEP=yes", "HOME=new", "PATH=new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %v, want %v", got, want)
	}
}

func TestExplicitEmptyEnvironmentDoesNotInheritParent(t *testing.T) {
	if got := EnvSet([]string{}, nil); got == nil {
		t.Fatal("nil environment inherits host secrets")
	}
}
