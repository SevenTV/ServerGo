package api_proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/auth"
	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/configure"
)

const baseUrlTwitch = "https://api.twitch.tv"

func GetTwitchUser(ctx context.Context, login string) (*userTwitch, error) {
	// Set Request URI
	uri := fmt.Sprintf("%v/helix/users?login=%v", baseUrlTwitch, login)

	// Get auth
	headers, err := getTwitchAuthorizeHeaders(ctx)
	if err != nil {
		return nil, err
	}

	// Send request
	resp, err := cache.CacheGetRequest(ctx, uri, time.Minute*30, time.Minute*15, headers...)
	if err != nil {
		return nil, err
	}

	// Decode
	var userResponse userResponseTwitch
	if err := json.Unmarshal(resp.Body, &userResponse); err != nil {
		return nil, err
	}

	if len(userResponse.Data) == 0 {
		return nil, nil
	}

	return &userResponse.Data[0], nil
}

func GetTwitchStreams(ctx context.Context, logins ...string) (*streamsResponseTwitch, error) {
	// Set Request URI
	uri := fmt.Sprintf("%v/helix/streams?user_login=%v", baseUrlTwitch, strings.Join(logins, ","))

	// Get auth
	headers, err := getTwitchAuthorizeHeaders(ctx)
	if err != nil {
		return nil, err
	}

	// Send request
	resp, err := cache.CacheGetRequest(ctx, uri, time.Minute*2+time.Second*30, time.Minute*2, headers...)
	if err != nil {
		return nil, err
	}

	var streamResponse *streamsResponseTwitch
	if err := json.Unmarshal(resp.Body, &streamResponse); err != nil {
		return nil, err
	}

	return streamResponse, nil
}

func GetTwitchFollowerCount(ctx context.Context, id string) (int32, error) {
	uri := fmt.Sprintf("%v/helix/users/follows?to_id=%v", baseUrlTwitch, id)

	// Get auth
	headers, err := getTwitchAuthorizeHeaders(ctx)
	if err != nil {
		return 0, err
	}

	resp, err := cache.CacheGetRequest(ctx, uri, time.Hour*3, time.Minute*1, headers...)
	if err != nil {
		return 0, err
	}

	var response *userFollowersResponseTwitch
	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return 0, err
	}

	return response.Total, nil
}

type userResponseTwitch struct {
	Data []userTwitch `json:"data"`
}

type userTwitch struct {
	ID              string    `json:"id"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
	DisplayName     string    `json:"display_name"`
	Logo            string    `json:"logo"`
	Login           string    `json:"login"`
	Type            string    `json:"type"`
	BroadcasterType string    `json:"broadcaster_type"`
}

type streamsResponseTwitch struct {
	Data       []streamTwitch    `json:"data"`
	Pagination map[string]string `json:"pagination"`
}

type streamTwitch struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	GameID       string    `json:"game_id"`
	GameName     string    `json:"game_name"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	ViewerCount  int32     `json:"viewer_count"`
	StartedAt    time.Time `json:"started_at"`
	Language     string    `json:"language"`
	ThumbnailURL string    `json:"thumbnail_url"`
	TagIDs       []string  `json:"tag_ids"`
	IsMature     bool      `json:"is_mature"`
}

type userFollowersResponseTwitch struct {
	Total int32 `json:"total"`
}

type RequestHeadersKeyValuePairs struct {
	Key   string
	Value string
}

func getTwitchAuthorizeHeaders(ctx context.Context) ([]struct {
	Key   string
	Value string
}, error) {
	token, err := auth.GetAuth(ctx)
	if err != nil {
		return []struct {
			Key   string
			Value string
		}{}, err
	}

	return []struct {
		Key   string
		Value string
	}{
		{Key: "Client-ID", Value: configure.Config.GetString("twitch_client_id")},
		{Key: "Authorization", Value: fmt.Sprintf("Bearer %v", token)},
	}, nil
}
