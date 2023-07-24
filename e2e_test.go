package mirageecs_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs"
)

func TestE2EAPI(t *testing.T) {
	cfg, err := mirageecs.NewConfig(&mirageecs.ConfigParams{
		LocalMode: true,
		Domain:    "localtest.me",
	})
	if err != nil {
		t.Error(err)
	}
	m := mirageecs.Setup(cfg)
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
		v := url.Values{}
		v.Add("subdomain", "mytask")
		v.Add("taskdef", "dummy")
		v.Add("branch", "develop")
		req, _ := http.NewRequest("POST", ts.URL+"/api/launch", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		if r.Result[0].Subdomain != "mytask" {
			t.Errorf("subdomain should be mytask %#v", r)
		}
		if r.Result[0].Taskdef != "dummy" {
			t.Errorf("taskdef should be dummy %#v", r)
		}
		if r.Result[0].Branch != "develop" {
			t.Errorf("branch should be develop %#v", r)
		}
	})

	t.Run("/api/purge", func(t *testing.T) {
		v := url.Values{}
		v.Add("duration", "300")
		req, _ := http.NewRequest("POST", ts.URL+"/api/purge", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		var r mirageecs.APIPurgeResponse
		json.NewDecoder(res.Body).Decode(&r)
		if r.Status != "ok" {
			t.Errorf("result should be ok %#v", r)
		}
	})

	t.Run("/api/terminate", func(t *testing.T) {
		v := url.Values{}
		v.Add("subdomain", "mytask")
		req, _ := http.NewRequest("POST", ts.URL+"/api/terminate", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
