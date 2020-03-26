package liveness

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_BasicCheck(t *testing.T) {
	mc := NewCheckCollector(time.Millisecond * 20)
	if !mc.IsLive(0) {
		t.Errorf("Multicheck with no checks should always pass")
	}

	c1 := mc.NewCheck()
	if !mc.IsLive(0) {
		t.Errorf("Just made check should pass")
	}

	time.Sleep(time.Millisecond * 30)
	if mc.IsLive(0) {
		t.Errorf("Multi check should have failed")
	}

	c1.CheckIn()
	if !mc.IsLive(0) {
		t.Errorf("Checker should passed after checkin")
	}

	c2 := mc.NewCheck()
	if !mc.IsLive(0) {
		t.Errorf("Just made check 2 should pass")
	}

	time.Sleep(time.Millisecond * 30)
	c1.CheckIn()
	// don't checkIn c2

	if mc.IsLive(0) {
		t.Errorf("Multi check should have failed by c2")
	}

	c2.CheckIn()
	if !mc.IsLive(0) {
		t.Errorf("Check 2 should pass after checkin")
	}
}

func Test_CheckHTTP(t *testing.T) {
	c := NewCheckCollector(time.Millisecond * 20)
	r := httptest.NewRequest(http.MethodGet, "/live", nil)
	wr := httptest.NewRecorder()

	c1 := c.NewCheck()
	_ = c.NewCheck() // never check-in c2

	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusOK {
		t.Errorf("Check should have passed")
	}

	time.Sleep(time.Millisecond * 30)
	c1.CheckIn()

	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusServiceUnavailable {
		t.Errorf("Check should not have passed")
	}
}

func Test_CheckHTTPOverride(t *testing.T) {
	c := NewCheckCollector(time.Millisecond * 20)
	r := httptest.NewRequest(http.MethodGet, "/live", nil)
	r.Header.Add(ToleranceHeader, "30s")
	wr := httptest.NewRecorder()

	c1 := c.NewCheck()

	c1.CheckIn()

	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusOK {
		t.Errorf("Check should have passed")
	}

	time.Sleep(time.Millisecond * 60)

	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusOK {
		t.Errorf("Check should still have passed")
	}

	r.Header.Del(ToleranceHeader)
	wr = httptest.NewRecorder()
	c.ServeHTTP(wr, r)
	if wr.Code != http.StatusServiceUnavailable {
		t.Errorf("Check should not have passed")
	}
}
