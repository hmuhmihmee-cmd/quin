package domain

type Student struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	GoogleSpaceName string `json:"google_space_name"`
	CycleStartDay   int    `json:"cycle_start_day"`
}
