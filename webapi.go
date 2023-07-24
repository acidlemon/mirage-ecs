package mirageecs

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"gopkg.in/acidlemon/rocket.v2"
)

var DNSNameRegexpWithPattern = regexp.MustCompile(`^[a-zA-Z*?\[\]][a-zA-Z0-9-*?\[\]]{0,61}[a-zA-Z0-9*?\[\]]$`)

const PurgeMinimumDuration = 5 * time.Minute

type WebApi struct {
	rocket.WebApp
	cfg    *Config
	mirage *Mirage
	echo   *echo.Echo
}

func NewWebApi(cfg *Config, m *Mirage) *WebApi {
	app := &WebApi{
		mirage: m,
	}
	app.Init()
	app.cfg = cfg

	e := echo.New()
	e.GET("/api/list", app.ApiList)
	e.POST("/api/launch", app.ApiLaunch)
	e.POST("/api/terminate", app.ApiTerminate)
	e.POST("/api/purge", app.ApiPurge)
	e.GET("/api/access", app.ApiAccess)
	e.GET("/api/logs", app.ApiLogs)
	app.echo = e

	app.BuildRouter()

	return app
}

func (api *WebApi) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, "/api") {
		api.echo.ServeHTTP(w, req)
	} else {
		api.Handler(w, req)
	}
}

func (api *WebApi) List(c rocket.CtxData) {
	info, err := api.mirage.ECS.List(statusRunning)
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
	/*
		result := api.launch()
		if result["result"] == "ok" {
			c.Redirect("/")
		} else {
			c.RenderJSON(result)
		}
	*/
}

func (api *WebApi) Terminate(c echo.Context) error {
	code, err := api.terminate(c)
	if err != nil {
		c.JSON(code, APICommonResponse{Result: err.Error()})
	}
	return c.Redirect(http.StatusFound, "/")
}

func (api *WebApi) ApiList(c echo.Context) error {
	info, err := api.mirage.ECS.List(statusRunning)
	if err != nil {
		return c.JSON(500, APIListResponse{})
	}
	return c.JSON(200, APIListResponse{Result: info})
}

func (api *WebApi) ApiLaunch(c echo.Context) error {
	code, err := api.launch(c)
	if err != nil {
		return c.JSON(code, APICommonResponse{Result: err.Error()})
	}
	return c.JSON(code, APICommonResponse{Result: "ok"})
}

func (api *WebApi) launch(c echo.Context) (int, error) {
	subdomain := c.FormValue("subdomain")
	subdomain = strings.ToLower(subdomain)
	if err := validateSubdomain(subdomain); err != nil {
		log.Println("[error] launch failed: ", err)
		return http.StatusBadRequest, err
	}

	ps, err := c.FormParams()
	if err != nil {
		log.Println("[error] failed to get form params: ", err)
		return http.StatusInternalServerError, err
	}
	taskdefs := ps["taskdef"]

	parameter, err := api.LoadParameter(c.FormValue)
	if err != nil {
		log.Println("[error] failed to load parameter: ", err)
		return http.StatusBadRequest, err
	}

	if subdomain == "" || len(taskdefs) == 0 {
		return http.StatusBadRequest, fmt.Errorf("parameter required: subdomain=%s, taskdef=%v", subdomain, taskdefs)
	} else {
		err := api.mirage.ECS.Launch(subdomain, parameter, taskdefs...)
		if err != nil {
			log.Println("[error] launch failed: ", err)
			return http.StatusInternalServerError, err
		}
	}

	return http.StatusOK, nil
}

func (api *WebApi) ApiLogs(c echo.Context) error {
	code, logs, err := api.logs(c)
	if err != nil {
		return c.JSON(code, APICommonResponse{Result: err.Error()})
	}
	return c.JSON(code, APILogsResponse{Result: logs})
}

func (api *WebApi) ApiTerminate(c echo.Context) error {
	code, err := api.terminate(c)
	if err != nil {
		return c.JSON(code, APICommonResponse{Result: err.Error()})
	}
	return c.JSON(code, APICommonResponse{Result: "ok"})
}

func (api *WebApi) ApiAccess(c echo.Context) error {
	code, sum, duration, err := api.accessCounter(c)
	if err != nil {
		return c.JSON(code, APICommonResponse{Result: err.Error()})
	}
	return c.JSON(code, APIAccessResponse{Result: "ok", Sum: sum, Duration: duration})
}

func (api *WebApi) ApiPurge(c echo.Context) error {
	code, err := api.purge(c)
	if err != nil {
		return c.JSON(code, APICommonResponse{Result: err.Error()})
	}
	return c.JSON(code, APIPurgeResponse{Status: "ok"})
}

func (api *WebApi) logs(c echo.Context) (int, []string, error) {
	subdomain := c.QueryParam("subdomain")
	since := c.QueryParam("since")
	tail := c.QueryParam("tail")

	if subdomain == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("parameter required: subdomain")
	}

	var sinceTime time.Time
	if since != "" {
		var err error
		sinceTime, err = time.Parse(time.RFC3339, since)
		if err != nil {
			return http.StatusBadRequest, nil, fmt.Errorf("cannot parse since: %s", err)
		}
	}
	var tailN int
	if tail != "" {
		if tail == "all" {
			tailN = 0
		} else if n, err := strconv.Atoi(tail); err != nil {
			return http.StatusBadRequest, nil, fmt.Errorf("cannot parse tail: %s", err)
		} else {
			tailN = n
		}
	}

	logs, err := api.mirage.ECS.Logs(subdomain, sinceTime, tailN)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, logs, nil
}

func (api *WebApi) terminate(c echo.Context) (int, error) {
	id := c.FormValue("id")
	subdomain := c.FormValue("subdomain")
	if id != "" {
		if err := api.mirage.ECS.Terminate(id); err != nil {
			return http.StatusInternalServerError, err
		}
	} else if subdomain != "" {
		if err := api.mirage.ECS.TerminateBySubdomain(subdomain); err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		return http.StatusBadRequest, fmt.Errorf("parameter required: id or subdomain")
	}
	return http.StatusOK, nil
}

func (api *WebApi) accessCounter(c echo.Context) (int, int64, int64, error) {
	subdomain := c.QueryParam("subdomain")
	duration := c.QueryParam("duration")
	durationInt, _ := strconv.ParseInt(duration, 10, 64)
	if durationInt == 0 {
		durationInt = 86400 // 24 hours
	}
	d := time.Duration(durationInt) * time.Second
	sum, err := api.mirage.GetAccessCount(subdomain, d)
	if err != nil {
		log.Println("[error] access counter failed: ", err)
		return http.StatusInternalServerError, 0, durationInt, err
	}
	return http.StatusOK, sum, durationInt, nil
}

func (api *WebApi) LoadParameter(getFunc func(string) string) (TaskParameter, error) {
	parameter := make(TaskParameter)

	for _, v := range api.cfg.Parameter {
		param := getFunc(v.Name)
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
		if utf8.RuneCountInString(param) > 255 {
			return nil, fmt.Errorf("parameter %s value is too long(max 255 unicode characters)", v.Name)
		}
		parameter[v.Name] = param
	}

	return parameter, nil
}

func validateSubdomain(s string) error {
	if s == "" {
		return fmt.Errorf("subdomain is empty")
	}
	if len(s) < 2 {
		return fmt.Errorf("subdomain is too short")
	}
	if len(s) > 63 {
		return fmt.Errorf("subdomain is too long")
	}
	if !DNSNameRegexpWithPattern.MatchString(s) {
		return fmt.Errorf("subdomain %s includes invalid characters", s)
	}
	if _, err := path.Match(s, "x"); err != nil {
		return err
	}
	return nil
}

func (api *WebApi) purge(c echo.Context) (int, error) {
	ps, err := c.FormParams()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	excludes := ps["excludes"]
	excludeTags := ps["exclude_tags"]
	d := c.FormValue("duration")
	di, err := strconv.ParseInt(d, 10, 64)
	mininum := int64(PurgeMinimumDuration.Seconds())
	if err != nil || di < mininum {
		msg := fmt.Sprintf("invalid duration %s (at least %d)", d, mininum)
		if err != nil {
			msg += ": " + err.Error()
		}
		log.Printf("[error] %s", msg)
		return http.StatusBadRequest, errors.New(msg)
	}

	excludesMap := make(map[string]struct{}, len(excludes))
	for _, exclude := range excludes {
		excludesMap[exclude] = struct{}{}
	}
	excludeTagsMap := make(map[string]string, len(excludeTags))
	for _, excludeTag := range excludeTags {
		p := strings.SplitN(excludeTag, ":", 2)
		if len(p) != 2 {
			msg := fmt.Sprintf("invalid exclude_tags format %s", excludeTag)
			if err != nil {
				msg += ": " + err.Error()
			}
			log.Println("[error]", msg)
			return http.StatusBadRequest, errors.New(msg)
		}
		k, v := p[0], p[1]
		excludeTagsMap[k] = v
	}
	duration := time.Duration(di) * time.Second
	begin := time.Now().Add(-duration)

	infos, err := api.mirage.ECS.List(statusRunning)
	if err != nil {
		log.Println("[error] list ecs failed: ", err)
		return http.StatusInternalServerError, err
	}
	tm := make(map[string]struct{}, len(infos))
	for _, info := range infos {
		if _, ok := excludesMap[info.SubDomain]; ok {
			log.Printf("[info] skip exclude subdomain: %s", info.SubDomain)
			continue
		}
		for _, t := range info.tags {
			k, v := aws.StringValue(t.Key), aws.StringValue(t.Value)
			if ev, ok := excludeTagsMap[k]; ok && ev == v {
				log.Printf("[info] skip exclude tag: %s=%s subdomain: %s", k, v, info.SubDomain)
				continue
			}
		}
		if info.Created.After(begin) {
			log.Printf("[info] skip recent created: %s subdomain: %s", info.Created.Format(time.RFC3339), info.SubDomain)
			continue
		}
		tm[info.SubDomain] = struct{}{}
	}
	terminates := lo.Keys(tm)
	if len(terminates) > 0 {
		go api.purgeSubdomains(terminates, duration)
	}

	return http.StatusOK, nil
}

func (api *WebApi) purgeSubdomains(subdomains []string, duration time.Duration) {
	if api.mirage.TryLock() {
		defer api.mirage.Unlock()
	} else {
		log.Println("[info] skip purge subdomains, another purge is running")
		return
	}
	log.Printf("[info] start purge subdomains %d", len(subdomains))
	purged := 0
	for _, subdomain := range subdomains {
		sum, err := api.mirage.GetAccessCount(subdomain, duration)
		if err != nil {
			log.Printf("[warn] access count failed: %s %s", subdomain, err)
			continue
		}
		if sum > 0 {
			log.Printf("[info] skip purge %s %d access", subdomain, sum)
			continue
		}
		if err := api.mirage.ECS.TerminateBySubdomain(subdomain); err != nil {
			log.Printf("[warn] terminate failed %s %s", subdomain, err)
		} else {
			purged++
			log.Printf("[info] purged %s", subdomain)
		}
		time.Sleep(3 * time.Second)
	}
	log.Printf("[info] purge %d subdomains completed", purged)
}
