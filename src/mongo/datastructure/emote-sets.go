package datastructure

import "go.mongodb.org/mongo-driver/bson/primitive"

type EmoteSet struct {
	ID primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	// Numeric unique ID for the set
	// Starts from 1 and increments per set created
	NumericID string `json:"id" bson:"id"`
	// Whether or not the emote set can be edited
	Immutable bool `json:"immutable" bson:"immutable"`
	// Whether or not the set is active. When false, the set isn't returned in various API endpoints
	Active bool `json:"active" bson:"active"`
	// The emotes assigned to this set
	EmoteIDs []primitive.ObjectID `bson:"emote_ids" bson:"emote_ids"`
	// The maximum amount of emotes this set is allowed to contain
	EmoteSlots int32 `json:"emote_slots" bson:"emote_slots"`
	// The set's editors, who are allowed to edit the set's emotes
	EditorIDs []primitive.ObjectID `json:"editor_ids" bson:"editor_ids"`

	// The type of emote set. Can be SELECTIVE or GLOBAL. if SELECTIVE, the only applies to select channels
	Type EmoteSetType `json:"type" bson:"type"`
	// The channels this set may apply to. This is only relevant when the set type is "SELECTIVE"
	Selection []primitive.ObjectID `json:"selection" bson:"selection"`

	// Relational
	Editors []*User `json:"editors" bson:"-"`
}

type EmoteSetType string

var (
	// Global Sets apply to all channels, regardless of channel selection
	EmoteSetTypeGlobal EmoteSetType = "GLOBAL"
	// Selective Sets apply only to select channels
	EmoteSetTypeSelective EmoteSetType = "SELECTIVE"
)
