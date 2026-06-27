package main

import "testing"

func TestActiveTraqChannelsSkipsArchivedAncestors(t *testing.T) {
	archivedParent := "archived-parent"
	active := activeTraqChannels([]traqChannel{
		{ID: "active", Name: "active"},
		{ID: archivedParent, Name: "archived", Archived: true},
		{ID: "child", Name: "child", ParentID: &archivedParent},
		{Name: "empty"},
	})

	if len(active) != 1 {
		t.Fatalf("len(active) = %d, want 1", len(active))
	}
	if active[0].ID != "active" {
		t.Fatalf("active[0].ID = %q, want active", active[0].ID)
	}
}

func TestTriggerInActiveChannels(t *testing.T) {
	active := map[string]bool{"active": true}
	tests := []struct {
		name    string
		trigger triggerPayload
		want    bool
	}{
		{name: "message active", trigger: triggerPayload{Type: "msg", Ch: "active"}, want: true},
		{name: "message inactive", trigger: triggerPayload{Type: "msg", Ch: "inactive"}, want: false},
		{name: "movement active", trigger: triggerPayload{Type: "mov", To: "active"}, want: true},
		{name: "movement inactive", trigger: triggerPayload{Type: "mov", To: "inactive"}, want: false},
		{name: "unknown", trigger: triggerPayload{Type: "other", Ch: "active"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := triggerInActiveChannels(tt.trigger, active); got != tt.want {
				t.Fatalf("triggerInActiveChannels() = %t, want %t", got, tt.want)
			}
		})
	}
}
