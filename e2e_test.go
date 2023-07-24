package mirageecs_test

import (
	"net/http/httptest"
	"testing"
)

func TestE2EAPI(t *testing.T) {
	ts := httptest.NewServer(nil)
	defer ts.Close()
}
