package main

import (
	"fmt"
	"log"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"gopkg.in/acidlemon/rocket.v2"
)

var DNSNameRegexpWithPattern = regexp.MustCompile(`^[a-zA-Z*?\[\]][a-zA-Z0-9-*?\[\]]{0,61}[a-zA-Z0-9*?\[\]]$`)

const PurgeMinimumDuration = 5 * time.Minute

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
	app.AddRoute("/api/access", app.ApiAccess, view)
	app.AddRoute("/api/purge", app.ApiPurge, view)

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

func (api *WebApi) ApiAccess(c rocket.CtxData) {
	result := api.accessCounter(c)
	c.RenderJSON(result)
}

func (api *WebApi) ApiPurge(c rocket.CtxData) {
	result := api.purge(c)
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
			log.Println("[error] launch failed: ", err)
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
		status = "parameter required: id"
	}

	result := rocket.RenderVars{
		"result": status,
	}

	return result
}

func (api *WebApi) accessCounter(c rocket.CtxData) rocket.RenderVars {
	subdomain, _ := c.ParamSingle("subdomain")
	duration, _ := c.ParamSingle("duration")
	durationInt, _ := strconv.ParseInt(duration, 10, 64)
	if durationInt == 0 {
		durationInt = 86400 // 24 hours
	}
	d := time.Duration(durationInt) * time.Second
	sum, err := app.GetAccessCount(subdomain, d)
	if err != nil {
		c.Res().StatusCode = http.StatusInternalServerError
		log.Println("[error] access counter failed: ", err)
		return rocket.RenderVars{
			"result": err.Error(),
		}
	}
	return rocket.RenderVars{
		"result":   "ok",
		"duration": durationInt,
		"sum":      sum,
	}
}

func (api *WebApi) loadParameter(c rocket.CtxData) (map[string]string, error) {
	var parameter map[string]string = make(map[string]string)

	for _, v := range api.cfg.Parameter {
		param, _ := c.ParamSingle(v.Name)
		if param == "" && v.Default != "" {
			param = v.Default
		}
		if param == "" && v.Required {
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

func validateSubdomain(s string) error {
	if !DNSNameRegexpWithPattern.MatchString(s) {
		return fmt.Errorf("subdomain includes invalid characters")
	}
	if _, err := path.Match(s, "x"); err != nil {
		return err
	}
	return nil
}

func (api *WebApi) purge(c rocket.CtxData) rocket.RenderVars {
	if c.Req().Method != "POST" {
		c.Res().StatusCode = http.StatusMethodNotAllowed
		c.RenderText("you must use POST")
		return rocket.RenderVars{}
	}

	excludes, _ := c.Param("excludes")
	d, _ := c.ParamSingle("duration")
	di, err := strconv.ParseInt(d, 10, 64)
	if err != nil || di <= int64(PurgeMinimumDuration.Seconds()) {
		c.Res().StatusCode = http.StatusBadRequest
		msg := fmt.Sprintf("[error] invalid duration (at least %d): %s %s", PurgeMinimumDuration, d, err)
		log.Println(msg)
		return rocket.RenderVars{
			"result": msg,
		}
	}

	excludesMap := make(map[string]struct{}, len(excludes))
	for _, exclude := range excludes {
		excludesMap[exclude] = struct{}{}
	}
	duration := time.Duration(di) * time.Second
	begin := time.Now().Add(-duration)

	infos, err := app.ECS.List(statusRunning)
	if err != nil {
		c.Res().StatusCode = http.StatusInternalServerError
		log.Println("[error] list ecs failed: ", err)
		return rocket.RenderVars{
			"result": err.Error(),
		}
	}
	tm := make(map[string]struct{}, len(infos))
	for _, info := range infos {
		if _, ok := excludesMap[info.SubDomain]; ok {
			log.Printf("[info] skip exclude subdomain: %s", info.SubDomain)
			continue
		}
		if info.Created.After(begin) {
			log.Printf("[info] skip recent created subdomain: %s %s", info.SubDomain, info.Created.Format(time.RFC3339))
			continue
		}
		tm[info.SubDomain] = struct{}{}
	}
	terminates := lo.Keys(tm)
	if len(terminates) > 0 {
		go purgeSubdomains(terminates, duration)
	}

	return rocket.RenderVars{
		"status": "ok",
	}
}

func purgeSubdomains(subdomains []string, duration time.Duration) {
	if app.TryLock() {
		defer app.Unlock()
	} else {
		log.Println("[info] skip purge subdomains, another purge is running")
		return
	}
	log.Printf("[info] start purge subdomains %d", len(subdomains))
	purged := 0
	for _, subdomain := range subdomains {
		sum, err := app.GetAccessCount(subdomain, duration)
		if err != nil {
			log.Printf("[warn] access count failed: %s %s", subdomain, err)
			continue
		}
		if sum > 0 {
			log.Printf("[info] skip purge %s %d access", subdomain, sum)
			continue
		}
		if err := app.ECS.TerminateBySubdomain(subdomain); err != nil {
			log.Printf("[warn] terminate failed %s %s", subdomain, err)
		} else {
			purged++
			log.Printf("[info] purged %s", subdomain)
		}
		time.Sleep(3 * time.Second)
	}
	log.Printf("[info] purge %d subdomains completed", purged)
}
