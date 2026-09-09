// File: infrastructure/onenote/ink.go
package onenote

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
)

type inkPoint struct {
	X, Y float64
}

type inkTrace struct {
	Points                 []inkPoint
	MinX, MinY, MaxX, MaxY float64
}

func (c *Client) fetchPageHTMLAndInk(ctx context.Context, client *http.Client, pageID string) (string, []byte, error) {
	endpoint := fmt.Sprintf("%s/pages/%s/content?includeIDs=true&includeInkML=true", graphAPIBase, url.PathEscape(pageID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "multipart/form-data, application/inkml+xml, text/html, application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("lấy nội dung trang OneNote: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", nil, fmt.Errorf("OneNote API lỗi (%d): %s", resp.StatusCode, string(body))
	}

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		reader := multipart.NewReader(resp.Body, params["boundary"])
		var htmlContent string
		var inkContent []byte

		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				break
			}

			partType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			formName := strings.ToLower(part.FormName())
			data, _ := io.ReadAll(part)
			part.Close()

			if formName == "presentation" || strings.Contains(partType, "html") {
				htmlContent = string(data)
			} else if formName == "inkml" || strings.Contains(partType, "inkml") || strings.Contains(partType, "xml") {
				inkContent = data
			}
		}
		return htmlContent, inkContent, nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	return string(body), nil, nil
}
// RenderInkMLToJPEG tạo ra 1 bức ảnh dài toàn bộ nét vẽ tay của học sinh trên nền trắng
func RenderInkMLToJPEG(inkXML []byte) ([]byte, error) {
	traces, err := parseInkTraces(inkXML)
	if err != nil || len(traces) == 0 {
		return nil, err
	}

	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, t := range traces {
		minX = math.Min(minX, t.MinX)
		minY = math.Min(minY, t.MinY)
		maxX = math.Max(maxX, t.MaxX)
		maxY = math.Max(maxY, t.MaxY)
	}

	padding := 30.0
	contentWidth := (maxX - minX) / 26.4583  // Chuyển Himetric sang Pixel
	contentHeight := (maxY - minY) / 26.4583

	// Chiều rộng cố định khổ ~850px chuẩn đọc tài liệu
	targetWidth := 850.0
	scale := 1.0
	if contentWidth > 0 {
		scale = targetWidth / contentWidth
		if scale > 1.2 {
			scale = 1.2
		}
	}

	width := int(math.Ceil(contentWidth*scale + padding*2))
	height := int(math.Ceil(contentHeight*scale + padding*2))

	if width < 300 { width = 300 }
	if height < 200 { height = 200 }

	dc := gg.NewContext(width, height)
	dc.SetRGB(1, 1, 1) // Nền trắng tờ giấy
	dc.Clear()
	dc.SetRGB(0, 0.08, 0.35) // Mực viết xanh đậm OneNote (#00145a)
	dc.SetLineWidth(2.4)     // Độ dày nét bút vừa vặn, không bị mảnh
	dc.SetLineCap(gg.LineCapRound)
	dc.SetLineJoin(gg.LineJoinRound)

	for _, t := range traces {
		if len(t.Points) == 0 {
			continue
		}
		startX := ((t.Points[0].X-minX)/26.4583)*scale + padding
		startY := ((t.Points[0].Y-minY)/26.4583)*scale + padding
		dc.MoveTo(startX, startY)

		for _, p := range t.Points[1:] {
			curX := ((p.X-minX)/26.4583)*scale + padding
			curY := ((p.Y-minY)/26.4583)*scale + padding
			dc.LineTo(curX, curY)
		}
		if len(t.Points) == 1 {
			dc.DrawCircle(startX, startY, 1.5)
			dc.Fill()
		} else {
			dc.Stroke()
		}
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 90})
	return buf.Bytes(), err
}

func parseInkTraces(data []byte) ([]inkTrace, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var traces []inkTrace

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if ok && start.Name.Local == "trace" {
			var raw string
			_ = decoder.DecodeElement(&raw, &start)
			pts := parseTracePoints(raw)
			if len(pts) > 0 {
				t := inkTrace{
					Points: pts,
					MinX:   math.MaxFloat64, MinY: math.MaxFloat64,
					MaxX:   -math.MaxFloat64, MaxY: -math.MaxFloat64,
				}
				for _, p := range pts {
					t.MinX = math.Min(t.MinX, p.X)
					t.MinY = math.Min(t.MinY, p.Y)
					t.MaxX = math.Max(t.MaxX, p.X)
					t.MaxY = math.Max(t.MaxY, p.Y)
				}
				traces = append(traces, t)
			}
		}
	}
	return traces, nil
}

func parseTracePoints(raw string) []inkPoint {
	clean := strings.NewReplacer(",", " ", ";", " ", "\n", " ", "\r", " ", "\t", " ").Replace(strings.TrimSpace(raw))
	fields := strings.Fields(clean)
	if len(fields) < 2 {
		return nil
	}

	var pts []inkPoint
	var prev inkPoint

	for i := 0; i+1 < len(fields); i += 2 {
		rawX, rawY := fields[i], fields[i+1]
		isDiff := strings.HasPrefix(rawX, "'") || strings.HasPrefix(rawX, "\"")
		rawX = strings.TrimLeft(rawX, "'\"!")
		rawY = strings.TrimLeft(rawY, "'\"!")

		x, errX := strconv.ParseFloat(rawX, 64)
		y, errY := strconv.ParseFloat(rawY, 64)
		if errX != nil || errY != nil {
			continue
		}

		if isDiff && len(pts) > 0 {
			x += prev.X
			y += prev.Y
		}

		p := inkPoint{X: x, Y: y}
		pts = append(pts, p)
		prev = p
	}
	return pts
}