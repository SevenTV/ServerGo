package resolvers

import (
	"fmt"
)

var (
	ErrInvalidName           = fmt.Errorf("Invalid Name")
	ErrLoginRequired         = fmt.Errorf("Authentication Required")
	ErrInvalidOwner          = fmt.Errorf("Invalid Owner ID")
	ErrInvalidTags           = fmt.Errorf("Too Many Tags (10)")
	ErrInvalidTag            = fmt.Errorf("Invalid Tags")
	ErrInvalidUpdate         = fmt.Errorf("Invalid Update")
	ErrUnknownEmote          = fmt.Errorf("Unknown Emote")
	ErrUnknownChannel        = fmt.Errorf("Unknown Channel")
	ErrUnknownUser           = fmt.Errorf("Unknown User")
	ErrAccessDenied          = fmt.Errorf("Insufficient Privilege")
	ErrUserBanned            = fmt.Errorf("User Is Banned")
	ErrUserNotBanned         = fmt.Errorf("User Is Not Banned")
	ErrYourself              = fmt.Errorf("Don't Be Silly")
	ErrNoReason              = fmt.Errorf("No Reason")
	ErrInternalServer        = fmt.Errorf("Internal Server Error")
	ErrDepth                 = fmt.Errorf("Max Depth Exceeded (%v)", MaxDepth)
	ErrQueryLimit            = fmt.Errorf("Max Query Limit Exceeded (%v)", QueryLimit)
	ErrInvalidSortOrder      = fmt.Errorf("SortOrder is either 0 (descending) or 1 (ascending)")
	ErrEmoteSlotLimitReached = func(count int32) error {
		return fmt.Errorf("Channel Emote Slots Limit Reached (%d)", count)
	}
)
