package mirageecs

// APIListResponse is a response of /api/list
type APIListResponse struct {
	Result []APITaskInfo `json:"result"`
}

type APITaskInfo = Information

// APILaunchResponse is a response of /api/launch, and /api/terminate
type APICommonResponse struct {
	Result string `json:"result"`
}

// APIPurgeResponse is a response of /api/purge
type APIPurgeResponse struct {
	Status string `json:"status"`
}

// APIAccessResponse is a response of /api/access
type APIAccessResponse struct {
	Result   string `json:"result"`
	Duration int64  `json:"duration"`
	Sum      int64  `json:"sum"`
}
