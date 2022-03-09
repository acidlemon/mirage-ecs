package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/acidlemon/rocket.v2"
)

var DNSNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]$`)

type WebApi struct {
	rocket.WebApp
	cfg *Config
}

func NewWebApi(cfg *Config) *WebApi {
	app := &WebApi{}
	app.Init()
	app.cfg = cfg

	view := &rocket.View{
		BasicTemplates: []string{cfg.HtmlDir + "/layout.html"},
	}

	app.AddRoute("/", app.List, view)
	app.AddRoute("/launcher", app.Launcher, view)
	app.AddRoute("/launch", app.Launch, view)
	app.AddRoute("/terminate", app.Terminate, view)
	app.AddRoute("/api/list", app.ApiList, view)
	app.AddRoute("/api/launch", app.ApiLaunch, view)
	app.AddRoute("/api/logs", app.ApiLogs, view)
	app.AddRoute("/api/terminate", app.ApiTerminate, view)

	app.BuildRouter()

	return app
}

func (api *WebApi) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	api.Handler(w, req)
}

func (api *WebApi) List(c rocket.CtxData) {
	info, err := app.ECS.List(statusRunning)
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	value := rocket.RenderVars{
		"info":  info,
		"error": errStr,
	}

	c.Render(api.cfg.HtmlDir+"/list.html", value)
}

func (api *WebApi) Launcher(c rocket.CtxData) {
	var taskdefs []string
	if api.cfg.Link.DefaultTaskDefinitions != nil {
		taskdefs = api.cfg.Link.DefaultTaskDefinitions
	} else {
		taskdefs = []string{api.cfg.ECS.DefaultTaskDefinition}
	}
	c.Render(api.cfg.HtmlDir+"/launcher.html", rocket.RenderVars{
		"DefaultTaskDefinitions": taskdefs,
		"Parameters":             api.cfg.Parameter,
	})
}

func (api *WebApi) Launch(c rocket.CtxData) {
	result := api.launch(c)
	if result["result"] == "ok" {
		c.Redirect("/")
	} else {
		c.RenderJSON(result)
	}
}

func (api *WebApi) Terminate(c rocket.CtxData) {
	result := api.terminate(c)
	if result["result"] == "ok" {
		c.Redirect("/")
	} else {
		c.RenderJSON(result)
	}
}

func (api *WebApi) ApiList(c rocket.CtxData) {
	info, err := app.ECS.List(statusRunning)
	var status interface{}
	if err != nil {
		status = err.Error()
	} else {
		status = info
	}

	result := rocket.RenderVars{
		"result": status,
	}

	c.RenderJSON(result)
}

func (api *WebApi) ApiLaunch(c rocket.CtxData) {
	result := api.launch(c)

	c.RenderJSON(result)
}

func (api *WebApi) ApiLogs(c rocket.CtxData) {
	result := api.logs(c)

	c.RenderJSON(result)
}

func (api *WebApi) ApiTerminate(c rocket.CtxData) {
	result := api.terminate(c)

	c.RenderJSON(result)
}

func (api *WebApi) launch(c rocket.CtxData) rocket.RenderVars {
	if c.Req().Method != "POST" {
		c.Res().StatusCode = http.StatusMethodNotAllowed
		c.RenderText("you must use POST")
		return rocket.RenderVars{}
	}

	subdomain, _ := c.ParamSingle("subdomain")
	subdomain = strings.ToLower(subdomain)
	if err := validateSubdomain(subdomain); err != nil {
		c.Res().StatusCode = http.StatusBadRequest
		c.RenderText(err.Error())
		return rocket.RenderVars{}
	}

	taskdefs, _ := c.Param("taskdef")

	parameter, err := api.loadParameter(c)
	if err != nil {
		result := rocket.RenderVars{
			"result": err.Error(),
		}

		return result
	}

	status := "ok"

	if subdomain == "" || len(taskdefs) == 0 {
		status = fmt.Sprintf("parameter required: subdomain=%s, taskdef=%v", subdomain, taskdefs)
	} else {
		err := app.ECS.Launch(subdomain, parameter, taskdefs...)
		if err != nil {
			status = err.Error()
		}
	}

	result := rocket.RenderVars{
		"result": status,
	}

	return result
}

func (api *WebApi) logs(c rocket.CtxData) rocket.RenderVars {
	if c.Req().Method != "GET" {
		c.Res().StatusCode = http.StatusMethodNotAllowed
		c.RenderText("you must use GET")
		return rocket.RenderVars{}
	}

	subdomain, _ := c.ParamSingle("subdomain")
	since, _ := c.ParamSingle("since")
	tail, _ := c.ParamSingle("tail")

	if subdomain == "" {
		return rocket.RenderVars{
			"result": "parameter required: subdomain",
		}
	}

	var sinceTime time.Time
	if since != "" {
		var err error
		sinceTime, err = time.Parse(time.RFC3339, since)
		if err != nil {
			return rocket.RenderVars{
				"result": fmt.Sprintf("cannot parse since: %s", err),
			}
		}
	}
	var tailN int
	if tail != "" {
		if tail == "all" {
			tailN = 0
		} else if n, err := strconv.Atoi(tail); err != nil {
			return rocket.RenderVars{
				"result": fmt.Sprintf("cannot parse tail: %s", err),
			}
		} else {
			tailN = n
		}
	}

	logs, err := app.ECS.Logs(subdomain, sinceTime, tailN)
	if err != nil {
		return rocket.RenderVars{
			"result": err.Error(),
		}
	}
	return rocket.RenderVars{
		"result": logs,
	}
}

func (api *WebApi) terminate(c rocket.CtxData) rocket.RenderVars {
	if c.Req().Method != "POST" {
		c.Res().StatusCode = http.StatusMethodNotAllowed
		c.RenderText("you must use POST")
		return rocket.RenderVars{}
	}

	status := "ok"

	id, _ := c.ParamSingle("id")
	subdomain, _ := c.ParamSingle("subdomain")
	if id != "" {
		if err := app.ECS.Terminate(id); err != nil {
			status = err.Error()
		}
	} else if subdomain != "" {
		if err := app.ECS.TerminateBySubdomain(subdomain); err != nil {
			status = err.Error()
		}
	} else {
		status = fmt.Sprintf("parameter required: id")
	}

	result := rocket.RenderVars{
		"result": status,
	}

	return result
}

func (api *WebApi) loadParameter(c rocket.CtxData) (map[string]string, error) {
	var parameter map[string]string = make(map[string]string)

	for _, v := range api.cfg.Parameter {
		param, _ := c.ParamSingle(v.Name)
		if param == "" && v.Required == true {
			return nil, fmt.Errorf("lack require parameter: %s", v.Name)
		} else if param == "" {
			continue
		}

		if v.Rule != "" {
			if !v.Regexp.MatchString(param) {
				return nil, fmt.Errorf("parameter %s value is rule error", v.Name)
			}
		}

		parameter[v.Name] = param
	}

	return parameter, nil
}

const rsLetters = "0123456789abcdef"

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = rsLetters[rand.Intn(len(rsLetters))]
	}
	return string(b)
}

func validateSubdomain(s string) error {
	if !DNSNameRegexp.MatchString(s) {
		return errors.New("subdomain format is invalid")
	}
	return nil
}
