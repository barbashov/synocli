package downloadstation

// MapTask converts a Task to a map suitable for JSON output.
func MapTask(t Task) map[string]any {
	m := map[string]any{
		"task_id":           t.ID,
		"title":             t.Title,
		"normalized_status": NormalizeStatus(t.Status),
		"raw_status":        t.Status,
		"status_enum":       StatusEnum(t.Status),
		"status_display":    StatusDisplay(t.Status),
		"status_extra":      t.StatusExtra,
		"type":              t.Type,
		"username":          t.Username,
		"destination":       DestinationOf(t),
		"uri":               URIOf(t),
		"size":              t.Size,
		"downloaded_size":   DownloadedOf(t),
		"uploaded_size":     UploadedOf(t),
		"download_speed":    DownSpeedOf(t),
		"upload_speed":      UpSpeedOf(t),
		"eta_seconds":       ETASecondsOf(t),
		"created_time":      CreatedOf(t),
		"completed_time":    CompletedOf(t),
		"error_detail":      ErrorDetailOf(t),
		"tracker":           TrackerOf(t),
		"peer":              PeerOf(t),
		"file":              FileOf(t),
	}
	if code, ok := StatusCode(t.Status); ok {
		m["status_code"] = code
	}
	return m
}

// MapTasks converts a slice of Tasks to a slice of maps.
func MapTasks(tasks []Task) []map[string]any {
	out := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, MapTask(t))
	}
	return out
}

// DestinationOf returns the download destination path if available.
func DestinationOf(t Task) string {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.Destination
	}
	return ""
}

// URIOf returns the download URI if available.
func URIOf(t Task) string {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.URI
	}
	return ""
}

// CreatedOf returns the task creation time as a Unix timestamp.
func CreatedOf(t Task) int64 {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.CreateTime
	}
	return 0
}

// CompletedOf returns the task completion time as a Unix timestamp.
func CompletedOf(t Task) int64 {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.CompletedTime
	}
	return 0
}

// ErrorDetailOf returns the error detail string if the task failed.
func ErrorDetailOf(t Task) string {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.ErrorDetail
	}
	return ""
}

// DownloadedOf returns the downloaded byte count.
func DownloadedOf(t Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SizeDownloaded
	}
	return 0
}

// UploadedOf returns the uploaded byte count.
func UploadedOf(t Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SizeUploaded
	}
	return 0
}

// DownSpeedOf returns the download speed in bytes/s.
func DownSpeedOf(t Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SpeedDownload
	}
	return 0
}

// UpSpeedOf returns the upload speed in bytes/s.
func UpSpeedOf(t Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SpeedUpload
	}
	return 0
}

// ETASecondsOf returns estimated remaining download time in seconds.
// Returns -1 when ETA cannot be estimated.
func ETASecondsOf(t Task) int64 {
	if t.Size <= 0 {
		return -1
	}
	remaining := t.Size - DownloadedOf(t)
	if remaining <= 0 {
		return 0
	}
	speed := DownSpeedOf(t)
	if speed <= 0 {
		return -1
	}
	return (remaining + speed - 1) / speed
}

// TrackerOf returns the tracker info, or nil.
func TrackerOf(t Task) any {
	if t.Additional != nil {
		return t.Additional.Tracker
	}
	return nil
}

// PeerOf returns the peer info, or nil.
func PeerOf(t Task) any {
	if t.Additional != nil {
		return t.Additional.Peer
	}
	return nil
}

// FileOf returns the file info, or nil.
func FileOf(t Task) any {
	if t.Additional != nil {
		return t.Additional.File
	}
	return nil
}
