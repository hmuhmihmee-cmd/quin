package onenote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

const graphBetaOneNoteAPIBase = "https://graph.microsoft.com/beta/me/onenote"

var cssTopPattern = regexp.MustCompile(`(?i)(?:^|;)\s*top\s*:\s*(-?[0-9]+(?:\.[0-9]+)?)\s*(px|pt)?`)

func (c *Client) fetchPageHTMLAndInk(ctx context.Context, client *http.Client, pageID string) (string, []byte, error) {
	betaEndpoint := fmt.Sprintf("%s/pages/%s/content?includeIDs=true&includeInkML=true", graphBetaOneNoteAPIBase, url.PathEscape(pageID))
	betaHTML, betaInk, betaErr := requestPageContent(ctx, client, betaEndpoint)
	if betaErr == nil && strings.TrimSpace(betaHTML) != "" && len(betaInk) > 0 {
		return betaHTML, betaInk, nil
	}
	if betaErr == nil && strings.TrimSpace(betaHTML) != "" {
		return betaHTML, nil, nil
	}

	// Một số response beta chỉ chứa InkML hoặc beta có thể tạm lỗi. Lấy HTML
	// qua Graph v1.0 để không làm mất chữ gõ và cấu trúc vùng từng câu.
	stableEndpoint := fmt.Sprintf("%s/pages/%s/content?includeIDs=true", graphAPIBase, url.PathEscape(pageID))
	htmlContent, _, stableErr := requestPageContent(ctx, client, stableEndpoint)
	if stableErr != nil {
		if betaErr != nil {
			return "", nil, fmt.Errorf("Graph beta InkML: %v; Graph v1.0 HTML: %w", betaErr, stableErr)
		}
		return "", nil, stableErr
	}
	return htmlContent, betaInk, nil
}

func requestPageContent(ctx context.Context, client *http.Client, endpoint string) (string, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "multipart/form-data, application/inkml+xml, text/html, application/xhtml+xml, */*")
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("lấy nội dung trang OneNote: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", nil, fmt.Errorf("OneNote content API (%d): %s", resp.StatusCode, string(body))
	}

	mediaType, parameters, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		boundary := parameters["boundary"]
		if boundary == "" {
			return "", nil, errors.New("OneNote multipart response thiếu boundary")
		}
		reader := multipart.NewReader(resp.Body, boundary)
		var htmlContent string
		var inkContent []byte
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return "", nil, fmt.Errorf("đọc OneNote multipart: %w", err)
			}
			formName := strings.ToLower(strings.TrimSpace(part.FormName()))
			partType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			partType = strings.ToLower(partType)
			limit := int64(25 << 20)
			if formName == "presentation" || partType == "text/html" || partType == "application/xhtml+xml" {
				limit = 10 << 20
			}
			data, readErr := io.ReadAll(io.LimitReader(part, limit+1))
			part.Close()
			if readErr != nil {
				return "", nil, readErr
			}
			if int64(len(data)) > limit {
				return "", nil, fmt.Errorf("OneNote part %s vượt giới hạn %d MB", partType, limit>>20)
			}
			switch {
			case formName == "presentation", partType == "text/html", partType == "application/xhtml+xml":
				htmlContent = string(data)
			case formName == "inkml", formName == "presentation-onenote-inkml", strings.Contains(partType, "inkml"), partType == "text/xml", partType == "application/xml":
				inkContent = data
			}
		}
		return htmlContent, inkContent, nil
	}

	limit := int64(10 << 20)
	if isInkMediaType(mediaType) {
		limit = 25 << 20
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return "", nil, err
	}
	if int64(len(body)) > limit {
		return "", nil, fmt.Errorf("nội dung trang OneNote vượt quá %d MB", limit>>20)
	}
	if isInkMediaType(mediaType) || (mediaType == "" && looksLikeInkML(body)) {
		return "", body, nil
	}
	return string(body), nil, nil
}

func isInkMediaType(mediaType string) bool {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return strings.Contains(mediaType, "inkml") || mediaType == "text/xml" || mediaType == "application/xml"
}

func looksLikeInkML(data []byte) bool {
	content := strings.ToLower(strings.TrimSpace(string(data)))
	return strings.HasPrefix(content, "<ink") || strings.Contains(content, "<ink:ink")
}

func extractInkExerciseBoundaries(pageHTML string, exerciseIDs []int) []inkExerciseBoundary {
	document, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	wanted := make(map[string]int, len(exerciseIDs))
	for _, exerciseID := range exerciseIDs {
		wanted[fmt.Sprintf("exercise-%d-region", exerciseID)] = exerciseID
		wanted[fmt.Sprintf("exercise-%d-work", exerciseID)] = exerciseID
	}
	found := make(map[int]float64, len(exerciseIDs))
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			dataID := nodeAttribute(node, "data-id")
			exerciseID, matches := wanted[dataID]
			if matches {
				for current := node; current != nil; current = current.Parent {
					if top, ok := parseCSSTop(nodeAttribute(current, "style")); ok {
						if _, alreadyFound := found[exerciseID]; !alreadyFound {
							found[exerciseID] = top
						}
						break
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	boundaries := make([]inkExerciseBoundary, 0, len(found))
	uniqueY := make(map[int64]struct{}, len(found))
	for _, exerciseID := range exerciseIDs {
		y, ok := found[exerciseID]
		if !ok {
			continue
		}
		boundaries = append(boundaries, inkExerciseBoundary{ExerciseID: exerciseID, Y: y})
		uniqueY[int64(y)] = struct{}{}
	}
	// Một outline cha duy nhất có cùng top cho mọi câu không tạo được ranh giới.
	if len(uniqueY) < 2 && len(exerciseIDs) > 1 {
		return nil
	}
	return boundaries
}

func nodeAttribute(node *xhtml.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func parseCSSTop(style string) (float64, bool) {
	match := cssTopPattern.FindStringSubmatch(style)
	if len(match) == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	// InkML OneNote dùng himetric: 1 inch = 2540 đơn vị.
	switch strings.ToLower(match[2]) {
	case "pt":
		return value * (2540.0 / 72.0), true
	default: // CSS px ở 96 DPI
		return value * (2540.0 / 96.0), true
	}
}
