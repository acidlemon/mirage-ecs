package mirageecs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"

	"gopkg.in/acidlemon/rocket.v2"
)

func TestLoadParameter(t *testing.T) {
	testFile := "config_sample.yml"
	cfg := NewConfig(testFile)
	app := NewWebApi(cfg)

	params := url.Values{}
	params.Set("nick", "mirageman")
	params.Set("branch", "develop")
	params.Set("test", "dummy")

	req, err := http.NewRequest("POST", fmt.Sprintf("localhost?%s", params.Encode()), nil)
	if err != nil {
		t.Error(err)
	}

	args := rocket.Args{}
	c := rocket.NewContext(req, args, nil)

	parameter, err := app.loadParameter(c)

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

	cfg = NewConfig(f.Name())
	app = NewWebApi(cfg)

	c = rocket.NewContext(req, args, nil)
	parameter, err = app.loadParameter(c)

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

	c = rocket.NewContext(req, args, nil)
	_, err = app.loadParameter(c)

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

	c = rocket.NewContext(req, args, nil)
	_, err = app.loadParameter(c)

	if err == nil {
		t.Error("Not apply parameter rule")
	}

}
