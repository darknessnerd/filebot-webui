package models

import (
	"encoding/json"
	"fmt"
)

// PlexUserInfo represents user information from Plex OAuth
// See: https://plex.tv/api/v2/user
// Example fields: id, username, email, avatar
// You may need to adjust based on Plex API response

type PlexUserInfo struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"` // Will be set from thumb
	Thumb       string `json:"thumb"`
	AccessToken string `'json:"access_token"` // Not from Plex API, set manually
}

func (p *PlexUserInfo) UnmarshalJSON(data []byte) error {
	type Alias PlexUserInfo
	aux := &struct {
		ID    interface{} `json:"id"`
		Thumb string      `json:"thumb"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch v := aux.ID.(type) {
	case float64:
		p.ID = fmt.Sprintf("%.0f", v)
	case int:
		p.ID = fmt.Sprintf("%d", v)
	case json.Number:
		p.ID = v.String()
	case string:
		p.ID = v
	case nil:
		p.ID = ""
	default:
		p.ID = fmt.Sprintf("%v", v)
	}
	p.Avatar = aux.Thumb // Set Avatar from thumb field
	return nil
}
