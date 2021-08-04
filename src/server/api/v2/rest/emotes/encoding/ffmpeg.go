package encoding

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

type ffmpeg struct{}

var FFmpeg = ffmpeg{}

func (ffmpeg) Encode(ogFilePath string, outPath string, width int32, height int32, quality int64) error {
	cmd := exec.Command("ffmpeg", []string{
		"-i", ogFilePath,
		"-vcodec", "libwebp",
		"-loop", "0",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		"-lossless", "0",
		"-compression_level", "4",
		"-q:v", strconv.Itoa(int(quality)),
		"-f", "webp",
		outPath,
	}...)

	cmd.Stderr = os.Stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
