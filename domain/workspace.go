package domain

type Workspace struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Chapters []Chapter `json:"chapters"`
}
type Chapter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Pages []Page `json:"pages"` 
}
type Page struct{
	ID string `json:"id"`
	Name string `json:"name"`
}

// type PublishTarget struct {
// 	WorkspaceID string   `json:"workspace_id"`  // Vở ghi nhận bài
// 	ChapterName string   `json:"chapter_name"`  // Tên chương (nếu chưa có hạ tầng tự tạo)
// 	Title       string   `json:"session_title"` // Tiêu đề buổi học (VD: "Buổi 1: Lượng giác - 15/08")
// 	TargetRole  Audience `json:"target_role"`   // Cho Giáo viên hay Học sinh
// }
