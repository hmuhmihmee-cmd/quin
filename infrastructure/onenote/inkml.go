package onenote

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
)

type inkPoint struct {
	X float64
	Y float64
}

type inkTrace struct {
	Points []inkPoint
	MinX   float64
	MinY   float64
	MaxX   float64
	MaxY   float64
}

type inkExerciseBoundary struct {
	ExerciseID int
	Y          float64
}

type inkTraceLayout struct {
	Stride int
	XIndex int
	YIndex int
}

// parseInkMLTraces dùng XML token stream để lấy cả trace trực tiếp lẫn trace
// nằm trong traceGroup. Namespace prefix của InkML không ảnh hưởng vì Go so
// khớp theo local name.
func parseInkMLTraces(data []byte) ([]inkTrace, error) {
	layout := parseInkTraceLayout(data)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	traces := make([]inkTrace, 0)
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse InkML: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "trace" {
			continue
		}
		var raw string
		if err := decoder.DecodeElement(&raw, &start); err != nil {
			return nil, fmt.Errorf("parse InkML trace: %w", err)
		}
		points := parseInkTracePointsWithLayout(raw, layout)
		if len(points) == 0 {
			continue
		}
		trace := inkTrace{
			Points: points,
			MinX:   math.MaxFloat64,
			MinY:   math.MaxFloat64,
			MaxX:   -math.MaxFloat64,
			MaxY:   -math.MaxFloat64,
		}
		for _, point := range points {
			trace.MinX = math.Min(trace.MinX, point.X)
			trace.MinY = math.Min(trace.MinY, point.Y)
			trace.MaxX = math.Max(trace.MaxX, point.X)
			trace.MaxY = math.Max(trace.MaxY, point.Y)
		}
		traces = append(traces, trace)
	}
	return traces, nil
}

func parseInkTracePoints(value string) []inkPoint {
	return parseInkTracePointsWithLayout(value, inkTraceLayout{})
}

func parseInkTraceLayout(data []byte) inkTraceLayout {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return inkTraceLayout{}
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "traceFormat" {
			continue
		}
		channels := make([]string, 0, 3)
		depth := 1
		for depth > 0 {
			token, err = decoder.Token()
			if err != nil {
				return inkTraceLayout{}
			}
			switch element := token.(type) {
			case xml.StartElement:
				depth++
				if element.Name.Local == "channel" {
					for _, attribute := range element.Attr {
						if attribute.Name.Local == "name" {
							channels = append(channels, strings.ToUpper(strings.TrimSpace(attribute.Value)))
							break
						}
					}
				}
			case xml.EndElement:
				depth--
			}
		}
		xIndex, yIndex := -1, -1
		for index, channel := range channels {
			switch channel {
			case "X":
				xIndex = index
			case "Y":
				yIndex = index
			}
		}
		if xIndex >= 0 && yIndex >= 0 {
			return inkTraceLayout{Stride: len(channels), XIndex: xIndex, YIndex: yIndex}
		}
	}
}

func parseInkTracePointsWithLayout(value string, layout inkTraceLayout) []inkPoint {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	// OneNote có thể phân tách từng điểm bằng dấu phẩy, chấm phẩy hoặc xuống
	// dòng. Nếu không có các dấu này, traceFormat quyết định stride X Y [F...];
	// trang không khai báo traceFormat được đọc theo từng cặp X Y.
	groups := strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || character == ';' || character == '\n' || character == '\r'
	})
	points := make([]inkPoint, 0)
	appendPoint := func(rawX, rawY string) {
		mode := byte(0)
		if rawX != "" && strings.ContainsRune("'\"!", rune(rawX[0])) {
			mode = rawX[0]
		}
		rawX = strings.TrimLeft(rawX, "'\"!")
		rawY = strings.TrimLeft(rawY, "'\"!")
		x, errX := strconv.ParseFloat(rawX, 64)
		y, errY := strconv.ParseFloat(rawY, 64)
		if errX != nil || errY != nil {
			return
		}
		point := inkPoint{X: x, Y: y}
		switch mode {
		case '\'': // first-order difference
			if len(points) > 0 {
				previous := points[len(points)-1]
				point.X += previous.X
				point.Y += previous.Y
			}
		case '"': // second-order difference
			if len(points) > 1 {
				previous := points[len(points)-1]
				beforePrevious := points[len(points)-2]
				point.X += 2*previous.X - beforePrevious.X
				point.Y += 2*previous.Y - beforePrevious.Y
			} else if len(points) == 1 {
				point.X += points[0].X
				point.Y += points[0].Y
			}
		}
		points = append(points, point)
	}

	if len(groups) > 1 {
		for _, group := range groups {
			fields := strings.Fields(group)
			if len(fields) < 2 {
				continue
			}
			xIndex, yIndex := 0, 1
			if layout.Stride > 0 && layout.XIndex < len(fields) && layout.YIndex < len(fields) {
				xIndex, yIndex = layout.XIndex, layout.YIndex
			}
			appendPoint(fields[xIndex], fields[yIndex])
		}
		return points
	}

	fields := strings.Fields(value)
	stride, xIndex, yIndex := 2, 0, 1
	if layout.Stride >= 2 {
		stride, xIndex, yIndex = layout.Stride, layout.XIndex, layout.YIndex
	}
	for offset := 0; offset+stride <= len(fields); offset += stride {
		appendPoint(fields[offset+xIndex], fields[offset+yIndex])
	}
	return points
}

func splitAndRenderInkPerExercise(data []byte, boundaries []inkExerciseBoundary, exerciseIDs []int) (map[int][]byte, error) {
	traces, err := parseInkMLTraces(data)
	if err != nil || len(traces) == 0 {
		return nil, err
	}
	result := make(map[int][]byte)
	groups := make(map[int][]inkTrace)

	validBoundaries := append([]inkExerciseBoundary(nil), boundaries...)
	sort.Slice(validBoundaries, func(i, j int) bool { return validBoundaries[i].Y < validBoundaries[j].Y })
	if len(validBoundaries) >= 2 {
		for _, trace := range traces {
			midY := (trace.MinY + trace.MaxY) / 2
			selected := validBoundaries[0].ExerciseID
			for _, boundary := range validBoundaries {
				if midY < boundary.Y {
					break
				}
				selected = boundary.ExerciseID
			}
			groups[selected] = append(groups[selected], trace)
		}
	} else {
		// Khi HTML chỉ có một outline chung, không đủ dữ liệu để biết nét nào
		// thuộc câu nào. Giữ toàn bộ bài viết tay cho mọi câu để AI vẫn nhìn thấy
		// bài làm, thay vì làm mất ảnh của các câu sau.
		fullPageImage, err := renderInkTracesToJPEG(traces)
		if err != nil {
			return nil, fmt.Errorf("render toàn bộ Ink trang: %w", err)
		}
		for _, exerciseID := range exerciseIDs {
			if len(fullPageImage) > 0 {
				result[exerciseID] = fullPageImage
			}
		}
		return result, nil
	}

	for exerciseID, exerciseTraces := range groups {
		image, err := renderInkTracesToJPEG(exerciseTraces)
		if err != nil {
			return nil, fmt.Errorf("render Ink câu %d: %w", exerciseID, err)
		}
		if len(image) > 0 {
			result[exerciseID] = image
		}
	}
	return result, nil
}

func renderInkTracesToJPEG(traces []inkTrace) ([]byte, error) {
	if len(traces) == 0 {
		return nil, nil
	}
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, trace := range traces {
		minX = math.Min(minX, trace.MinX)
		minY = math.Min(minY, trace.MinY)
		maxX = math.Max(maxX, trace.MaxX)
		maxY = math.Max(maxY, trace.MaxY)
	}
	if minX == math.MaxFloat64 {
		return nil, nil
	}

	const padding = 30.0
	contentWidth := maxX - minX
	contentHeight := maxY - minY
	scale := math.Min(1, math.Min(1400/contentWidth, 1800/contentHeight))
	width := int(math.Ceil(contentWidth*scale + padding*2))
	height := int(math.Ceil(contentHeight*scale + padding*2))
	if width < 200 {
		width = 200
	}
	if height < 100 {
		height = 100
	}

	drawing := gg.NewContext(width, height)
	drawing.SetRGB(1, 1, 1)
	drawing.Clear()
	drawing.SetRGB(0, 0.08, 0.35)
	drawing.SetLineWidth(math.Max(2, 2.5*scale))
	drawing.SetLineCap(gg.LineCapRound)
	drawing.SetLineJoin(gg.LineJoinRound)
	for _, trace := range traces {
		if len(trace.Points) == 0 {
			continue
		}
		drawing.MoveTo((trace.Points[0].X-minX)*scale+padding, (trace.Points[0].Y-minY)*scale+padding)
		for _, point := range trace.Points[1:] {
			drawing.LineTo((point.X-minX)*scale+padding, (point.Y-minY)*scale+padding)
		}
		if len(trace.Points) == 1 {
			drawing.DrawCircle((trace.Points[0].X-minX)*scale+padding, (trace.Points[0].Y-minY)*scale+padding, 1.5)
			drawing.Fill()
		} else {
			drawing.Stroke()
		}
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, drawing.Image(), &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
