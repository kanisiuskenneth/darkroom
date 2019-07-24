package native

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"github.com/adalberht/imageorient"
	"github.com/anthonynsimon/bild/clone"
	"github.com/anthonynsimon/bild/effect"
	"github.com/anthonynsimon/bild/transform"
	"github.com/gojek/darkroom/pkg/processor"
)

var resizeBoundOption = &transform.RotationOptions{
	ResizeBounds: true,
}

var fixOperationFunctions = map[int]imageorient.FixOrientationFunction {
	2: func(img image.Image) (image.Image, error) {
		return transform.FlipH(img), nil
	},
	3: func(img image.Image) (image.Image, error) {
		return transform.Rotate(img, 180, nil), nil
	},
	4: func(img image.Image) (image.Image, error) {
		img = transform.FlipH(img)
		return transform.Rotate(img, 180, nil), nil
	},
	5: func(img image.Image) (image.Image, error) {
		img = transform.FlipV(img)
		img = transform.Rotate(img, 90, resizeBoundOption)
		return img, nil
	},
	6: func(img image.Image) (image.Image, error) {
		return transform.Rotate(img, 90, resizeBoundOption), nil
	},
	7: func(img image.Image) (image.Image, error) {
		img = transform.FlipV(img)
		return transform.Rotate(img, 270, resizeBoundOption), nil
	},
	8: func(img image.Image) (image.Image, error) {
		return transform.Rotate(img, 270, resizeBoundOption), nil
	},
}

// BildProcessor uses bild library to process images using native Golang image.Image interface
type BildProcessor struct {
	decoder imageorient.Decoder
	encoders *Encoders
}

// Crop takes an input image, width, height and a CropPoint and returns the cropped image
func (bp *BildProcessor) Crop(img image.Image, width, height int, point processor.CropPoint) image.Image {
	w, h := getResizeWidthAndHeightForCrop(width, height, img.Bounds().Dx(), img.Bounds().Dy())

	img = transform.Resize(img, w, h, transform.Linear)
	x0, y0 := getStartingPointForCrop(w, h, width, height, point)
	rect := image.Rect(x0, y0, width+x0, height+y0)
	img = (clone.AsRGBA(img)).SubImage(rect)

	return img
}

// Resize takes an input image, width and height and returns the re-sized image
func (bp *BildProcessor) Resize(img image.Image, width, height int) image.Image {

	initW := img.Bounds().Dx()
	initH := img.Bounds().Dy()

	w, h := getResizeWidthAndHeight(width, height, initW, initH)
	if w != initW || h != initH {
		img = transform.Resize(img, w, h, transform.Linear)
	}

	return img
}

// Watermark takes an input byte array, overlay byte array and opacity value
// and returns the watermarked image bytes or error
func (bp *BildProcessor) Watermark(base []byte, overlay []byte, opacity uint8) ([]byte, error) {
	baseImg, f, err := bp.Decode(base)
	if err != nil {
		return nil, err
	}
	overlayImg, _, err := bp.Decode(overlay)
	if err != nil {
		return nil, err
	}

	ratio := float64(overlayImg.Bounds().Dy()) / float64(overlayImg.Bounds().Dx())
	dWidth := float64(baseImg.Bounds().Dx()) / 2.0

	// Resizing overlay image according to base image
	overlayImg = transform.Resize(overlayImg, int(dWidth), int(dWidth*ratio), transform.Linear)

	// Anchor point for overlaying
	x := (baseImg.Bounds().Dx() - overlayImg.Bounds().Dx()) / 2
	y := (baseImg.Bounds().Dy() - overlayImg.Bounds().Dy()) / 2
	offset := image.Pt(int(x), int(y))

	// Mask image (that is just a solid light gray image)
	mask := image.NewUniform(color.Alpha{A: opacity})

	// Performing overlay
	draw.DrawMask(baseImg.(draw.Image), overlayImg.Bounds().Add(offset), overlayImg, image.ZP, mask, image.ZP, draw.Over)

	return bp.Encode(baseImg, f)
}

// GrayScale takes an input image and returns the grayscaled image
func (bp *BildProcessor) GrayScale(img image.Image) image.Image {
	// Rec. 601 Luma formula (https://en.wikipedia.org/wiki/Luma_%28video%29#Rec._601_luma_versus_Rec._709_luma_coefficients)
	return effect.GrayscaleWithWeights(img, 0.299, 0.587, 0.114)
}

// Flip takes an input image and returns the image flipped. The direction of flip
// is determined by the specified mode - 'v' for a vertical flip, 'h' for a
// horizontal flip and 'vh'(or 'hv') for both.
func (bp *BildProcessor) Flip(img image.Image, mode string) image.Image {
	mode = strings.ToLower(mode)
	for _, op := range mode {
		switch op {
		case 'v':
			img = transform.FlipV(img)
		case 'h':
			img = transform.FlipH(img)
		}
	}
	return img
}

// Rotate takes an input image and returns a image rotated by the specified degrees.
// The rotation is applied clockwise, and fractional angles are also supported.
func (bp *BildProcessor) Rotate(img image.Image, angle float64) image.Image {
	return transform.Rotate(img, angle, nil)
}

// Decode takes a byte array and returns the decoded image, format, or the error
func (bp *BildProcessor) Decode(data []byte) (image.Image, string, error) {
	img, f, err := bp.decoder.Decode(bytes.NewReader(data))
	return img, f, err
}

// Encode takes an image and the preferred format (extension) of the output
// Current supported format are "png", "jpg" and "jpeg"
func (bp *BildProcessor) Encode(img image.Image, fmt string) ([]byte, error) {
	enc := bp.encoders.GetEncoder(img, fmt)
	data, err := enc.Encode(img)
	return data, err
}

// NewBildProcessor creates a new BildProcessor with default compression
func NewBildProcessor() *BildProcessor {
	return &BildProcessor{
		decoder: imageorient.NewDecoder(fixOperationFunctions),
		encoders: NewEncoders(DefaultCompressionOptions),
	}
}

// NewBildProcessorWithCompression takes an input of encoding options
// 	and creates a newBildProcessor with custom compression options
func NewBildProcessorWithCompression(opts *CompressionOptions) *BildProcessor {
	return &BildProcessor{
		decoder: imageorient.NewDecoder(fixOperationFunctions),
		encoders: NewEncoders(opts),
	}
}
