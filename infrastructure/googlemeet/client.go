package googlemeet

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

//go:embed ./../Config/google-oauth.json
var credentials []byte
type Client struct {
	config    *oauth2.Config
	tokenPath string
}

func New( tokenPath, redirectURL string) (*Client, error) {
	if len(credentials) == 0 {
		return nil, errors.New("cấu hình Google OAuth nhúng trong ứng dụng đang trống")
	}
	config, err := google.ConfigFromJSON(credentials, "https://www.googleapis.com/auth/meetings.space.readonly")
	if err != nil {
		return nil, fmt.Errorf("cấu hình Google OAuth không hợp lệ: %w", err)
	}
	config.RedirectURL = redirectURL
	return &Client{config: config, tokenPath: tokenPath}, nil
}

func (c *Client) IsConnected() bool { _, err := c.readToken(); return err == nil }

func (c *Client) AuthorizationURL(state string) (string, error) {
	return c.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

func (c *Client) Exchange(ctx context.Context, code string) error {
	token, err := c.config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("đổi OAuth token: %w", err)
	}
	return c.saveToken(token)
}

func (c *Client) Disconnect() error {
	err := os.Remove(c.tokenPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c *Client) ListMeetings(ctx context.Context, since time.Time) ([]application.ImportedMeeting, error) {
	token, err := c.readToken()
	if err != nil {
		return nil, err
	}
	tokenSource := c.config.TokenSource(ctx, token)
	freshToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("làm mới phiên Google: %w", err)
	}
	if freshToken.AccessToken != token.AccessToken || !freshToken.Expiry.Equal(token.Expiry) {
		_ = c.saveToken(freshToken)
	}
	httpClient := oauth2.NewClient(ctx, tokenSource)
	filter := fmt.Sprintf("start_time >= %q", since.UTC().Format(time.RFC3339))
	var result []application.ImportedMeeting
	pageToken := ""
	for {
		var page ListConferenceRecordsResponse
		values := url.Values{"filter": []string{filter}, "pageSize": []string{"100"}}
		if pageToken != "" {
			values.Set("pageToken", pageToken)
		}
		if err := c.getJSON(ctx, httpClient, "https://meet.googleapis.com/v2/conferenceRecords?"+values.Encode(), &page); err != nil {
			return nil, err
		}
		for _, record := range page.ConferenceRecords {
			startedAt, err := time.Parse(time.RFC3339, record.StartTime)
			if err != nil {
				return nil, fmt.Errorf("thời gian conference record không hợp lệ: %w", err)
			}
			endedAt, _ := time.Parse(time.RFC3339, record.EndTime)
			participants, err := c.listParticipants(ctx, httpClient, record.Name, startedAt, endedAt)
			if err != nil {
				return nil, fmt.Errorf("đọc người tham gia %s: %w", record.Name, err)
			}
			result = append(result, application.ImportedMeeting{
				GoogleRecordName: trimResourcePrefix(record.Name, "conferenceRecords/"),
				GoogleSpaceName:  trimResourcePrefix(record.Space, "spaces/"),
				StartedAt:        startedAt,
				EndedAt:          endedAt,
				Participants:     participants,
			})
		}
		pageToken = page.NextPageToken
		if pageToken == "" {
			return result, nil
		}
	}
}

func (c *Client) listParticipants(ctx context.Context, client *http.Client, recordName string, meetingStart, meetingEnd time.Time) ([]domain.Participant, error) {
	var source []Participant
	pageToken := ""
	for {
		var page ListParticipantsResponse
		values := url.Values{"pageSize": []string{"250"}}
		if pageToken != "" {
			values.Set("pageToken", pageToken)
		}
		endpoint := "https://meet.googleapis.com/v2/" + recordName + "/participants?" + values.Encode()
		if err := c.getJSON(ctx, client, endpoint, &page); err != nil {
			return nil, err
		}
		source = append(source, page.Participants...)
		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}

	result := make([]domain.Participant, 0, len(source))
	for _, person := range source {
		value, err := c.readSessions(ctx, client, person, meetingStart, meetingEnd)
		if err != nil {
			return nil, err
		}
		if !value.JoinedAt.IsZero() {
			result = append(result, value)
		}
	}
	return result, nil
}

func (c *Client) readSessions(ctx context.Context, client *http.Client, person Participant, meetingStart, meetingEnd time.Time) (domain.Participant, error) {
	result := domain.Participant{
		GoogleParticipantName: trimResourcePrefix(person.Name, "conferenceRecords/"),
		GoogleUserName:        person.googleUserName(),
		DisplayName:           person.displayName(),
	}
	var intervals []timeInterval
	pageToken := ""
	for {
		var page ListParticipantSessionsResponse
		values := url.Values{"pageSize": []string{"250"}}
		if pageToken != "" {
			values.Set("pageToken", pageToken)
		}
		endpoint := "https://meet.googleapis.com/v2/" + person.Name + "/participantSessions?" + values.Encode()
		if err := c.getJSON(ctx, client, endpoint, &page); err != nil {
			return result, err
		}
		for _, participantSession := range page.ParticipantSessions {
			start, err := time.Parse(time.RFC3339, participantSession.StartTime)
			if err != nil {
				continue
			}
			end, err := time.Parse(time.RFC3339, participantSession.EndTime)
			if err != nil {
				end = meetingEnd
				if end.IsZero() {
					end = time.Now().UTC()
				}
			}
			intervals = append(intervals, timeInterval{start: start, end: end})
		}
		pageToken = page.NextPageToken
		if pageToken == "" {
			result.JoinedAt, result.LeftAt, result.DurationMinutes = mergedDuration(intervals, meetingStart, meetingEnd)
			return result, nil
		}
	}
}

func trimResourcePrefix(value, prefix string) string {
	return strings.TrimPrefix(value, prefix)
}

type timeInterval struct{ start, end time.Time }

func mergedDuration(intervals []timeInterval, meetingStart, meetingEnd time.Time) (time.Time, time.Time, int64) {
	valid := make([]timeInterval, 0, len(intervals))
	for _, interval := range intervals {
		if interval.start.Before(meetingStart) {
			interval.start = meetingStart
		}
		if !meetingEnd.IsZero() && interval.end.After(meetingEnd) {
			interval.end = meetingEnd
		}
		if interval.end.After(interval.start) {
			valid = append(valid, interval)
		}
	}
	if len(valid) == 0 {
		return time.Time{}, time.Time{}, 0
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].start.Before(valid[j].start) })
	joinedAt := valid[0].start
	currentEnd := valid[0].end
	leftAt := currentEnd
	var total time.Duration
	for _, interval := range valid[1:] {
		if !interval.start.After(currentEnd) {
			if interval.end.After(currentEnd) {
				currentEnd = interval.end
			}
			if currentEnd.After(leftAt) {
				leftAt = currentEnd
			}
			continue
		}
		total += currentEnd.Sub(joinedAt)
		joinedAt = interval.start
		currentEnd = interval.end
		if currentEnd.After(leftAt) {
			leftAt = currentEnd
		}
	}
	total += currentEnd.Sub(joinedAt)
	return valid[0].start, leftAt, int64(total.Minutes())
}

func (c *Client) getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("Google Meet trả về %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func (c *Client) readToken() (*oauth2.Token, error) {
	file, err := os.Open(c.tokenPath)
	if err != nil {
		return nil, errors.New("hãy liên kết tài khoản Google trước")
	}
	defer file.Close()
	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("đọc token: %w", err)
	}
	return &token, nil
}

func (c *Client) saveToken(token *oauth2.Token) error {
	file, err := os.OpenFile(c.tokenPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(token)
}

type ConferenceRecord struct {
	Name      string `json:"name"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Space     string `json:"space"`
}

type ListConferenceRecordsResponse struct {
	ConferenceRecords []ConferenceRecord `json:"conferenceRecords"`
	NextPageToken     string             `json:"nextPageToken"`
}

type SignedInUser struct {
	User        string `json:"user"`
	DisplayName string `json:"displayName"`
}

type AnonymousUser struct {
	DisplayName string `json:"displayName"`
}

type Participant struct {
	Name          string         `json:"name"`
	SignedInUser  *SignedInUser  `json:"signedInUser"`
	AnonymousUser *AnonymousUser `json:"anonymousUser"`
}

func (p Participant) displayName() string {
	if p.SignedInUser != nil && p.SignedInUser.DisplayName != "" {
		return p.SignedInUser.DisplayName
	}
	if p.AnonymousUser != nil && p.AnonymousUser.DisplayName != "" {
		return p.AnonymousUser.DisplayName
	}
	return "Khách ẩn danh"
}

func (p Participant) googleUserName() string {
	if p.SignedInUser == nil {
		return ""
	}
	return p.SignedInUser.User
}

type ListParticipantsResponse struct {
	Participants  []Participant `json:"participants"`
	NextPageToken string        `json:"nextPageToken"`
	TotalSize     int           `json:"totalSize"`
}

type ParticipantSession struct {
	Name      string `json:"name"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type ListParticipantSessionsResponse struct {
	ParticipantSessions []ParticipantSession `json:"participantSessions"`
	NextPageToken       string               `json:"nextPageToken"`
}
