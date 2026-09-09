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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	"meet-attendance-clean/infrastructure/html"

	xhtml "golang.org/x/net/html"
	"golang.org/x/oauth2"
)

const graphAPIBase = "https://graph.microsoft.com/v1.0/me/onenote"

type Client struct {
	config        *oauth2.Config
	tokenPath     string
	htmlConverter *html.LessonRenderer
}

var (
	unsafeHTMLBlockPattern  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
	unsafeHTMLAttrPattern   = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*')`)
	debugPathUnsafePattern  = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	answerDebugSnapshotRoot = filepath.Join("debug", "onenote-answer-snapshots")
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
		htmlConverter: html.NewLessonRenderer(),
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
	return c.saveToken(token)
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

// ==================== 2. TRUY VẤN & GIAO BÀI ====================

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

func (c *Client) PublishSession(ctx context.Context, target application.WorkspaceTarget, audience domain.Audience, lesson domain.Lesson) (application.PublishResult, error) {
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return application.PublishResult{}, err
	}

	workspaceID, err := c.resolveNotebook(ctx, client, target)
	if err != nil {
		return application.PublishResult{}, fmt.Errorf("chuẩn bị Notebook: %w", err)
	}

	sectionID, err := c.resolveSection(ctx, client, workspaceID, target)
	if err != nil {
		return application.PublishResult{}, fmt.Errorf("chuẩn bị Section: %w", err)
	}

	pageName := lesson.Title
	if target.PageName != nil && strings.TrimSpace(*target.PageName) != "" {
		pageName = strings.TrimSpace(*target.PageName)
	}

	htmlPayload := c.htmlConverter.RenderLessonHTML(pageName, &lesson, audience)

	endpoint := fmt.Sprintf("%s/sections/%s/pages", graphAPIBase, url.PathEscape(sectionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(htmlPayload))
	if err != nil {
		return application.PublishResult{}, fmt.Errorf("lỗi khởi tạo request tạo trang: %w", err)
	}
	req.Header.Set("Content-Type", "text/html; charset=utf-8")

	res, err := client.Do(req)
	if err != nil {
		return application.PublishResult{}, fmt.Errorf("lỗi gửi request tạo trang OneNote: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return application.PublishResult{}, fmt.Errorf("OneNote API lỗi (%d): %s", res.StatusCode, string(body))
	}

	var pageResp struct {
		ID    string `json:"id"`
		Links struct {
			OneNoteWebURL struct {
				Href string `json:"href"`
			} `json:"oneNoteWebUrl"`
		} `json:"links"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pageResp); err != nil || pageResp.ID == "" {
		return application.PublishResult{}, errors.New("tạo trang thành công nhưng OneNote không trả về PageID")
	}

	return application.PublishResult{
		PageID:      pageResp.ID,
		WorkspaceID: workspaceID,
		PageWebURL:  pageResp.Links.OneNoteWebURL.Href,
	}, nil
}

func (c *Client) PublishStudentLesson(ctx context.Context, target application.WorkspaceTarget, lesson *domain.Lesson) (application.PublishResult, error) {
	if lesson == nil {
		return application.PublishResult{}, errors.New("lesson không được nil")
	}
	return c.PublishSession(ctx, target, domain.AudienceStudent, *lesson)
}

// ==================== 3. THU BÀI & TRẢ NHẬN XÉT ====================

// FetchStudentAnswers bóc tách chữ gõ, ảnh dán và đính kèm ảnh nét vẽ toàn trang
func (c *Client) FetchStudentAnswers(ctx context.Context, assignment *domain.Assignment) error {
	if assignment == nil || strings.TrimSpace(assignment.TargetPageID) == "" {
		return errors.New("assignment thiếu TargetPageID")
	}
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return err
	}

	// 1. Tải HTML và InkML từ OneNote
	contentHTML, inkData, err := c.fetchPageHTMLAndInk(ctx, client, assignment.TargetPageID)
	if err != nil {
		return err
	}

	// 2. Render nét vẽ tay thành 1 file ảnh toàn trang DUY NHẤT
	if len(inkData) > 0 {
		assignment.PageInkImage, _ = RenderInkMLToJPEG(inkData) // Lưu ở cấp bài tập (1 lần)
	}

	extractedAt := time.Now().UTC()

	// 3. Bóc tách nội dung riêng của từng câu hỏi
	for i := range assignment.Items {
		item := &assignment.Items[i]
		exerciseID := item.Exercise.ID

		answer := &domain.StudentAnswer{
			ChoiceSelection: domain.ChoiceNotApplicable,
			ExtractedAt:     extractedAt,
		}

		// A. Bóc tách text và ảnh dán trong ô làm bài của câu này
		if answerHTML, found := extractElementInnerByID(contentHTML, fmt.Sprintf("exercise-%d-answer-content", exerciseID)); found {
			parsed, err := c.parseStudentAnswer(ctx, client, answerHTML, extractedAt)
			if err == nil {
				answer.Text = parsed.Text
				answer.Images = append(answer.Images, parsed.Images...)
			}
		}

		// Lựa chọn A/B/C/D là một trạng thái nghiệp vụ riêng, không trộn vào bài
		// trình bày. Logic chấm sẽ chặn chưa chọn/chọn nhiều trước khi gọi Gemini.
		if len(item.Exercise.Options) > 0 {
			answer.ChoiceSelection = domain.ChoiceUnselected
			if choiceHTML, found := extractElementInnerByID(contentHTML, fmt.Sprintf("exercise-%d-choice", exerciseID)); found {
				switch selected := extractSelectedOptions(choiceHTML); len(selected) {
				case 1:
					answer.ChoiceSelection = domain.ChoiceSelected
					answer.SelectedOption = selected[0]
				default:
					if len(selected) > 1 {
						answer.ChoiceSelection = domain.ChoiceMultiple
					}
				}
			}
		}

		// B. Bóc tách thêm ảnh nếu dán lệch ra ngoài ô làm bài của câu này
		if regionHTML, found := extractElementInnerByID(contentHTML, fmt.Sprintf("exercise-%d-region", exerciseID)); found {
			parsed, err := c.parseStudentAnswer(ctx, client, regionHTML, extractedAt)
			if err == nil {
				for _, img := range parsed.Images {
					answer.Images = appendUniqueImage(answer.Images, img)
				}
			}
		}

		item.StudentAnswer = answer
	}
	return nil
}

// PatchFeedback chèn hộp nhận xét vào đúng thẻ chờ <div data-id="exercise-X-feedback"></div>
func (c *Client) PatchFeedback(ctx context.Context, assignment *domain.Assignment) error {
	if assignment == nil || strings.TrimSpace(assignment.TargetPageID) == "" {
		return errors.New("assignment thiếu TargetPageID")
	}
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return err
	}

	pageHTML, err := c.fetchPageHTML(ctx, client, assignment.TargetPageID)
	if err != nil {
		return err
	}

	commands := make([]map[string]string, 0, len(assignment.Items))
	missingTargets := make([]int, 0)
	eligibleCount := 0

	for _, item := range assignment.Items {
		if item.Result == nil || item.Result.Status == domain.AIExerciseSkippedEmpty {
			continue
		}
		eligibleCount++

		// Tìm thẻ feedback hiện tại. Bản cũ thay placeholder bằng ID chung,
		// nên cần fallback tìm card bên trong đúng exercise region.
		targetID, found := findFeedbackTargetID(pageHTML, item.Exercise.ID)
		if !found {
			missingTargets = append(missingTargets, item.Exercise.ID)
			continue
		}

		// Tạo HTML hộp nhận xét
		feedbackBoxHTML := renderFeedbackCard(item.Exercise.ID, item.Result)

		commands = append(commands, map[string]string{
			"target":  targetID,
			"action":  "replace",
			"content": feedbackBoxHTML,
		})
	}

	if eligibleCount == 0 {
		return nil
	}
	if len(missingTargets) > 0 {
		return fmt.Errorf("không tìm thấy vị trí nhận xét OneNote cho các câu: %v", missingTargets)
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
		return fmt.Errorf("gửi lệnh PATCH feedback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("OneNote PATCH API (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// ==================== 4. HELPER FUNCTIONS ====================

func renderFeedbackCard(exerciseID int, result *domain.GradingResult) string {
	borderColor := "#16a34a" // Xanh lá
	badgeColor := "#15803d"
	badgeText := "✅ Đúng"
	feedbackHTML := ""
	if strings.TrimSpace(result.FeedbackHTML) != "" {
		feedbackHTML = compactFeedbackHTML(result.FeedbackHTML)
	}

	if result.Status == domain.AIExerciseAwaitingSelection {
		borderColor = "#f59e0b" // Cam: học sinh cần chọn lại đáp án
		badgeColor = "#b45309"
		badgeText = "⚠️ Chưa hoàn tất"
	} else if !result.IsCorrect {
		borderColor = "#dc2626" // Đỏ
		badgeColor = "#b91c1c"
		badgeText = "❌ Cần sửa"
	}

	return fmt.Sprintf(
		`<div data-id="exercise-%d-feedback" style="width:246px; background:#f8fafc; border:1px solid #e2e8f0; border-left:4px solid %s; padding:6pt 8pt; margin:0; border-radius:0 4px 4px 0; word-wrap:break-word;">`+
			`<p style="margin:0 0 2pt 0; font-weight:bold; color:%s;">%s</p>`+
			`<div style="color:#1e293b; font-size:9pt; line-height:1.25;">%s</div>`+
			`</div>`,
		exerciseID, borderColor, badgeColor, badgeText, feedbackHTML,
	)
}

func extractSelectedOptions(rawHTML string) []string {
	doc, err := xhtml.Parse(strings.NewReader("<html><body>" + rawHTML + "</body></html>"))
	if err != nil {
		return nil
	}

	selected := make([]string, 0, 1)
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		// OneNote chuyển <p data-tag> ở HTML đầu vào thành <span data-tag>
		// trong HTML trả về, nên không được giới hạn ở riêng thẻ p.
		if node.Type == xhtml.ElementNode {
			var id, tag string
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "data-id":
					id = attr.Val
				case "data-tag":
					tag = strings.ToLower(attr.Val)
				}
			}
			if strings.HasPrefix(tag, "to-do:completed") {
				if index := strings.LastIndex(id, "-option-"); index >= 0 {
					option := strings.TrimSpace(id[index+len("-option-"):])
					if option != "" {
						selected = append(selected, option)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return selected
}

func (c *Client) parseStudentAnswer(ctx context.Context, client *http.Client, rawHTML string, extractedAt time.Time) (*domain.StudentAnswer, error) {
	doc, err := xhtml.Parse(strings.NewReader("<html><body>" + rawHTML + "</body></html>"))
	if err != nil {
		return nil, err
	}

	var textParts []string
	var imageURLs []string

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			val := strings.TrimSpace(stdhtml.UnescapeString(n.Data))
			// Chỉ bỏ qua đúng dòng văn bản placeholder hướng dẫn mặc định của hệ thống
			if val != "" && !strings.HasPrefix(val, "✍️ Bài làm (Gõ chữ") {
				textParts = append(textParts, val)
			}
		}
		if n.Type == xhtml.ElementNode && n.Data == "img" {
			src := ""
			for _, a := range n.Attr {
				if a.Key == "data-fullres-src" && strings.TrimSpace(a.Val) != "" {
					src = a.Val
					break
				}
				if a.Key == "src" && strings.TrimSpace(a.Val) != "" {
					src = a.Val
				}
			}
			if src != "" {
				imageURLs = append(imageURLs, src)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	images := make([][]byte, 0, len(imageURLs))
	for _, imgURL := range imageURLs {
		if data, err := c.fetchAnswerImage(ctx, client, imgURL); err == nil && len(data) > 0 {
			images = append(images, data)
		}
	}

	return &domain.StudentAnswer{
		Text:        strings.Join(textParts, " "),
		Images:      images,
		ExtractedAt: extractedAt,
	}, nil
}

func (c *Client) fetchAnswerImage(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" {
		return nil, errors.New("URL không hợp lệ")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("OneNote Image API (%d)", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}

func isBlankAnswer(text string, images [][]byte) bool {
	if len(images) > 0 {
		return false
	}
	clean := strings.TrimSpace(text)
	clean = strings.ReplaceAll(strings.ToLower(clean), "bài làm:", "")
	clean = strings.ReplaceAll(strings.ToLower(clean), "bài làm", "")
	clean = strings.Map(func(r rune) rune {
		switch r {
		case '.', '…', '_', '-', '*', ' ', '\t', '\n', '\r', '\u00a0':
			return -1
		default:
			return r
		}
	}, clean)
	return strings.TrimSpace(clean) == ""
}

func sanitizeFeedbackHTML(value string) string {
	value = unsafeHTMLBlockPattern.ReplaceAllString(value, "")
	value = unsafeHTMLAttrPattern.ReplaceAllString(value, "")
	if strings.TrimSpace(value) == "" {
		return `<p>Không có nhận xét chi tiết.</p>`
	}
	return value
}

// Giới hạn feedback dù AI trả về dài: hộp bên phải phải luôn thấp hơn vùng
// làm bài để không làm tăng chiều cao hàng và đẩy câu kế tiếp xuống dưới.
func compactFeedbackHTML(value string) string {
	if strings.TrimSpace(value) == "" {
		return `<p>Chưa có nhận xét cụ thể.</p>`
	}
	value = sanitizeFeedbackHTML(value)
	doc, err := xhtml.Parse(strings.NewReader("<html><body>" + value + "</body></html>"))
	if err != nil {
		return `<p>Không có nhận xét.</p>`
	}

	paragraphs := make([]string, 0, 2)
	var collectText func(*xhtml.Node) string
	collectText = func(node *xhtml.Node) string {
		var text strings.Builder
		var walk func(*xhtml.Node)
		walk = func(current *xhtml.Node) {
			if current.Type == xhtml.TextNode {
				text.WriteString(current.Data)
			}
			for child := current.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(node)
		return strings.Join(strings.Fields(stdhtml.UnescapeString(text.String())), " ")
	}
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "p" && len(paragraphs) < 2 {
			if text := collectText(node); text != "" {
				paragraphs = append(paragraphs, truncateRunes(text, 90))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(paragraphs) == 0 {
		if text := collectText(doc); text != "" {
			paragraphs = append(paragraphs, truncateRunes(text, 90))
		}
	}
	if len(paragraphs) == 0 {
		return `<p>Không có nhận xét.</p>`
	}

	var result strings.Builder
	for _, paragraph := range paragraphs {
		result.WriteString("<p style=\"margin:0 0 2pt 0;\">")
		result.WriteString(escapeFeedbackHTML(paragraph))
		result.WriteString("</p>")
	}
	return result.String()
}

func truncateRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
}

func escapeFeedbackHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, `"`, "&quot;")
	return text
}

func appendUniqueImage(images [][]byte, candidate []byte) [][]byte {
	for _, existing := range images {
		if bytes.Equal(existing, candidate) {
			return images
		}
	}
	return append(images, candidate)
}

func extractElementInnerByID(pageHTML, targetID string) (string, bool) {
	doc, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", false
	}
	var target *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if target != nil {
			return
		}
		if n.Type == xhtml.ElementNode {
			for _, a := range n.Attr {
				if (a.Key == "id" || a.Key == "data-id") && a.Val == targetID {
					target = n
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if target == nil {
		return "", false
	}
	var result strings.Builder
	for child := target.FirstChild; child != nil; child = child.NextSibling {
		xhtml.Render(&result, child)
	}
	return strings.TrimSpace(result.String()), true
}

func findGeneratedIDByDataID(pageHTML, dataID string) (string, bool) {
	doc, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", false
	}
	var generatedID string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if generatedID != "" {
			return
		}
		if n.Type == xhtml.ElementNode {
			isMatch := false
			idVal := ""
			for _, a := range n.Attr {
				if a.Key == "data-id" && a.Val == dataID {
					isMatch = true
				}
				if a.Key == "id" {
					idVal = a.Val
				}
			}
			if isMatch && idVal != "" {
				generatedID = idVal
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return generatedID, generatedID != ""
}

func findFeedbackTargetID(pageHTML string, exerciseID int) (string, bool) {
	if id, found := findGeneratedIDByDataID(pageHTML, fmt.Sprintf("exercise-%d-feedback", exerciseID)); found {
		return id, true
	}

	doc, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", false
	}
	regionID := fmt.Sprintf("exercise-%d-region", exerciseID)
	var region *xhtml.Node
	var findRegion func(*xhtml.Node)
	findRegion = func(node *xhtml.Node) {
		if region != nil {
			return
		}
		if node.Type == xhtml.ElementNode && hasHTMLAttribute(node, "data-id", regionID) {
			region = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			findRegion(child)
		}
	}
	findRegion(doc)
	if region == nil {
		return "", false
	}

	var generatedID string
	var findLegacyCard func(*xhtml.Node)
	findLegacyCard = func(node *xhtml.Node) {
		if generatedID != "" {
			return
		}
		if node.Type == xhtml.ElementNode && hasHTMLAttribute(node, "data-id", "exercise-feedback-result") {
			for _, attr := range node.Attr {
				if attr.Key == "id" && strings.TrimSpace(attr.Val) != "" {
					generatedID = attr.Val
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			findLegacyCard(child)
		}
	}
	findLegacyCard(region)
	return generatedID, generatedID != ""
}

func hasHTMLAttribute(node *xhtml.Node, key, value string) bool {
	for _, attr := range node.Attr {
		if attr.Key == key && attr.Val == value {
			return true
		}
	}
	return false
}

func (c *Client) fetchPageHTML(ctx context.Context, client *http.Client, pageID string) (string, error) {
	endpoint := fmt.Sprintf("%s/pages/%s/content?includeIDs=true", graphAPIBase, url.PathEscape(pageID))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	return string(body), nil
}

func (c *Client) DumpStudentAnswersDebug(assignment *domain.Assignment) (string, error) {
	if assignment == nil {
		return "", errors.New("assignment nil")
	}

	safePageID := debugPathUnsafePattern.ReplaceAllString(assignment.TargetPageID, "_")
	timestamp := time.Now().Format("20060102_150405")
	sessionDir := filepath.Join(answerDebugSnapshotRoot, fmt.Sprintf("page_%s_%s", safePageID, timestamp))

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", err
	}

	for i, item := range assignment.Items {
		exDir := filepath.Join(sessionDir, fmt.Sprintf("cau_%d_id_%d", i+1, item.Exercise.ID))
		_ = os.MkdirAll(exDir, 0755)

		var summary strings.Builder
		summary.WriteString(fmt.Sprintf("=== CÂU %d (ID: %d) ===\n", i+1, item.Exercise.ID))
		summary.WriteString(fmt.Sprintf("Đề bài:\n%s\n\n", item.Exercise.Question))
		if item.StudentAnswer != nil {
			summary.WriteString(fmt.Sprintf("Text: \"%s\"\n", item.StudentAnswer.Text))
			summary.WriteString(fmt.Sprintf("Images count: %d\n", len(item.StudentAnswer.Images)))
		}
		_ = os.WriteFile(filepath.Join(exDir, "summary.txt"), []byte(summary.String()), 0644)

		if item.StudentAnswer != nil {
			for imgIdx, imgBytes := range item.StudentAnswer.Images {
				_ = os.WriteFile(filepath.Join(exDir, fmt.Sprintf("image_%d.jpg", imgIdx+1)), imgBytes, 0644)
			}
		}
	}

	return sessionDir, nil
}

func (c *Client) resolveNotebook(ctx context.Context, client *http.Client, target application.WorkspaceTarget) (string, error) {
	if id := strings.TrimSpace(pointerValue(target.WorkspaceID)); id != "" {
		return id, nil
	}
	name := sanitizeNotebookName(pointerValue(target.WorkspaceName))
	if name == "" {
		return "", errors.New("thiếu tên notebook để tạo mới")
	}
	endpoint := graphAPIBase + "/notebooks?$select=id,displayName"
	var notebooks struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"value"`
	}
	if err := c.getJSON(ctx, client, endpoint, &notebooks); err != nil {
		return "", fmt.Errorf("lấy danh sách notebook: %w", err)
	}
	for _, nb := range notebooks.Value {
		if strings.EqualFold(strings.TrimSpace(nb.DisplayName), strings.TrimSpace(name)) {
			return nb.ID, nil
		}
	}

	body, err := json.Marshal(map[string]string{"displayName": name})
	if err != nil {
		return "", fmt.Errorf("mã hoá tên notebook: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphAPIBase+"/notebooks", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("khởi tạo request tạo notebook: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gửi request tạo notebook: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("OneNote API tạo notebook lỗi (%d): %s", res.StatusCode, string(body))
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("đọc phản hồi tạo notebook: %w", err)
	}
	if strings.TrimSpace(created.ID) == "" {
		return "", errors.New("OneNote tạo notebook thành công nhưng không trả về notebook ID")
	}
	return created.ID, nil
}

func (c *Client) resolveSection(ctx context.Context, client *http.Client, notebookID string, target application.WorkspaceTarget) (string, error) {
	if id := strings.TrimSpace(pointerValue(target.ChapterID)); id != "" {
		return id, nil
	}
	name := strings.TrimSpace(pointerValue(target.ChapterName))
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
		return "", fmt.Errorf("lấy danh sách section: %w", err)
	}
	for _, s := range resp.Value {
		if strings.EqualFold(strings.TrimSpace(s.DisplayName), strings.TrimSpace(chapterName)) {
			return s.ID, nil
		}
	}

	createEndpoint := fmt.Sprintf("%s/notebooks/%s/sections", graphAPIBase, url.PathEscape(notebookID))
	reqBody, err := json.Marshal(map[string]string{"displayName": chapterName})
	if err != nil {
		return "", fmt.Errorf("mã hoá tên section: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("khởi tạo request tạo section: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gửi request tạo section: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("OneNote API tạo section lỗi (%d): %s", res.StatusCode, string(body))
	}

	var newSec struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&newSec); err != nil {
		return "", fmt.Errorf("đọc phản hồi tạo section: %w", err)
	}
	if strings.TrimSpace(newSec.ID) == "" {
		return "", errors.New("OneNote tạo section thành công nhưng không trả về section ID")
	}
	return newSec.ID, nil
}

func (c *Client) getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("OneNote API lỗi (%d): %s", res.StatusCode, string(body))
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func sanitizeNotebookName(name string) string {
	invalidChars := []string{"?", "*", "\\", "/", ":", "<", ">", "|", "'", `"`}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, " ")
	}
	name = strings.TrimSpace(name)
	if len([]rune(name)) > 128 {
		name = string([]rune(name)[:128])
	}
	return name
}

func sanitizeSectionName(name string) string {
	invalidChars := []string{"?", "*", "\\", "/", ":", "<", ">", "|", "&", "#", "%", "~"}
	for _, ch := range invalidChars {
		name = strings.ReplaceAll(name, ch, " ")
	}
	if strings.TrimSpace(name) == "" {
		name = "Chương mới"
	}
	return strings.TrimSpace(name)
}

func pointerValue(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}
