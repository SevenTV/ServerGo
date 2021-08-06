package chatterino

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/gofiber/fiber/v2"
)

func Chatterino(app fiber.Router) fiber.Router {
	chatterino := app.Group("/chatterino")

	chatterino.Get("/version/:platform/:branch", func(c *fiber.Ctx) error {

		result := VersionResult{
			Download:         configure.Config.GetString(fmt.Sprintf("chatterino.%s.%s.portable_download", c.Params("branch"), c.Params("platform"))),
			PortableDownload: configure.Config.GetString(fmt.Sprintf("chatterino.%s.%s.download", c.Params("branch"), c.Params("platform"))),
			UpdateExe:        configure.Config.GetString(fmt.Sprintf("chatterino.%s.%s.updateexe", c.Params("branch"), c.Params("platform"))),
			Version:          configure.Config.GetString("chatterino.version"),
		}

		b, err := json.Marshal(result)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		c.Set("Content-Type", "application/json")
		return c.Status(200).Send(b)
	})

	return chatterino
}

type VersionResult struct {
	Download         string `json:"download"`
	PortableDownload string `json:"portable_download,omitempty"`
	UpdateExe        string `json:"updateexe,omitempty"`
	Version          string `json:"version"`
}
