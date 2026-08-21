// File: infrastructure/onenote/client.go
package onenote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	"meet-attendance-clean/infrastructure/html"
	pdfformat "meet-attendance-clean/infrastructure/pdf"

	xhtml "golang.org/x/net/html"
	"golang.org/x/oauth2"
)

const graphAPIBase = "https://graph.microsoft.com/v1.0/me/onenote"

type Client struct {
	config        *oauth2.Config
	tokenPath     string
	htmlConverter *html.Converter // Cắm converter vào
	formatter     *pdfformat.Formatter
}

var (
	exerciseHeadingPattern = regexp.MustCompile(`(?i)^\*\*Câu\s+(\d+)\b`)
	htmlTagPattern         = regexp.MustCompile(`(?s)<[^>]*>`)
	unsafeHTMLBlockPattern = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
	unsafeHTMLAttrPattern  = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*')`)
	verboseFeedbackPattern = regexp.MustCompile(`(?is)<p[^>]*>\s*(?:rất tiếc|em đã chọn|câu trả lời của em|đáp án em chọn)[^<]*</p>`)
)

func New(clientID, clientSecret, redirectURL, tokenPath string) *Client {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		},
		Scopes: []string{"Notes.ReadWrite", "offline_access", "User.Read"},
	}
	return &Client{
		config:        cfg,
		tokenPath:     tokenPath,
		htmlConverter: html.New(),
		formatter:     pdfformat.NewFormatter(),
	}
}

// ==================== 1. XÁC THỰC OAUTH2 ====================

func (c *Client) IsConnected() bool {
	_, err := os.Stat(c.tokenPath)
	return err == nil
}

func (c *Client) AuthorizationURL(state string) string {
	return c.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "select_account"))
}

func (c *Client) Exchange(ctx context.Context, code string) error {
	token, err := c.config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("đổi token Microsoft: %w", err)
	}
	file, err := os.OpenFile(c.tokenPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(token)
}

func (c *Client) Disconnect() error {
	if err := os.Remove(c.tokenPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (c *Client) getHTTPClient(ctx context.Context) (*http.Client, error) {
	file, err := os.Open(c.tokenPath)
	if err != nil {
		return nil, errors.New("chưa liên kết tài khoản Microsoft OneNote")
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("đọc token Microsoft: %w", err)
	}

	tokenSource := c.config.TokenSource(ctx, &token)
	freshToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("phiên đăng nhập Microsoft đã hết hạn: %w", err)
	}

	if freshToken.AccessToken != token.AccessToken {
		_ = c.saveToken(freshToken)
	}

	return oauth2.NewClient(ctx, tokenSource), nil
}

func (c *Client) saveToken(token *oauth2.Token) error {
	file, err := os.OpenFile(c.tokenPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(token)
}

// ==================== 2. IMPLEMENT WorkspaceGateway ====================

func (c *Client) ListWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/notebooks?$expand=sections($select=id,displayName)&$select=id,displayName", graphAPIBase)

	var resp struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Sections    []struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"sections"`
		} `json:"value"`
	}

	if err := c.getJSON(ctx, client, endpoint, &resp); err != nil {
		return nil, fmt.Errorf("lấy danh sách OneNote: %w", err)
	}

	workspaces := make([]domain.Workspace, 0, len(resp.Value))
	for _, nb := range resp.Value {
		chapters := make([]domain.Chapter, 0, len(nb.Sections))
		for _, sec := range nb.Sections {
			chapters = append(chapters, domain.Chapter{
				ID:   sec.ID,
				Name: sec.DisplayName,
			})
		}

		workspaces = append(workspaces, domain.Workspace{
			ID:       nb.ID,
			Name:     nb.DisplayName,
			Chapters: chapters,
		})
	}

	return workspaces, nil
}

func (c *Client) PublishSession(ctx context.Context, target application.WorkspaceTarget, audience domain.Audience, lesson domain.Lesson) (string, string, error) {
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return "", "", err
	}

	workspaceID, err := c.resolveNotebook(ctx, client, target)
	if err != nil {
		return "", "", fmt.Errorf("chuẩn bị Notebook: %w", err)
	}
	sectionID, err := c.resolveSection(ctx, client, workspaceID, target)
	if err != nil {
		return "", "", fmt.Errorf("chuẩn bị Section: %w", err)
	}

	markdown := c.formatter.Format(&lesson, audience, "")
	if audience == domain.AudienceStudent {
		markdown = addExerciseMarkers(markdown)
	}
	pageName := strings.TrimSpace(pointerValue(target.PageName))
	htmlPayload := c.htmlConverter.MarkdownToDocument(pageName, markdown)

	// Mỗi lần publish luôn tạo một Page mới.
	endpoint := fmt.Sprintf("%s/sections/%s/pages", graphAPIBase, url.PathEscape(sectionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(htmlPayload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "text/html; charset=utf-8")

	res, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("lỗi gửi request tạo trang OneNote: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return "", "", fmt.Errorf("OneNote API (%d): %s", res.StatusCode, string(body))
	}

	var pageResp struct {
		ID    string `json:"id"`
		Links struct {
			OneNoteWebUrl struct {
				Href string `json:"href"`
			} `json:"oneNoteWebUrl"`
		} `json:"links"`
	}
	_ = json.NewDecoder(res.Body).Decode(&pageResp)

	// HẠ TẦNG MỚI: Assignment cần PageID thật để GET/PATCH nội dung về sau;
	// URL web chỉ dùng để mở trang và không thể thay cho ID trong Graph API.
	if pageResp.ID == "" {
		return "", "", errors.New("OneNote tạo trang thành công nhưng không trả page ID")
	}
	return pageResp.ID, workspaceID, nil
}

// ==================== HẠ TẦNG MỚI: OneNoteGateway chấm bài ====================

func (c *Client) PublishStudentLesson(ctx context.Context, target application.WorkspaceTarget, lesson *domain.Lesson) (string, error) {
	if lesson == nil {
		return "", errors.New("lesson không được nil")
	}
	pageID, _, err := c.PublishSession(ctx, target, domain.AudienceStudent, *lesson)
	return pageID, err
}

func (c *Client) FetchStudentAnswers(ctx context.Context, assignment *domain.Assignment) error {
	if assignment == nil || strings.TrimSpace(assignment.TargetPageID) == "" {
		return errors.New("assignment thiếu OneNote page ID")
	}
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return err
	}
	content, inkML, err := c.fetchPageHTMLAndInk(ctx, client, assignment.TargetPageID)
	if err != nil {
		return err
	}

	exerciseIDs := make([]int, 0, len(assignment.Items))
	for i := range assignment.Items {
		exerciseIDs = append(exerciseIDs, assignment.Items[i].Exercise.ID)
	}
	inkImages := make(map[int][]byte)
	if len(inkML) > 0 {
		boundaries := extractInkExerciseBoundaries(content, exerciseIDs)
		inkImages, err = splitAndRenderInkPerExercise(inkML, boundaries, exerciseIDs)
		if err != nil {
			return fmt.Errorf("đọc nét bút OneNote: %w", err)
		}
	}
	return c.populateStudentAnswers(ctx, client, assignment, content, inkImages, time.Now().UTC())
}

func (c *Client) populateStudentAnswers(
	ctx context.Context,
	client *http.Client,
	assignment *domain.Assignment,
	content string,
	inkImages map[int][]byte,
	extractedAt time.Time,
) error {
	for i := range assignment.Items {
		exerciseID := assignment.Items[i].Exercise.ID
		answer := &domain.StudentAnswer{IsBlank: true, ExtractedAt: extractedAt}
		answerHTML, found := extractAnswerBlock(content, exerciseID)
		if found {
			parsedAnswer, err := c.parseStudentAnswer(ctx, client, answerHTML, extractedAt)
			if err != nil {
				return fmt.Errorf("đọc bài làm câu %d: %w", exerciseID, err)
			}
			answer = parsedAnswer
		}
		// Ảnh có thể được học sinh đặt cạnh vùng bài làm thay vì nằm trong ô
		// answer. Trang mới có wrapper cho toàn bộ câu để vẫn thu được các ảnh đó.
		if regionHTML, regionFound := extractElementInnerByID(content, fmt.Sprintf("exercise-%d-region", exerciseID)); regionFound {
			region, err := c.parseStudentAnswer(ctx, client, regionHTML, extractedAt)
			if err != nil {
				return fmt.Errorf("đọc ảnh trong vùng câu %d: %w", exerciseID, err)
			}
			for _, image := range region.Images {
				answer.Images = appendUniqueImage(answer.Images, image)
			}
		}
		if inkImage := inkImages[exerciseID]; len(inkImage) > 0 {
			answer.Images = appendUniqueImage(answer.Images, inkImage)
		}
		answer.IsBlank = answer.IsBlank && len(answer.Images) == 0
		assignment.Items[i].StudentAnswer = answer
	}
	return nil
}

func (c *Client) PatchFeedback(ctx context.Context, assignment *domain.Assignment) error {
	if assignment == nil || strings.TrimSpace(assignment.TargetPageID) == "" {
		return errors.New("assignment thiếu OneNote page ID")
	}
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return err
	}
	pageHTML, err := c.fetchPageHTML(ctx, client, assignment.TargetPageID)
	if err != nil {
		return err
	}
	commands, err := buildFeedbackPatchCommands(pageHTML, assignment)
	if err != nil {
		return err
	}
	if len(commands) == 0 {
		return nil
	}
	payload, err := json.Marshal(commands)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/pages/%s/content", graphAPIBase, url.PathEscape(assignment.TargetPageID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("đẩy nhận xét lên OneNote: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("OneNote PATCH API (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func buildFeedbackPatchCommands(pageHTML string, assignment *domain.Assignment) ([]map[string]string, error) {
	legacyTableTargets := findGeneratedTableIDs(pageHTML)

	commands := make([]map[string]string, 0, len(assignment.Items))
	for itemIndex, item := range assignment.Items {
		if item.Result == nil {
			continue
		}
		// Không replace vùng làm bài của câu bỏ trống. Học sinh phải tiếp tục
		// viết được vào đúng khung ban đầu và có thể yêu cầu chấm lại sau đó.
		if item.Result.Status == domain.AIExerciseSkippedEmpty {
			continue
		}
		exerciseID := strconv.Itoa(item.Exercise.ID)
		answerContent := `<p><em>Bài làm:</em> Chưa làm</p>`
		if currentAnswer, found := extractAnswerBlock(pageHTML, item.Exercise.ID); found && !isBlankAnswerHTML(currentAnswer) {
			answerContent = currentAnswer
		}
		content := buildWorkTable(exerciseID, answerContent, item.Result)
		generatedID, found := findGeneratedIDByDataID(pageHTML, "exercise-"+exerciseID+"-work")
		if !found && itemIndex < len(legacyTableTargets) {
			generatedID, found = legacyTableTargets[itemIndex], true
		}
		if !found {
			return nil, fmt.Errorf("trang OneNote chưa có vùng bài làm cập nhật được cho câu %s; vui lòng giao lại bài bằng phiên bản mới", exerciseID)
		}
		commands = append(commands, map[string]string{
			"target": generatedID, "action": "replace", "content": content,
		})
	}
	return commands, nil
}

func appendUniqueImage(images [][]byte, candidate []byte) [][]byte {
	for _, existing := range images {
		if bytes.Equal(existing, candidate) {
			return images
		}
	}
	return append(images, candidate)
}

func (c *Client) parseStudentAnswer(ctx context.Context, client *http.Client, answerHTML string, extractedAt time.Time) (*domain.StudentAnswer, error) {
	document, err := xhtml.Parse(strings.NewReader("<html><body>" + answerHTML + "</body></html>"))
	if err != nil {
		return nil, err
	}
	var textParts []string
	var imageURLs []string
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			if value := strings.TrimSpace(stdhtml.UnescapeString(node.Data)); value != "" {
				textParts = append(textParts, value)
			}
		}
		if node.Type == xhtml.ElementNode && node.Data == "img" {
			imageSource := ""
			fullResolutionSource := ""
			for _, attribute := range node.Attr {
				if attribute.Key == "data-fullres-src" && strings.TrimSpace(attribute.Val) != "" {
					fullResolutionSource = attribute.Val
				}
				if attribute.Key == "src" && strings.TrimSpace(attribute.Val) != "" {
					imageSource = attribute.Val
				}
			}
			if fullResolutionSource != "" {
				imageURLs = append(imageURLs, fullResolutionSource)
			} else if imageSource != "" {
				imageURLs = append(imageURLs, imageSource)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)

	images := make([][]byte, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		data, err := c.fetchAnswerImage(ctx, client, imageURL)
		if err != nil {
			return nil, err
		}
		images = append(images, data)
	}
	text := strings.Join(textParts, " ")
	return &domain.StudentAnswer{
		Text:        text,
		Images:      images,
		IsBlank:     isBlankAnswerHTML(answerHTML) && len(images) == 0,
		ExtractedAt: extractedAt,
	}, nil
}

func (c *Client) fetchAnswerImage(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" {
		return nil, errors.New("URL ảnh bài làm không hợp lệ")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "graph.microsoft.com" && host != "www.onenote.com" && !strings.HasSuffix(host, ".onenote.com") {
		return nil, fmt.Errorf("không tải ảnh từ host không tin cậy %q", host)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tải ảnh bài làm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("OneNote image API (%d)", resp.StatusCode)
	}
	const maxImageSize = 10 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageSize {
		return nil, errors.New("ảnh bài làm vượt quá 10 MB")
	}
	return data, nil
}

func buildWorkTable(exerciseID, answerContent string, result *domain.GradingResult) string {
	if result.Status == domain.AIExerciseSkippedEmpty {
		// Câu chưa làm giữ nguyên toàn chiều rộng. Replace một ô đồng thời dọn
		// cột “Chưa làm” đã được phiên bản cũ chèn vào.
		return `<table data-id="exercise-` + exerciseID + `-work" width="760" border="0" cellspacing="0" cellpadding="0" style="width:760px;border-collapse:collapse;margin:8pt 0 14pt;"><tr>` +
			`<td width="760" style="width:760px;padding:0;vertical-align:top;"><div data-id="exercise-` + exerciseID + `-answer-content">` + answerContent + `</div></td></tr></table>`
	}
	var feedback strings.Builder
	feedback.WriteString(`<div data-id="exercise-` + exerciseID + `-feedback">`)
	feedback.WriteString(`<p style="margin:0 0 8pt;"><strong>Nhận xét:</strong> `)
	if result.IsCorrect {
		feedback.WriteString(`<span style="color:#15803d;">Đúng</span></p>`)
	} else {
		feedback.WriteString(`<span style="color:#b91c1c;">Cần sửa</span></p>`)
	}
	feedback.WriteString(sanitizeFeedbackHTML(result.FeedbackHTML))
	feedback.WriteString(`</div>`)
	return `<table data-id="exercise-` + exerciseID + `-work" width="760" style="width:760px;border-collapse:collapse;margin:8pt 0 14pt;"><tr>` +
		`<td width="450" style="vertical-align:top;padding:10pt;border:1px solid #cbd5e1;"><div data-id="exercise-` + exerciseID + `-answer-content">` + answerContent + `</div></td>` +
		`<td width="310" style="vertical-align:top;padding:10pt;border:1px solid #cbd5e1;background:#f8fafc;">` + feedback.String() + `</td>` +
		`</tr></table>`
}

func (c *Client) fetchPageHTML(ctx context.Context, client *http.Client, pageID string) (string, error) {
	endpoint := fmt.Sprintf("%s/pages/%s/content?includeIDs=true", graphAPIBase, url.PathEscape(pageID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lấy cấu trúc trang OneNote: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("OneNote content API (%d): %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}

func addExerciseMarkers(markdown string) string {
	if strings.Contains(markdown, ":ANSWER]]") {
		return markdown
	}
	var out strings.Builder
	currentExerciseID := 0
	for _, line := range strings.Split(markdown, "\n") {
		if match := exerciseHeadingPattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			if currentExerciseID > 0 {
				out.WriteString(fmt.Sprintf("[[EXERCISE:%d:FEEDBACK]]\n", currentExerciseID))
				out.WriteString(fmt.Sprintf("[[EXERCISE:%d:END]]\n", currentExerciseID))
			}
			currentExerciseID, _ = strconv.Atoi(match[1])
			out.WriteString(fmt.Sprintf("[[EXERCISE:%d:START]]\n", currentExerciseID))
		}
		if currentExerciseID > 0 && strings.Contains(strings.ToLower(line), "bài làm") {
			out.WriteString(fmt.Sprintf("[[EXERCISE:%d:ANSWER]]\n", currentExerciseID))
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if currentExerciseID > 0 {
		out.WriteString(fmt.Sprintf("[[EXERCISE:%d:FEEDBACK]]\n", currentExerciseID))
		out.WriteString(fmt.Sprintf("[[EXERCISE:%d:END]]\n", currentExerciseID))
	}
	return out.String()
}

func extractAnswerBlock(pageHTML string, exerciseID int) (string, bool) {
	if answer, ok := extractElementInnerByID(pageHTML, fmt.Sprintf("exercise-%d-answer-content", exerciseID)); ok {
		return answer, true
	}
	if answer, ok := extractElementInnerByID(pageHTML, fmt.Sprintf("exercise-%d-work", exerciseID)); ok {
		return answer, true
	}
	// Tương thích trang cũ đã tạo trước khi marker được chuyển thành HTML id.
	marker := fmt.Sprintf("[[EXERCISE:%d:ANSWER]]", exerciseID)
	start := strings.Index(pageHTML, marker)
	if start < 0 {
		return "", false
	}
	start += len(marker)
	end := strings.Index(pageHTML[start:], "[[EXERCISE:")
	if end < 0 {
		end = len(pageHTML) - start
	}
	return strings.TrimSpace(pageHTML[start : start+end]), true
}

func extractElementInnerByID(pageHTML, targetID string) (string, bool) {
	document, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", false
	}
	var target *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if target != nil {
			return
		}
		if node.Type == xhtml.ElementNode {
			for _, attribute := range node.Attr {
				if (attribute.Key == "id" || attribute.Key == "data-id") && attribute.Val == targetID {
					target = node
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if target == nil {
		return "", false
	}
	var result strings.Builder
	for child := target.FirstChild; child != nil; child = child.NextSibling {
		if err := xhtml.Render(&result, child); err != nil {
			return "", false
		}
	}
	return strings.TrimSpace(result.String()), true
}

func findGeneratedIDByDataID(pageHTML, dataID string) (string, bool) {
	document, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", false
	}
	var generatedID string
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if generatedID != "" {
			return
		}
		if node.Type == xhtml.ElementNode {
			matches := false
			candidate := ""
			for _, attribute := range node.Attr {
				switch attribute.Key {
				case "data-id":
					matches = attribute.Val == dataID
				case "id":
					candidate = attribute.Val
				}
			}
			if matches && candidate != "" {
				generatedID = candidate
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	return generatedID, generatedID != ""
}

// findGeneratedTableIDs hỗ trợ các trang được tạo ở phiên bản bảng cũ, khi
// OneNote đã loại thuộc tính id đầu vào nhưng vẫn sinh ID riêng cho mỗi bảng.
func findGeneratedTableIDs(pageHTML string) []string {
	document, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	targets := make([]string, 0)
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "table" {
			for _, attribute := range node.Attr {
				if attribute.Key == "id" && strings.HasPrefix(attribute.Val, "table:") {
					targets = append(targets, attribute.Val)
					break
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	return targets
}

func isBlankAnswerHTML(answerHTML string) bool {
	if strings.Contains(strings.ToLower(answerHTML), "<img") {
		return false
	}
	plain := stdhtml.UnescapeString(htmlTagPattern.ReplaceAllString(answerHTML, " "))
	plain = strings.ReplaceAll(strings.ToLower(plain), "bài làm:", "")
	plain = strings.Map(func(r rune) rune {
		switch r {
		case '.', '…', '_', '-', '*', ' ', '\t', '\n', '\r', '\u00a0':
			return -1
		default:
			return r
		}
	}, plain)
	return strings.TrimSpace(plain) == ""
}

func sanitizeFeedbackHTML(value string) string {
	value = unsafeHTMLBlockPattern.ReplaceAllString(value, "")
	value = unsafeHTMLAttrPattern.ReplaceAllString(value, "")
	value = verboseFeedbackPattern.ReplaceAllString(value, "")
	if strings.TrimSpace(value) == "" {
		return `<p>Không có nhận xét chi tiết.</p>`
	}
	return value
}

func (c *Client) resolveNotebook(ctx context.Context, client *http.Client, target application.WorkspaceTarget) (string, error) {
	if id := strings.TrimSpace(pointerValue(target.WorkspaceID)); id != "" {
		return id, nil
	}
	name := sanitizeNotebookName(pointerValue(target.WorkspaceName))
	if name == "" {
		return "", errors.New("thiếu workspace_id hoặc workspace_name")
	}

	endpoint := graphAPIBase + "/notebooks?$select=id,displayName"
	var notebooks struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"value"`
	}
	if err := c.getJSON(ctx, client, endpoint, &notebooks); err != nil {
		return "", fmt.Errorf("liệt kê Notebook: %w", err)
	}
	for _, notebook := range notebooks.Value {
		if strings.EqualFold(strings.TrimSpace(notebook.DisplayName), name) {
			return notebook.ID, nil
		}
	}

	body, err := json.Marshal(map[string]string{"displayName": name})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphAPIBase+"/notebooks", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tạo Notebook: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("OneNote create Notebook API (%d): %s", res.StatusCode, string(responseBody))
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil || strings.TrimSpace(created.ID) == "" {
		return "", errors.New("OneNote tạo Notebook nhưng không trả notebook ID")
	}
	return created.ID, nil
}

func (c *Client) resolveSection(ctx context.Context, client *http.Client, notebookID string, target application.WorkspaceTarget) (string, error) {
	if id := strings.TrimSpace(pointerValue(target.ChapterID)); id != "" {
		return id, nil
	}
	name := strings.TrimSpace(pointerValue(target.ChapterName))
	if name == "" {
		return "", errors.New("thiếu chapter_id hoặc chapter_name")
	}
	return c.ensureSection(ctx, client, notebookID, name)
}

func (c *Client) ensureSection(ctx context.Context, client *http.Client, notebookID, chapterName string) (string, error) {
	chapterName = sanitizeSectionName(chapterName)

	endpoint := fmt.Sprintf("%s/notebooks/%s/sections?$select=id,displayName", graphAPIBase, url.PathEscape(notebookID))
	var resp struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"value"`
	}

	if err := c.getJSON(ctx, client, endpoint, &resp); err != nil {
		return "", fmt.Errorf("liệt kê Section: %w", err)
	}
	for _, s := range resp.Value {
		if strings.EqualFold(strings.TrimSpace(s.DisplayName), strings.TrimSpace(chapterName)) {
			return s.ID, nil
		}
	}

	createEndpoint := fmt.Sprintf("%s/notebooks/%s/sections", graphAPIBase, url.PathEscape(notebookID))
	reqBody, _ := json.Marshal(map[string]string{"displayName": chapterName})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("OneNote create Section API (%d): %s", res.StatusCode, string(responseBody))
	}

	var newSec struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&newSec); err != nil || newSec.ID == "" {
		return "", fmt.Errorf("không thể tạo chương '%s'", chapterName)
	}

	return newSec.ID, nil
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (c *Client) getJSON(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("OneNote API lỗi %d: %s", res.StatusCode, string(body))
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func sanitizeSectionName(name string) string {
	invalidChars := []string{"?", "*", "\\", "/", ":", "<", ">", "|", "&", "#", "%", "~"}
	for _, ch := range invalidChars {
		name = strings.ReplaceAll(name, ch, " ")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Chương mới"
	}
	return name
}

func sanitizeNotebookName(name string) string {
	invalidChars := []string{"?", "*", "\\", "/", ":", "<", ">", "|", "'", "\""}
	for _, character := range invalidChars {
		name = strings.ReplaceAll(name, character, " ")
	}
	name = strings.TrimSpace(name)
	runes := []rune(name)
	if len(runes) > 128 {
		name = strings.TrimSpace(string(runes[:128]))
	}
	return name
}
