package handlers

import "net/http"

type SyncMeet struct{ job *SyncJob }

func NewSyncMeet(job *SyncJob) *SyncMeet { return &SyncMeet{job: job} }
func (h *SyncMeet) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h.job.Start() {
		redirectNotice(writer, request, "Đã bắt đầu đồng bộ Google Meet.")
		return
	}
	redirectNotice(writer, request, "Google Meet đang được đồng bộ.")
}
