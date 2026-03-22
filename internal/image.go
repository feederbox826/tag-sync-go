package internal

import (
	"image"
	"os"
	"path/filepath"

	_ "golang.org/x/image/webp"
)

func GetImageDimensionsWithType(filePath string) (width, height int, format string, err error) {
	ext := filepath.Ext(filePath)
	if len(ext) > 1 {
		format = ext[1:]
	}
	if ext != ".webp" {
		return 0, 0, format, nil
	}
	reader, err := os.Open(filePath)
	if err != nil {
		return 0, 0, "", err
	}
	defer reader.Close()

	im, imgFormat, err := image.DecodeConfig(reader)
	if err != nil {
		return 0, 0, "", err
	}
	return im.Width, im.Height, imgFormat, nil
}
