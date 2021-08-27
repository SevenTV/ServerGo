package validation

import (
	"regexp"

	"github.com/SevenTV/ServerGo/src/utils"
)

var (
	emoteNameRegex = regexp.MustCompile(`^[-_A-Za-z():0-9]{2,100}$`)
	emoteTagRegex  = regexp.MustCompile(`^[a-z]{3,30}$`)

//	ValidateEmoteTag = regexp.MustCompile(`^[\\w-]{2,100}$`)
)

func ValidateEmoteName(name []byte) bool {
	return emoteNameRegex.Match(name)
}

func ValidateEmoteTags(tags []string) (bool, string) {
	for _, s := range tags {
		if ok := emoteTagRegex.Match(utils.S2B(s)); !ok {
			return false, s
		}
	}

	return true, ""
}
