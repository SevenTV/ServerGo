package encoding

import (
	"fmt"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
	"gopkg.in/gographics/imagick.v3/imagick"
)

type imageMagick struct{}

var ImageMagick = imageMagick{}

func (imageMagick) Encode(ogFilePath string, outFile string, width int32, height int32, quality string) error {
	var err error
	// Create new boundaries for frames
	mw := imagick.NewMagickWand() // Get magick wand & read the original image
	if err = mw.SetResourceLimit(imagick.RESOURCE_MEMORY, 500); err != nil {
		log.WithError(err).Error("SetResourceLimit")
	}
	if err := mw.ReadImage(ogFilePath); err != nil {
		return fmt.Errorf("Input File Not Readable: %s", err)
	}

	// Merge all frames with coalesce
	aw := mw.CoalesceImages()
	if err = aw.SetResourceLimit(imagick.RESOURCE_MEMORY, 500); err != nil {
		log.WithError(err).Error("SetResourceLimit")
	}
	mw.Destroy()
	defer aw.Destroy()

	// Set delays
	mw = imagick.NewMagickWand()
	if err = mw.SetResourceLimit(imagick.RESOURCE_MEMORY, 500); err != nil {
		log.WithError(err).Error("SetResourceLimit")
	}
	defer mw.Destroy()

	// Add each frame to our animated image
	mw.ResetIterator()
	for ind := 0; ind < int(aw.GetNumberImages()); ind++ {
		aw.SetIteratorIndex(ind)
		img := aw.GetImage()

		if err = img.ResizeImage(uint(width), uint(height), imagick.FILTER_LANCZOS); err != nil {
			log.WithError(err).Errorf("ResizeImage i=%v", ind)
			continue
		}
		if err = mw.AddImage(img); err != nil {
			log.WithError(err).Errorf("AddImage i=%v", ind)
		}
		img.Destroy()
	}

	// Done - convert to WEBP
	q, _ := strconv.Atoi(quality)
	if err = mw.SetImageCompressionQuality(uint(q)); err != nil {
		log.WithError(err).Error("SetImageCompressionQuality")
		return err
	}
	if err = mw.SetImageFormat("webp"); err != nil {
		log.WithError(err).Error("SetImageFormat")
		return err
	}

	// Write to file
	b := mw.GetImagesBlob()
	file, err := os.Create(outFile)
	if err != nil {
		log.WithError(err).Error("could not write image bytes")
		return err
	}
	_, err = file.Write(b)
	if err != nil {
		log.WithError(err).Error("could not write image bytes")
		return err
	}
	file.Close()

	return nil
}
