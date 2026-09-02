package imaging

import (
	"bytes"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/chai2010/webp"
	dimg "github.com/disintegration/imaging"
)

// Format 通过魔数嗅探返回图片格式：jpeg|png|gif|webp|svg，无法识别返回空串。
func Format(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpeg"
	case len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return "png"
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return "gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp"
	case bytes.Contains(bytes.ToLower(b[:min(len(b), 256)]), []byte("<svg")):
		return "svg"
	default:
		return ""
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Process 缩放超限图片并可选转 WebP，返回处理后的字节与新宽高。
// format 为 jpeg|png|webp；gif/svg 不应调用本函数（原样存储）。
func Process(data []byte, format string, maxDimension int, convertWebp bool) ([]byte, int, int, error) {
	src, err := dimg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := src.Bounds()
	if maxDimension > 0 && (bounds.Dx() > maxDimension || bounds.Dy() > maxDimension) {
		src = dimg.Fit(src, maxDimension, maxDimension, dimg.Lanczos)
		bounds = src.Bounds()
	}

	var out bytes.Buffer
	if convertWebp {
		if err := webp.Encode(&out, src, &webp.Options{Lossless: false, Quality: 82}); err != nil {
			return nil, 0, 0, err
		}
	} else {
		switch format {
		case "jpeg":
			err = dimg.Encode(&out, src, dimg.JPEG, dimg.JPEGQuality(85))
		case "png":
			err = dimg.Encode(&out, src, dimg.PNG)
		case "webp":
			err = webp.Encode(&out, src, &webp.Options{Lossless: false, Quality: 82})
		default:
			err = dimg.Encode(&out, src, dimg.PNG)
		}
		if err != nil {
			return nil, 0, 0, err
		}
	}
	return out.Bytes(), bounds.Dx(), bounds.Dy(), nil
}
