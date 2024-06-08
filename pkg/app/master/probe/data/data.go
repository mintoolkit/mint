package data

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
)

const (
	DefaultText     = "text data"
	DefaultTextJSON = `{"data": "text"}`
)

const (
	TypeGenerateText      = "generate.text"
	TypeGenerateTextJSON  = "generate.text.json"
	TypeGenerateImage     = "generate.image"
	TypeGenerateImagePNG  = "generate.image.png"
	TypeGenerateImageJPEG = "generate.image.jpeg"
	TypeGenerateImageGIF  = "generate.image.gif"
)

func IsGenerateType(input string) bool {
	switch input {
	case TypeGenerateText, TypeGenerateTextJSON, TypeGenerateImagePNG, TypeGenerateImageJPEG, TypeGenerateImageGIF:
		return true
	}

	return false
}

func GenerateImage(genType string) ([]byte, error) {
	width := 10
	height := 10
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	//fillColor := color.RGBA{0, 0, 255, 255} //blue
	fillColor := color.RGBA{255, 0, 0, 255} //red
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, fillColor)
		}
	}

	var out bytes.Buffer
	var err error
	switch genType {
	case TypeGenerateImagePNG, TypeGenerateImage:
		err = png.Encode(&out, img)
	case TypeGenerateImageJPEG:
		err = jpeg.Encode(&out, img, nil)
	case TypeGenerateImageGIF:
		err = gif.Encode(&out, img, nil)
	default:
		err = fmt.Errorf("unknown image type: %s", genType)
	}

	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}
