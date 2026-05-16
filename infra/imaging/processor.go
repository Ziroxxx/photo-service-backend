package imaging

import (
	"context"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"photo-service-back/domain/photo"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/sync/errgroup"
)

type Processor struct {
	PreviewMaxWidth   int
	PreviewMaxHeight  int
	WatermarkText     string
	WatermarkImage    image.Image
	JPEGQuality       int
	WatermarkMaxRatio float64
}

func NewProcessor(
	previewMaxWidth,
	previewMaxHeight int,
	watermarkText string,
	jpegQuality int,
	watermarkImagePath string,
) *Processor {
	if previewMaxWidth <= 0 {
		previewMaxWidth = 1600
	}
	if previewMaxHeight <= 0 {
		previewMaxHeight = 1600
	}
	if watermarkText == "" {
		watermarkText = "Photo Service"
	}
	if jpegQuality <= 0 || jpegQuality > 100 {
		jpegQuality = 85
	}

	var wm image.Image
	if strings.TrimSpace(watermarkImagePath) != "" {
		loaded, err := loadImageFromPath(watermarkImagePath)
		if err == nil {
			wm = loaded
		}
	}

	return &Processor{
		PreviewMaxWidth:   previewMaxWidth,
		PreviewMaxHeight:  previewMaxHeight,
		WatermarkText:     watermarkText,
		WatermarkImage:    wm,
		JPEGQuality:       jpegQuality,
		WatermarkMaxRatio: 0.22, // watermark ≈ 22% ширины изображения
	}
}

func (p *Processor) Process(ctx context.Context, input photo.ProcessInput) (*photo.ProcessedPhoto, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	f, err := os.Open(input.SourcePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, photo.ErrInvalidImage
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	origStat, err := os.Stat(input.SourcePath)
	if err != nil {
		return nil, err
	}

	originalMime := detectOriginalMime(format, input.DeclaredMimeType)

	result := &photo.ProcessedPhoto{
		Original: photo.ProcessedVariant{
			Variant:      photo.VariantOriginal,
			TempFilePath: input.SourcePath,
			MimeType:     originalMime,
			SizeBytes:    origStat.Size(),
			Width:        width,
			Height:       height,
		},
	}

	watermarkedPath, err := createTempJPEG("photo-watermarked-*.jpg")
	if err != nil {
		return nil, err
	}
	previewPath, err := createTempJPEG("photo-preview-*.jpg")
	if err != nil {
		_ = os.Remove(watermarkedPath)
		return nil, err
	}

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		wm := p.applyWatermark(img)
		if err := p.writeJPEG(watermarkedPath, wm); err != nil {
			return err
		}
		stat, err := os.Stat(watermarkedPath)
		if err != nil {
			return err
		}
		result.Watermarked = photo.ProcessedVariant{
			Variant:      photo.VariantWatermarked,
			TempFilePath: watermarkedPath,
			MimeType:     "image/jpeg",
			SizeBytes:    stat.Size(),
			Width:        width,
			Height:       height,
		}
		return nil
	})

	g.Go(func() error {
		resized := resizeToFit(img, p.PreviewMaxWidth, p.PreviewMaxHeight)
		wmPreview := p.applyWatermark(resized)
		if err := p.writeJPEG(previewPath, wmPreview); err != nil {
			return err
		}
		stat, err := os.Stat(previewPath)
		if err != nil {
			return err
		}
		pb := wmPreview.Bounds()
		result.Preview = photo.ProcessedVariant{
			Variant:      photo.VariantPreview,
			TempFilePath: previewPath,
			MimeType:     "image/jpeg",
			SizeBytes:    stat.Size(),
			Width:        pb.Dx(),
			Height:       pb.Dy(),
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		_ = os.Remove(watermarkedPath)
		_ = os.Remove(previewPath)
		return nil, err
	}

	return result, nil
}

func (p *Processor) writeJPEG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return jpeg.Encode(f, img, &jpeg.Options{Quality: p.JPEGQuality})
}

func (p *Processor) applyWatermark(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	stddraw.Draw(dst, b, src, b.Min, stddraw.Src)

	// Если watermark-картинка загружена — используем её.
	if p.WatermarkImage != nil {
		wm := resizeWatermarkToFit(src.Bounds(), p.WatermarkImage, p.WatermarkMaxRatio)

		wmBounds := wm.Bounds()
		padding := 20

		x := b.Max.X - wmBounds.Dx() - padding
		y := b.Max.Y - wmBounds.Dy() - padding

		if x < b.Min.X+10 {
			x = b.Min.X + 10
		}
		if y < b.Min.Y+10 {
			y = b.Min.Y + 10
		}

		target := image.Rect(x, y, x+wmBounds.Dx(), y+wmBounds.Dy())
		stddraw.Draw(dst, target, wm, wmBounds.Min, stddraw.Over)
		return dst
	}

	// Fallback на старый текстовый watermark, если картинка не загрузилась.
	text := p.WatermarkText
	if strings.TrimSpace(text) == "" {
		text = "Photo Service"
	}

	textWidth := len(text) * 7
	textHeight := 13

	paddingX := 12
	paddingY := 10

	rectWidth := textWidth + paddingX*2
	rectHeight := textHeight + paddingY*2

	x0 := b.Max.X - rectWidth - 20
	y0 := b.Max.Y - rectHeight - 20
	if x0 < b.Min.X+10 {
		x0 = b.Min.X + 10
	}
	if y0 < b.Min.Y+10 {
		y0 = b.Min.Y + 10
	}

	bg := image.Rect(x0, y0, x0+rectWidth, y0+rectHeight)
	stddraw.Draw(dst, bg, &image.Uniform{C: color.NRGBA{R: 0, G: 0, B: 0, A: 140}}, image.Point{}, stddraw.Over)

	// Можно оставить пусто, либо вернуть сюда старую текстовую отрисовку.
	return dst
}

func resizeToFit(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()

	if w <= maxW && h <= maxH {
		return src
	}

	ratioW := float64(maxW) / float64(w)
	ratioH := float64(maxH) / float64(h)
	ratio := ratioW
	if ratioH < ratio {
		ratio = ratioH
	}

	newW := int(float64(w) * ratio)
	newH := int(float64(h) * ratio)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

func resizeWatermarkToFit(photoBounds image.Rectangle, watermark image.Image, maxRatio float64) image.Image {
	if maxRatio <= 0 || maxRatio >= 1 {
		maxRatio = 0.22
	}

	pw := photoBounds.Dx()
	ph := photoBounds.Dy()

	wb := watermark.Bounds()
	ww := wb.Dx()
	wh := wb.Dy()

	if ww <= 0 || wh <= 0 {
		return watermark
	}

	maxW := int(float64(pw) * maxRatio)
	maxH := int(float64(ph) * maxRatio)

	scaleW := float64(maxW) / float64(ww)
	scaleH := float64(maxH) / float64(wh)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	// Не увеличиваем watermark, только уменьшаем.
	if scale >= 1 {
		return watermark
	}

	newW := int(float64(ww) * scale)
	newH := int(float64(wh) * scale)

	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), watermark, wb, xdraw.Over, nil)
	return dst
}

func loadImageFromPath(path string) (image.Image, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func detectOriginalMime(format, declared string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		if declared != "" {
			return declared
		}
		return "application/octet-stream"
	}
}

func createTempJPEG(pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, nil
}

var (
	_ = gif.GIF{}
	_ = png.Encoder{}
)
