package mirageecs

import "time"

// APIListResponse is a response of /api/list
type APIListResponse struct {
	Result []APITaskInfo `json:"result"`
}

type APITaskInfo struct {
	ID         string            `json:"id"`
	ShortID    string            `json:"short_id"`
	Subdomain  string            `json:"subdomain"`
	Branch     string            `json:"branch"`
	Taskdef    string            `json:"taskdef"`
	IPAddress  string            `json:"ipaddress"`
	Created    time.Time         `json:"created"`
	LastStatus string            `json:"last_status"`
	PortMap    map[string]int    `json:"port_map"`
	Env        map[string]string `json:"env"`
}

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
	Duration int    `json:"duration"`
	Sum      int    `json:"sum"`
}
