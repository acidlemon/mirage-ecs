package mirageecs_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
)

var e2eRequestsForm = map[string]string{
	"/api/launch": url.Values{
		"subdomain": []string{"mytask"},
		"taskdef":   []string{"dummy"},
		"branch":    []string{"develop"},
		"env":       []string{"test"},
	}.Encode(),
	"/api/purge": url.Values{
		"duration": []string{"300"},
	}.Encode(),
	"/api/terminate": url.Values{
		"subdomain": []string{"mytask"},
	}.Encode(),
}

var e2eRequestsJSON = map[string]string{
	"/api/launch":    `{"subdomain":"mytask","taskdef":["dummy"],"branch":"develop","parameters":{"env":"test"}}`,
	"/api/purge":     `{"duration":"300"}`,
	"/api/terminate": `{"subdomain":"mytask"}`,
}

func TestE2EAPI(t *testing.T) {
	t.Run("form", func(t *testing.T) {
		testE2EAPI(t, e2eRequestsForm, "application/x-www-form-urlencoded")
	})
	t.Run("json", func(t *testing.T) {
		testE2EAPI(t, e2eRequestsJSON, "application/json")
	})
}

func testE2EAPI(t *testing.T, reqs map[string]string, contentType string) {
	ctx := context.Background()
	cfg, err := mirageecs.NewConfig(ctx, &mirageecs.ConfigParams{
		LocalMode: true,
		Domain:    "localtest.me",
	})
	cfg.Parameter = append(cfg.Parameter, &mirageecs.Parameter{
		Name:     "env",
		Env:      "ENV",
		Required: true,
	})

	if err != nil {
		t.Error(err)
	}
	m := mirageecs.New(context.Background(), cfg)
	ts := httptest.NewServer(m.WebApi)
	defer ts.Close()
	client := ts.Client()

	t.Run("/api/list at first", func(t *testing.T) {
		res, err := client.Get(ts.URL + "/api/list")
		if err != nil {
			t.Error(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("status code should be 200: %d", res.StatusCode)
		}
		var r mirageecs.APIListResponse
		json.NewDecoder(res.Body).Decode(&r)
		if len(r.Result) != 0 {
			t.Errorf("result should be empty %#v", r)
		}
	})

	t.Run("/api/launch", func(t *testing.T) {
		req, _ := http.NewRequest("POST", ts.URL+"/api/launch", strings.NewReader(reqs["/api/launch"]))
		req.Header.Set("Content-Type", contentType)
		res, err := client.Do(req)
		if err != nil {
			t.Error(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			body, _ := io.ReadAll(res.Body)
			t.Errorf("status code should be 200: %d", res.StatusCode)
			t.Errorf("body: %s", body)
			return
		}
		var r mirageecs.APICommonResponse
		json.NewDecoder(res.Body).Decode(&r)
		if r.Result != "ok" {
			t.Errorf("result should be ok %#v", r)
		}
	})

	t.Run("/api/list after launched", func(t *testing.T) {
		res, err := client.Get(ts.URL + "/api/list")
		if err != nil {
			t.Error(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("status code should be 200: %d", res.StatusCode)
		}
		var r mirageecs.APIListResponse
		json.NewDecoder(res.Body).Decode(&r)
		if len(r.Result) != 1 {
			t.Errorf("result should be empty %#v", r)
		}
		if r.Result[0].SubDomain != "mytask" {
			t.Errorf("subdomain should be mytask %#v", r)
		}
		if r.Result[0].TaskDef != "dummy" {
			t.Errorf("taskdef should be dummy %#v", r)
		}
		if r.Result[0].GitBranch != "develop" {
			t.Errorf("branch should be develop %#v", r)
		}
	})

	t.Run("/api/access", func(t *testing.T) {
		res, err := client.Get(ts.URL + "/api/access?subdomain=mytask&duration=300")
		if err != nil {
			t.Error(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("status code should be 200: %d", res.StatusCode)
		}
		var r mirageecs.APIAccessResponse
		json.NewDecoder(res.Body).Decode(&r)
		if r.Result != "ok" {
			t.Errorf("result should be ok %#v", r)
		}
		if r.Duration != 300 {
			t.Errorf("duration should be 300 %#v", r)
		}
	})

	t.Run("/api/purge", func(t *testing.T) {
		req, _ := http.NewRequest("POST", ts.URL+"/api/purge", strings.NewReader(reqs["/api/purge"]))
		req.Header.Set("Content-Type", contentType)
		res, err := client.Do(req)
		if err != nil {
			t.Error(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			body, _ := io.ReadAll(res.Body)
			t.Errorf("status code should be 200: %d", res.StatusCode)
			t.Errorf("body: %s", body)
			return
		}
		var r mirageecs.APICommonResponse
		json.NewDecoder(res.Body).Decode(&r)
		if r.Result != "accepted" {
			t.Errorf("result should be ok %#v", r)
		}
	})

	t.Run("/api/terminate", func(t *testing.T) {
		req, _ := http.NewRequest("POST", ts.URL+"/api/terminate", strings.NewReader(reqs["/api/terminate"]))
		req.Header.Set("Content-Type", contentType)
		res, err := client.Do(req)
		if err != nil {
			t.Error(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			body, _ := io.ReadAll(res.Body)
			t.Errorf("status code should be 200: %d", res.StatusCode)
			t.Errorf("body: %s", body)
			return
		}
		var r mirageecs.APICommonResponse
		json.NewDecoder(res.Body).Decode(&r)
		if r.Result != "ok" {
			t.Errorf("result should be ok %#v", r)
		}
	})

	t.Run("/api/list after terminate", func(t *testing.T) {
		res, err := client.Get(ts.URL + "/api/list")
		if err != nil {
			t.Error(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("status code should be 200: %d", res.StatusCode)
		}
		var r mirageecs.APIListResponse
		json.NewDecoder(res.Body).Decode(&r)
		if len(r.Result) != 0 {
			t.Errorf("result should be empty %#v", r)
		}
	})
}
