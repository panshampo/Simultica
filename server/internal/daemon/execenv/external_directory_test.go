package execenv

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExternalWritableRoots(t *testing.T) {
	resources := []ProjectResourceForEnv{
		{
			ID:           "r1",
			ResourceType: "external_directory",
			ResourceRef:  json.RawMessage(`{"local_path":"/Users/foo/Documents/../Documents","daemon_id":"daemon-a"}`),
		},
		{
			ID:           "r2",
			ResourceType: "external_directory",
			ResourceRef:  json.RawMessage(`{"local_path":"/Users/foo/Code","daemon_id":"daemon-a"}`),
		},
		{
			ID:           "r3",
			ResourceType: "external_directory",
			ResourceRef:  json.RawMessage(`{"local_path":"/Users/foo/Other","daemon_id":"daemon-b"}`),
		},
		{
			ID:           "r4",
			ResourceType: "local_directory",
			ResourceRef:  json.RawMessage(`{"local_path":"/Users/foo/InPlace","daemon_id":"daemon-a"}`),
		},
	}

	got := externalWritableRoots(resources, "daemon-a")
	want := []string{"/Users/foo/Code", "/Users/foo/Documents"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("externalWritableRoots = %#v, want %#v", got, want)
	}
}

func TestExternalWritableRootsRequiresDaemonID(t *testing.T) {
	resources := []ProjectResourceForEnv{
		{
			ID:           "r1",
			ResourceType: "external_directory",
			ResourceRef:  json.RawMessage(`{"local_path":"/Users/foo/Documents","daemon_id":"daemon-a"}`),
		},
	}
	if got := externalWritableRoots(resources, ""); got != nil {
		t.Fatalf("externalWritableRoots without daemon = %#v, want nil", got)
	}
}
