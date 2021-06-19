package resolvers

import (
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
)

var (
	ErrInvalidName           = fmt.Errorf("invalid name")
	ErrLoginRequired         = fmt.Errorf("authentication required")
	ErrInvalidOwner          = fmt.Errorf("invalid ownerid")
	ErrInvalidTags           = fmt.Errorf("too many tags (10)")
	ErrInvalidTag            = fmt.Errorf("invalid tags")
	ErrInvalidUpdate         = fmt.Errorf("invalid update")
	ErrUnknownEmote          = fmt.Errorf("unknown emote")
	ErrUnknownChannel        = fmt.Errorf("unknown channel")
	ErrUnknownUser           = fmt.Errorf("unknown user")
	ErrAccessDenied          = fmt.Errorf("insufficient privilege")
	ErrUserBanned            = fmt.Errorf("user is banned")
	ErrUserNotBanned         = fmt.Errorf("user is not banned")
	ErrYourself              = fmt.Errorf("do not be silly")
	ErrNoReason              = fmt.Errorf("no reason")
	ErrInternalServer        = fmt.Errorf("internal server error")
	ErrDepth                 = fmt.Errorf("max depth exceeded (%v)", MaxDepth)
	ErrQueryLimit            = fmt.Errorf("max query limit exceeded (%v)", QueryLimit)
	ErrInvalidSortOrder      = fmt.Errorf("sort-order is either 0 (descending) or 1 (ascending)")
	ErrEmoteSlotLimitReached = fmt.Errorf("channel emote slots limit reached (%v)", configure.Config.GetInt("limits.meta.channel_emote_slots"))
)
