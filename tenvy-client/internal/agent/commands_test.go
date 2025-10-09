package agent

import (
	"reflect"
	"testing"
)

func TestMergeEnvironmentsCaseSensitive(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/home/test"}
	overrides := map[string]string{"PATH": "/custom/bin", "EMPTY": ""}

	got := mergeEnvironmentsWithComparer(base, overrides, false)
	want := []string{"HOME=/home/test", "PATH=/custom/bin", "EMPTY="}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected environment merge result: got %v want %v", got, want)
	}
}

func TestMergeEnvironmentsCaseInsensitive(t *testing.T) {
	base := []string{"Path=/usr/bin", "ComSpec=C:/Windows/System32/cmd.exe"}
	overrides := map[string]string{"PATH": `C:/Program Files/Custom/bin`}

	got := mergeEnvironmentsWithComparer(base, overrides, true)
	want := []string{"ComSpec=C:/Windows/System32/cmd.exe", "PATH=C:/Program Files/Custom/bin"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected environment merge result: got %v want %v", got, want)
	}
}

func TestMergeEnvironmentsDeduplicatesBase(t *testing.T) {
	base := []string{"A=1", "A=2", "B=3", " =invalid"}
	overrides := map[string]string{"C": "4"}

	got := mergeEnvironmentsWithComparer(base, overrides, false)
	want := []string{"A=1", "B=3", " =invalid", "C=4"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected environment merge result: got %v want %v", got, want)
	}
}
