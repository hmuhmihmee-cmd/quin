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

// Khai báo múi giờ Việt Nam (UTC+7) dùng cho toàn bộ package hạ tầng này
var vnLocation = time.FixedZone("ICT", 7*3600)

type Client struct {
	config    *oauth2.Config
	tokenPath string
}

func New(credentials []byte, tokenPath, redirectURL string) (*Client, error) {
	if len(credentials) == 0 {
		return nil, errors.New("cấu hình Google OAuth nhúng trong ứng dụng đang trống")
	}
	cfg, err := google.ConfigFromJSON(credentials, "https://www.googleapis.com/auth/meetings.space.readonly")
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
	if err!=nil{
		return nil,err
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

	// 1. CHIỀU ĐI: Nhận giờ UTC+7 từ Application -> Đổi sang UTC để gửi cho Google
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
			// 2. CHIỀU VỀ: Nhận UTC từ Google -> Đổi ngay sang UTC+7 trước khi tạo domain.Meeting
			startedAtUTC, err := time.Parse(time.RFC3339, rec.StartTime)
			if err != nil {
				continue
			}
			startedAtVN := startedAtUTC.In(vnLocation)

			var endedAtVN time.Time
			if rec.EndTime != "" {
				if t, err := time.Parse(time.RFC3339, rec.EndTime); err == nil {
					endedAtVN = t.In(vnLocation)
				}
			}

			participants, err := c.listParticipants(ctx, client, rec.Name, startedAtVN, endedAtVN)
			if err != nil {
				return nil, fmt.Errorf("lấy người tham gia %s: %w", rec.Name, err)
			}

			meetings = append(meetings, domain.Meeting{
				ID:           strings.TrimPrefix(rec.Name, "conferenceRecords/"),
				Class:        strings.TrimPrefix(rec.Space, "spaces/"),
				StartedAt:    startedAtVN, // Trả về giờ VN thuần túy
				EndedAt:      endedAtVN,   // Trả về giờ VN thuần túy
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

func (c *Client) listParticipants(ctx context.Context, client *http.Client, recordName string, meetStartVN, meetEndVN time.Time) ([]domain.Participant, error) {
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

			firstJoinVN, lastLeaveVN, duration := c.readSessions(ctx, client, p.Name, meetStartVN, meetEndVN)
			if !firstJoinVN.IsZero() {
				participants = append(participants, domain.Participant{
					Name:          displayName,
					FirstJoinedAt: firstJoinVN, // Giờ VN
					LastLeftAt:    lastLeaveVN,  // Giờ VN
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

func (c *Client) readSessions(ctx context.Context, client *http.Client, participantName string, meetStartVN, meetEndVN time.Time) (time.Time, time.Time, int) {
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

			// Chuyển session về giờ VN
			startVN := startUTC.In(vnLocation)
			var endVN time.Time
			if err2 == nil {
				endVN = endUTC.In(vnLocation)
			} else {
				endVN = meetEndVN
				if endVN.IsZero() {
					endVN = time.Now().In(vnLocation)
				}
			}

			intervals = append(intervals, [2]time.Time{startVN, endVN})
		}

		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return calculateDuration(intervals, meetStartVN, meetEndVN)
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