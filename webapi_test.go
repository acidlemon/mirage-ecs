package mirageecs_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	mirageecs "github.com/acidlemon/mirage-ecs"
	"github.com/labstack/echo/v4"
)

func TestLoadParameter(t *testing.T) {
	testFile := "config_sample.yml"
	cfg, _ := mirageecs.NewConfig(&mirageecs.ConfigParams{Path: testFile})
	app := mirageecs.NewWebApi(cfg, &mirageecs.LocalTaskRunner{})

	params := url.Values{}
	params.Set("nick", "mirageman")
	params.Set("branch", "develop")
	params.Set("test", "dummy")

	req, err := http.NewRequest("POST", fmt.Sprintf("localhost?%s", params.Encode()), nil)
	if err != nil {
		t.Error(err)
	}

	e := echo.New()
	c := e.NewContext(req, nil)
	parameter, err := app.LoadParameter(c.FormValue)

	if err != nil {
		t.Error(err)
	}

	if len(parameter) != 1 {
		t.Error(errors.New("could not parse parameter"))
	}

	if parameter["branch"] != "develop" {
		t.Error(errors.New("could not parse parameter"))
	}

	if parameter["test"] != "" {
		t.Error(errors.New("could not parse parameter"))
	}

	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	data := `---
parameters:
  - name: branch
    env: GIT_BRANCH
    rule: "[0-9a-z]{5,32}"
    required: true
  - name: nick
    env: NICK
    rule: "[0-9A-Za-z]{1,10}"
    required: false
  - name: test
    env: TEST
    rule:
    required: false
`
	if err := ioutil.WriteFile(f.Name(), []byte(data), 0644); err != nil {
		t.Error(err)
	}

	cfg, err = mirageecs.NewConfig(&mirageecs.ConfigParams{Path: f.Name()})
	if err != nil {
		t.Error(err)
	}
	app = mirageecs.NewWebApi(cfg, &mirageecs.LocalTaskRunner{})

	c = e.NewContext(req, nil)
	parameter, err = app.LoadParameter(c.FormValue)
	if err != nil {
		t.Error(err)
	}

	if len(parameter) != 3 {
		t.Error(errors.New("could not parse parameter"))
	}

	if parameter["test"] != "dummy" {
		t.Error(errors.New("could not parse parameter"))
	}

	params = url.Values{}
	params.Set("nick", "mirageman")
	params.Set("branch", "aaa")
	params.Set("test", "dummy")

	req, err = http.NewRequest("POST", fmt.Sprintf("localhost?%s", params.Encode()), nil)
	if err != nil {
		t.Error(err)
	}

	c = e.NewContext(req, nil)
	_, err = app.LoadParameter(c.FormValue)

	if err == nil {
		t.Error("Not apply parameter rule")
	}

	params = url.Values{}
	params.Set("nick", "mirageman")
	params.Set("test", "dummy")

	req, err = http.NewRequest("POST", fmt.Sprintf("localhost?%s", params.Encode()), nil)
	if err != nil {
		t.Error(err)
	}

	c = e.NewContext(req, nil)
	_, err = app.LoadParameter(c.FormValue)

	if err == nil {
		t.Error("Not apply parameter rule")
	}

}

var validSubdomains = []string{
	"ab",
	"abc",
	"a-z",
	"AB-CD",
	"a-z-0-9",
	"a123456789",
	"www*",
	"foo[0-9]",
	"api-?-test",
	"*-xxx",
	strings.Repeat("a", 63),
}

var invalidSubdomains = []string{
	"0abc",
	"a-",
	"-a",
	"a.b",
	"a+b",
	"a_b",
	"a^b",
	"a$b",
	"a%b",
	"www/xxx",
	"foo[0-9",
	strings.Repeat("a", 64),
}

func TestValidateSubdomain(t *testing.T) {
	for _, s := range validSubdomains {
		if err := mirageecs.ValidateSubdomain(s); err != nil {
			t.Errorf("%s should be valid", s)
		}
	}

	for _, s := range invalidSubdomains {
		if err := mirageecs.ValidateSubdomain(s); err == nil {
			t.Errorf("%s should be invalid", s)
		}
	}
}
