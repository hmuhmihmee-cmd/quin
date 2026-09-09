package googlemeet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"meet-attendance-clean/domain"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const meetAPIBase = "https://meet.googleapis.com/v2"
const calendarAPIBase = "https://www.googleapis.com/calendar/v3"

type Client struct {
	config    *oauth2.Config
	tokenPath string
}

func New(credentials []byte, tokenPath, redirectURL string) (*Client, error) {
	if len(credentials) == 0 {
		return nil, errors.New("cấu hình Google OAuth nhúng trong ứng dụng đang trống")
	}
	cfg, err := google.ConfigFromJSON(credentials,
		"https://www.googleapis.com/auth/meetings.space.readonly",
		"https://www.googleapis.com/auth/calendar.readonly",
	)
	if err != nil {
		return nil, fmt.Errorf("cấu hình Google OAuth không hợp lệ: %w", err)
	}
	cfg.RedirectURL = redirectURL
	return &Client{config: cfg, tokenPath: tokenPath}, nil
}

func (c *Client) IsConnected() bool {
	_, err := os.Stat(c.tokenPath)
	return err == nil
}

func (c *Client) AuthorizationURL(state string) (string, error) {
	return c.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

func (c *Client) Exchange(ctx context.Context, code string) error {
	token, err := c.config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("đổi OAuth token: %w", err)
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
		return nil, errors.New("hãy liên kết tài khoản Google trước")
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("đọc token: %w", err)
	}

	tokenSource := c.config.TokenSource(ctx, &token)
	freshToken, err := tokenSource.Token()
	if err != nil {
		// Token bị Google thu hồi hoặc refresh thất bại -> Tự động xóa file rác
		_ = c.Disconnect()
		return nil, errors.New("AUTH_EXPIRED: phiên đăng nhập Google đã hết hạn")
	}
	if freshToken.AccessToken == token.AccessToken {
		return c.config.Client(ctx, &token), nil
	}
	err = c.saveToken(freshToken)
	if err != nil {
		return nil, err
	}
	return c.config.Client(ctx, freshToken), nil
}
func (c *Client) saveToken(token *oauth2.Token) error {
	data, err := json.Marshal(token) // hoặc json.MarshalIndent(token, "", "  ") để JSON thụt lề đẹp mắt
	if err != nil {
		return err
	}
	// Tự động tạo và ghi đè file với quyền bảo mật 0600
	return os.WriteFile(c.tokenPath, data, 0600)
}

// ==================== IMPLEMENT ListMeetRepoForSync ====================

func (c *Client) ListMeetingsFrom(ctx context.Context, fromTime time.Time) ([]domain.Meeting, error) {
	client, err := c.getHTTPClient(ctx)
	if err != nil {
		return nil, err
	}

	calendarTitles, err := c.calendarTitlesByMeetingCode(ctx, client, fromTime)
	if err != nil {
		return nil, err
	}
	spaceDetails := make(map[string]meetingSpace)

	// Domain và database chuẩn hoá UTC; Google Meet cũng nhận filter RFC3339 UTC.
	filter := fmt.Sprintf("start_time >= %q", fromTime.UTC().Format(time.RFC3339))
	var meetings []domain.Meeting
	pageToken := ""

	for {
		endpoint := fmt.Sprintf("%s/conferenceRecords?filter=%s&pageSize=100", meetAPIBase, url.QueryEscape(filter))
		if pageToken != "" {
			endpoint += "&pageToken=" + pageToken
		}

		var page struct {
			Records []struct {
				Name      string `json:"name"`
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
				Space     string `json:"space"`
			} `json:"conferenceRecords"`
			NextPageToken string `json:"nextPageToken"`
		}

		if err := c.getJSON(ctx, client, endpoint, &page); err != nil {
			return nil, err
		}

		for _, rec := range page.Records {
			spaceID := strings.TrimPrefix(rec.Space, "spaces/")
			details, found := spaceDetails[spaceID]
			if !found {
				details, err = c.getMeetingSpace(ctx, client, spaceID)
				if err != nil {
					return nil, fmt.Errorf("lấy mã Meet của %s: %w", rec.Space, err)
				}
				spaceDetails[spaceID] = details
			}
			// Google trả RFC3339 UTC. Chỉ frontend mới quy đổi sang giờ Việt Nam.
			startedAtUTC, err := time.Parse(time.RFC3339, rec.StartTime)
			if err != nil {
				continue
			}
			startedAtUTC = startedAtUTC.UTC()

			var endedAtUTC time.Time
			if rec.EndTime != "" {
				if t, err := time.Parse(time.RFC3339, rec.EndTime); err == nil {
					endedAtUTC = t.UTC()
				}
			}

			participants, err := c.listParticipants(ctx, client, rec.Name, startedAtUTC, endedAtUTC)
			if err != nil {
				return nil, fmt.Errorf("lấy người tham gia %s: %w", rec.Name, err)
			}

			meetings = append(meetings, domain.Meeting{
				ID:           strings.TrimPrefix(rec.Name, "conferenceRecords/"),
				Class:        spaceID,
				MeetingCode:  details.MeetingCode,
				SpaceName:    calendarTitles[strings.ToLower(details.MeetingCode)],
				StartedAt:    startedAtUTC,
				EndedAt:      endedAtUTC,
				Participants: participants,
			})
		}

		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return meetings, nil
}

type meetingSpace struct {
	MeetingCode string
}

func (c *Client) getMeetingSpace(ctx context.Context, client *http.Client, spaceID string) (meetingSpace, error) {
	var response struct {
		MeetingCode string `json:"meetingCode"`
	}
	if err := c.getJSON(ctx, client, fmt.Sprintf("%s/spaces/%s", meetAPIBase, url.PathEscape(spaceID)), &response); err != nil {
		return meetingSpace{}, err
	}
	if strings.TrimSpace(response.MeetingCode) == "" {
		return meetingSpace{}, errors.New("Google Meet không trả meetingCode")
	}
	return meetingSpace{MeetingCode: response.MeetingCode}, nil
}

// calendarTitlesByMeetingCode đọc Calendar một lần mỗi lượt sync. Nó chỉ lấy
// title và điểm vào Meet để gắn tên lịch do giáo viên đặt, không sửa Calendar.
func (c *Client) calendarTitlesByMeetingCode(ctx context.Context, client *http.Client, fromTime time.Time) (map[string]string, error) {
	result := make(map[string]string)
	pageToken := ""
	for {
		query := url.Values{
			"singleEvents": {"false"},
			"timeMin":      {fromTime.UTC().Add(-24 * time.Hour).Format(time.RFC3339)},
			"timeMax":      {time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)},
			"maxResults":   {"2500"},
			"fields":       {"items(summary,hangoutLink,conferenceData(entryPoints(entryPointType,uri,meetingCode))),nextPageToken"},
		}
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		var response struct {
			Items []struct {
				Summary        string `json:"summary"`
				HangoutLink    string `json:"hangoutLink"`
				ConferenceData struct {
					EntryPoints []struct {
						EntryPointType string `json:"entryPointType"`
						URI            string `json:"uri"`
						MeetingCode    string `json:"meetingCode"`
					} `json:"entryPoints"`
				} `json:"conferenceData"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := c.getJSON(ctx, client, calendarAPIBase+"/calendars/primary/events?"+query.Encode(), &response); err != nil {
			return nil, fmt.Errorf("đọc Google Calendar (hãy kết nối Google lại để cấp quyền Calendar): %w", err)
		}
		for _, event := range response.Items {
			title := strings.TrimSpace(event.Summary)
			if title == "" {
				continue
			}
			for _, code := range eventMeetingCodes(event) {
				result[strings.ToLower(code)] = title
			}
		}
		pageToken = response.NextPageToken
		if pageToken == "" {
			return result, nil
		}
	}
}

func eventMeetingCodes(event struct {
	Summary        string `json:"summary"`
	HangoutLink    string `json:"hangoutLink"`
	ConferenceData struct {
		EntryPoints []struct {
			EntryPointType string `json:"entryPointType"`
			URI            string `json:"uri"`
			MeetingCode    string `json:"meetingCode"`
		} `json:"entryPoints"`
	} `json:"conferenceData"`
}) []string {
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if parsed, err := url.Parse(value); err == nil && parsed.Host == "meet.google.com" {
			value = strings.Trim(parsed.Path, "/")
		}
		if value != "" {
			seen[strings.ToLower(value)] = struct{}{}
		}
	}
	add(event.HangoutLink)
	for _, entry := range event.ConferenceData.EntryPoints {
		if entry.EntryPointType == "video" {
			add(entry.MeetingCode)
			add(entry.URI)
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	return values
}

func (c *Client) listParticipants(ctx context.Context, client *http.Client, recordName string, meetStartUTC, meetEndUTC time.Time) ([]domain.Participant, error) {
	var participants []domain.Participant
	pageToken := ""

	for {
		endpoint := fmt.Sprintf("%s/%s/participants?pageSize=100", meetAPIBase, recordName)
		if pageToken != "" {
			endpoint += "&pageToken=" + pageToken
		}

		var page struct {
			Participants []struct {
				Name         string `json:"name"`
				SignedInUser *struct {
					DisplayName string `json:"displayName"`
				} `json:"signedInUser"`
				AnonymousUser *struct {
					DisplayName string `json:"displayName"`
				} `json:"anonymousUser"`
			} `json:"participants"`
			NextPageToken string `json:"nextPageToken"`
		}

		if err := c.getJSON(ctx, client, endpoint, &page); err != nil {
			return nil, err
		}

		for _, p := range page.Participants {
			displayName := "Khách ẩn danh"
			if p.SignedInUser != nil && p.SignedInUser.DisplayName != "" {
				displayName = p.SignedInUser.DisplayName
			} else if p.AnonymousUser != nil && p.AnonymousUser.DisplayName != "" {
				displayName = p.AnonymousUser.DisplayName
			}

			firstJoinUTC, lastLeaveUTC, duration := c.readSessions(ctx, client, p.Name, meetStartUTC, meetEndUTC)
			if !firstJoinUTC.IsZero() {
				participants = append(participants, domain.Participant{
					Name:          displayName,
					FirstJoinedAt: firstJoinUTC,
					LastLeftAt:    lastLeaveUTC,
					Duration:      duration,
				})
			}
		}

		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return participants, nil
}

func (c *Client) readSessions(ctx context.Context, client *http.Client, participantName string, meetStartUTC, meetEndUTC time.Time) (time.Time, time.Time, int) {
	var intervals [][2]time.Time
	pageToken := ""

	for {
		endpoint := fmt.Sprintf("%s/%s/participantSessions?pageSize=100", meetAPIBase, participantName)
		if pageToken != "" {
			endpoint += "&pageToken=" + pageToken
		}

		var page struct {
			Sessions []struct {
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
			} `json:"participantSessions"`
			NextPageToken string `json:"nextPageToken"`
		}

		if err := c.getJSON(ctx, client, endpoint, &page); err != nil || len(page.Sessions) == 0 {
			break
		}

		for _, s := range page.Sessions {
			startUTC, err1 := time.Parse(time.RFC3339, s.StartTime)
			endUTC, err2 := time.Parse(time.RFC3339, s.EndTime)
			if err1 != nil {
				continue
			}

			startUTC = startUTC.UTC()
			if err2 == nil {
				endUTC = endUTC.UTC()
			} else {
				endUTC = meetEndUTC
				if endUTC.IsZero() {
					endUTC = time.Now().UTC()
				}
			}

			intervals = append(intervals, [2]time.Time{startUTC, endUTC})
		}

		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return calculateDuration(intervals, meetStartUTC, meetEndUTC)
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

	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("Google Meet API (%d): %s", res.StatusCode, string(body))
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func calculateDuration(intervals [][2]time.Time, meetStart, meetEnd time.Time) (time.Time, time.Time, int) {
	if len(intervals) == 0 {
		return time.Time{}, time.Time{}, 0
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i][0].Before(intervals[j][0]) })

	firstJoin := intervals[0][0]
	lastLeave := intervals[0][1]
	var total time.Duration

	for _, it := range intervals {
		if it[1].After(lastLeave) {
			lastLeave = it[1]
		}
		total += it[1].Sub(it[0])
	}
	return firstJoin, lastLeave, int(total.Minutes())
}
