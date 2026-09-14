package domain

type Workspace struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Chapters []Chapter `json:"chapters"`
}
type Chapter struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Pages []Page `json:"pages"`
}
type Page struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
