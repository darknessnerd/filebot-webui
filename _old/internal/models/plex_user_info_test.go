package models

import (
	"encoding/json"
	"testing"
)

func TestPlexUserInfoUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectedID string
	}{
		{
			name:       "ID as float64",
			input:      `{"id": 123456, "username": "user1", "email": "user1@example.com", "avatar": "avatar1.png"}`,
			expectedID: "123456",
		},
		{
			name:       "ID as string",
			input:      `{"id": "654321", "username": "user2", "email": "user2@example.com", "avatar": "avatar2.png"}`,
			expectedID: "654321",
		},
		{
			name:       "ID as json.Number",
			input:      `{"id": 789012, "username": "user3", "email": "user3@example.com", "avatar": "avatar3.png"}`,
			expectedID: "789012",
		},
		{
			name:       "ID as nil",
			input:      `{"id": null, "username": "user4", "email": "user4@example.com", "avatar": "avatar4.png"}`,
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var info PlexUserInfo
			err := json.Unmarshal([]byte(tt.input), &info)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if info.ID != tt.expectedID {
				t.Errorf("expected ID %q, got %q", tt.expectedID, info.ID)
			}
		})
	}
}
