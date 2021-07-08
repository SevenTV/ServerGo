package restutil

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
)

type ErrorResponse struct {
	Status  int
	Message string
}

func (e *ErrorResponse) Send(c *fiber.Ctx, placeholders ...string) error {
	if len(placeholders) > 0 {
		e.Message = fmt.Sprintf(e.Message, strings.Join(placeholders, ", "))
	}

	b, _ := json.Marshal(e)
	return c.Status(e.Status).Send(b)
}

func createErrorResponse(status int, message string) *ErrorResponse {
	return &ErrorResponse{
		status, message,
	}
}

var (
	ErrUnknownEmote       = func() *ErrorResponse { return createErrorResponse(404, "Unknown Emote") }
	ErrUnknownUser        = func() *ErrorResponse { return createErrorResponse(404, "Unknown User") }
	MalformedObjectId     = func() *ErrorResponse { return createErrorResponse(400, "Malformed Object ID") }
	ErrInternalServer     = func() *ErrorResponse { return createErrorResponse(500, "Internal Server Error (%s)") }
	ErrBadRequest         = func() *ErrorResponse { return createErrorResponse(400, "Bad Request (%s)") }
	ErrLoginRequired      = func() *ErrorResponse { return createErrorResponse(403, "Authentication Required") }
	ErrAccessDenied       = func() *ErrorResponse { return createErrorResponse(403, "Insufficient Privilege") }
	ErrMissingQueryParams = func() *ErrorResponse { return createErrorResponse(400, "Missing Query Params (%s)") }
)

func CreateEmoteResponse(emote *datastructure.Emote, owner *datastructure.User) EmoteResponse {
	// Generate URLs
	urls := make([][]string, 4)
	for i := 1; i <= 4; i++ {
		a := make([]string, 2)
		a[0] = fmt.Sprintf("%d", i)
		a[1] = utils.GetCdnURL(emote.ID.Hex(), int8(i))

		urls[i-1] = a
	}

	// Generate simple visibility value
	simpleVis := emote.GetSimpleVisibility()

	// Create the final response
	response := EmoteResponse{
		ID:               emote.ID.Hex(),
		Name:             emote.Name,
		Owner:            nil,
		Visibility:       emote.Visibility,
		VisibilitySimple: &simpleVis,
		Mime:             emote.Mime,
		Status:           emote.Status,
		Tags:             emote.Tags,
		Width:            emote.Width,
		Height:           emote.Height,
		URLs:             urls,
	}
	if owner != nil {
		response.Owner = CreateUserResponse(owner)
	}

	return response
}

type EmoteResponse struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Owner            *UserResponse `json:"owner"`
	Visibility       int32         `json:"visibility"`
	VisibilitySimple *[]string     `json:"visibility_simple"`
	Mime             string        `json:"mime"`
	Status           int32         `json:"status"`
	Tags             []string      `json:"tags"`
	Width            [4]int16      `json:"width"`
	Height           [4]int16      `json:"height"`
	URLs             [][]string    `json:"urls"`
}

func CreateUserResponse(user *datastructure.User) *UserResponse {
	response := UserResponse{
		ID:           user.ID.Hex(),
		Login:        user.Login,
		DisplayName:  user.DisplayName,
		Role:         datastructure.GetRole(user.RoleID),
		EmoteAliases: user.EmoteAlias,
	}

	return &response
}

type UserResponse struct {
	ID           string             `json:"id"`
	TwitchID     string             `json:"twitch_id"`
	Login        string             `json:"login"`
	DisplayName  string             `json:"display_name"`
	Role         datastructure.Role `json:"role"`
	EmoteAliases map[string]string  `json:"emote_aliases"`
}

func CreateBadgeResponse(badge *datastructure.Badge, users []*datastructure.User, idType string) *BadgeResponse {
	// Select user ID type
	userIDs := make([]string, len(users))
	for i, u := range users {
		switch idType {
		case "object_id":
			userIDs[i] = u.ID.Hex()
		case "twitch_id":
			userIDs[i] = u.TwitchID
		case "login":
			userIDs[i] = u.Login
		}
	}

	// Generate URLs
	urls := make([][]string, 3)
	for i := 1; i <= 3; i++ {
		a := make([]string, 2)
		a[0] = fmt.Sprintf("%d", i)
		a[1] = utils.GetBadgeCdnURL(badge.ID.Hex(), int8(i))

		urls[i-1] = a
	}

	response := &BadgeResponse{
		ID:      badge.ID.Hex(),
		Name:    badge.Name,
		Tooltip: badge.Tooltip,
		Users:   userIDs,
		URLs:    urls,
	}

	return response
}

type BadgeResponse struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Tooltip string     `json:"tooltip"`
	URLs    [][]string `json:"urls"`
	Users   []string   `json:"users"`
}
