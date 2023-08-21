package mirageecs

import (
	"encoding/json"
	"net/url"
)

// APIListResponse is a response of /api/list
type APIListResponse struct {
	Result []APITaskInfo `json:"result"`
}

type APITaskInfo = Information

// APILaunchResponse is a response of /api/launch, and /api/terminate
type APICommonResponse struct {
	Result string `json:"result"`
}

type APILogsResponse struct {
	Result []string `json:"result"`
}

// APIAccessResponse is a response of /api/access
type APIAccessResponse struct {
	Result   string `json:"result"`
	Duration int64  `json:"duration"`
	Sum      int64  `json:"sum"`
}

type APILaunchRequest struct {
	Subdomain  string            `json:"subdomain" form:"subdomain"`
	Branch     string            `json:"branch" form:"branch"`
	Taskdef    []string          `json:"taskdef" form:"taskdef"`
	Parameters map[string]string `json:"parameters" form:"parameters"`
}

func (r *APILaunchRequest) GetParameter(key string) string {
	if key == "branch" {
		return r.Branch
	}
	return r.Parameters[key]
}

func (r *APILaunchRequest) MergeForm(form url.Values) {
	if r.Parameters == nil {
		r.Parameters = make(map[string]string, len(form))
	}
	for key, values := range form {
		if key == "branch" || key == "subdomain" || key == "taskdef" {
			continue
		}
		r.Parameters[key] = values[0]
	}
}

type APIPurgeRequest struct {
	Duration    json.Number `json:"duration" form:"duration"`
	Excludes    []string    `json:"excludes" form:"excludes"`
	ExcludeTags []string    `json:"exclude_tags" form:"exclude_tags"`
}

type APITerminateRequest struct {
	ID        string `json:"id" form:"id"`
	Subdomain string `json:"subdomain" form:"subdomain"`
}
