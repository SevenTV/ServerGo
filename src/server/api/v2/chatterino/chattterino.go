package chatterino

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/gofiber/fiber/v2"
)

func Chatterino(app fiber.Router) fiber.Router {
	chatterino := app.Group("/chatterino")

	chatterino.Get("/version/:os/:branch", func(c *fiber.Ctx) error {
		download := configure.Config.GetString(fmt.Sprintf("chatterino.%s.%s.download", c.Params("branch"), c.Params("os")))
		portableDownload := configure.Config.GetString(fmt.Sprintf("chatterino.%s.%s.portable_download", c.Params("branch"), c.Params("os")))
		updateExe := configure.Config.GetString(fmt.Sprintf("chatterino.%s.%s.updateexe", c.Params("branch"), c.Params("os")))
		version := configure.Config.GetString("chatterino.version")

		result := VersionResult{
			download,
			portableDownload,
			updateExe,
			version,
		}

		b, err := json.Marshal(result)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		return c.Status(200).Send(b)
	})

	return chatterino
}

type VersionResult struct {
	Download         string `json:"download"`
	PortableDownload string `json:"portable_download"`
	UpdateExe        string `json:"updateexe"`
	Version          string `json:"version"`
}
