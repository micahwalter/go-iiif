package native

// to consider... https://github.com/corona10/goimghdr

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/anthonynsimon/bild/effect"
	"github.com/anthonynsimon/bild/segment"
	"github.com/anthonynsimon/bild/transform"
	iiifconfig "github.com/go-iiif/go-iiif/config"
	iiifimage "github.com/go-iiif/go-iiif/image"
	iiifsource "github.com/go-iiif/go-iiif/source"
	"github.com/whosonfirst/go-whosonfirst-mimetypes"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	_ "log"
)

type NativeImage struct {
	iiifimage.Image
	config    *iiifconfig.Config
	source    iiifsource.Source
	source_id string
	id        string
	img       image.Image
	format    string
}

type NativeDimensions struct {
	iiifimage.Dimensions
	bounds image.Rectangle
}

func (d *NativeDimensions) Height() int {
	return d.bounds.Max.Y
}

func (d *NativeDimensions) Width() int {
	return d.bounds.Max.X
}

func (im *NativeImage) Update(body []byte) error {

	img, fmt, err := decodeImageBytes(body)

	if err != nil {
		return err
	}

	im.img = img
	im.format = fmt

	return nil
}

func (im *NativeImage) Body() []byte {

	body, _ := encodeImage(im.img, im.format)
	return body
}

func (im *NativeImage) Format() string {

	return im.format
}

func (im *NativeImage) ContentType() string {

	format := im.Format()

	t := mimetypes.TypesByExtension(format)
	return t[0]
}

func (im *NativeImage) Identifier() string {
	return im.id
}

func (im *NativeImage) Rename(id string) error {
	im.id = id
	return nil
}

func (im *NativeImage) Dimensions() (iiifimage.Dimensions, error) {

	dims := &NativeDimensions{
		bounds: im.img.Bounds(),
	}

	return dims, nil
}

func (im *NativeImage) Transform(t *iiifimage.Transformation) error {

	if t.Region != "full" {

		rgi, err := t.RegionInstructions(im)

		if err != nil {
			return err
		}

		if rgi.SmartCrop {

			// "github.com/muesli/smartcrop"
			return errors.New("Smart cropping is unavailable for native image processing")

			/*
				si, err := t.SizeInstructions(im)

				if err != nil {
					return err
				}

				resizer := nfnt.NewDefaultResizer()
				analyzer := smartcrop.NewAnalyzer(resizer)
				topCrop, err := analyzer.FindBestCrop(im.img, si.Width, si.Height)

				if err != nil {
					return err
				}

				type SubImager interface {
					SubImage(r image.Rectangle) image.Image
				}

				img := im.img.(SubImager).SubImage(topCrop)
				im.img = img
			*/

		} else {
			bounds := image.Rect(rgi.X, rgi.Y, rgi.Width, rgi.Height)
			img := transform.Crop(im.img, bounds)
			im.img = img
		}
	}

	if t.Size != "max" && t.Size != "full" {

		si, err := t.SizeInstructions(im)

		if err != nil {
			return err
		}

		img := transform.Resize(im.img, si.Width, si.Height, transform.Linear)
		im.img = img
	}

	ri, err := t.RotationInstructions(im)

	if err != nil {
		return nil
	}

	// auto-rotate checks... are they necessary in a plain-vanilla Go context?

	if ri.Angle > 0.0 {
		angle := float64(ri.Angle)
		img := transform.Rotate(im.img, angle, nil)
		im.img = img
	}

	if ri.Flip {
		img := transform.FlipH(im.img)
		im.img = img
	}

	switch t.Quality {
	case "color", "default":
		// do nothing.
	case "gray":
		img := effect.Grayscale(im.img)
		im.img = img
	case "bitonal":
		img := segment.Threshold(im.img, 128)
		im.img = img
	default:
		// this should be trapped above
	}

	fi, err := t.FormatInstructions(im)

	if err != nil {
		return err
	}

	encode := false

	if fi.Format != im.format {

		encode = true

		// sigh... computers, amirite?

		if fi.Format == "jpg" && im.format == "jpeg" {
			encode = false
		}
	}

	if encode {

		body, err := encodeImage(im.img, fi.Format)

		if err != nil {
			return err
		}

		img, format, err := decodeImageBytes(body)

		if err != nil {
			return err
		}

		im.img = img
		im.format = format
	}

	err = iiifimage.ApplyCustomTransformations(t, im)

	if err != nil {
		return err
	}

	return nil
}

func decodeImageBytes(body []byte) (image.Image, string, error) {

	buf := bytes.NewBuffer(body)
	return image.Decode(buf)
}

func encodeImage(im image.Image, format string) ([]byte, error) {

	var b bytes.Buffer
	wr := bufio.NewWriter(&b)

	var err error

	switch format {
	case "bmp":
		bmp.Encode(wr, im)
	case "jpg", "jpeg":
		opts := jpeg.Options{Quality: 100}
		err = jpeg.Encode(wr, im, &opts)
	case "png":
		err = png.Encode(wr, im)
	case "gif":
		opts := gif.Options{}
		err = gif.Encode(wr, im, &opts)
	case "tiff":
		opts := tiff.Options{}
		err = tiff.Encode(wr, im, &opts)
	default:
		err = errors.New("Unsupported encoding")
	}

	if err != nil {
		return nil, err
	}

	wr.Flush()

	return b.Bytes(), nil
}
