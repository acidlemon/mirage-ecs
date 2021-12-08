package mirageecs

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	data := `---
host:
  webapi: localhost
  reverse_proxy_suffix: .dev.example.net
listen:
  foreign_address: 127.0.0.1
  http:
    - listen: 8080
      target: 5000
docker:
  endpoint: unix:///var/run/docker.sock
storage:
  datadir: ./data
  htmldir: ./html
parameters:
  - name: branch
    env: GIT_BRANCH
    rule: "[0-9a-z-]{32}"
    required: true
  - name: nick
    env: NICK
    rule: "[0-9A-Za-z]{10}"
    required: false
`

	if err := ioutil.WriteFile(f.Name(), []byte(data), 0644); err != nil {
		t.Error(err)
	}

	cfg := NewConfig(f.Name())

	if cfg.Parameter[0].Name != "branch" {
		t.Error("could not parse parameter")
	}

	if cfg.Parameter[1].Env != "NICK" {
		t.Error("could not parse parameter")
	}

	if cfg.Parameter[0].Required != true {
		t.Error("could not parse parameter")
	}
}
