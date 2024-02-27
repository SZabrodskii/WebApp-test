package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"webapp/pkg/data"
)

func Test_application_addIPToContext(t *testing.T) {
	tests := []struct {
		headerName  string
		headerValue string
		addr        string
		emptyAddr   bool
	}{
		{headerName: "", headerValue: "", addr: "", emptyAddr: false},
		{headerName: "", headerValue: "", addr: "", emptyAddr: true},
		{headerName: "X-Forwarded_For", headerValue: "192:3:2:1", addr: "", emptyAddr: false},
		{headerName: "", headerValue: "", addr: "hello:world", emptyAddr: false},
	}
	newHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := r.Context().Value(contextUserKey)
		if val == nil {
			t.Errorf("%v not present", contextUserKey)
		}
		ip, ok := val.(string)
		if !ok {
			t.Errorf("%v not string", ip)
		}
		t.Log(ip)
	})

	for _, e := range tests {
		handlerToTest := app.addIPToContext(newHandler)

		req := httptest.NewRequest("GET", "http://testing", nil)
		if e.emptyAddr {
			req.RemoteAddr = ""
		}
		if len(e.headerName) > 0 {
			req.Header.Add(e.headerName, e.headerValue)
		}
		if len(e.addr) > 0 {
			req.RemoteAddr = e.addr
		}

		handlerToTest.ServeHTTP(httptest.NewRecorder(), req)
	}

}

func Test_application_ipFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextUserKey, "shit happens")

	result := app.ipFromContext(ctx)
	expected := "shit happens"

	if result != expected {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func Test_app_ipFromContext(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, contextUserKey, "some data")
	result := app.ipFromContext(ctx)
	if !strings.EqualFold("some data", result) {
		t.Errorf("Expected %v but got %v", "some data", result)

	}
}

func Test_app_auth(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})

	var tests = []struct {
		name   string
		isAuth bool
	}{
		{name: "logged in", isAuth: true},
		{name: "not logged in", isAuth: false},
	}

	for _, e := range tests {
		handlerToTest := app.auth(nextHandler)
		req := httptest.NewRequest("GET", "http://testing", nil)
		req = addContextAndSessionToRequest(req, app)
		if e.isAuth {
			app.Session.Put(req.Context(), "user", data.User{ID: 1})
		}

		rr := httptest.NewRecorder()
		handlerToTest.ServeHTTP(rr, req)

		if e.isAuth && rr.Code != http.StatusOK {
			t.Errorf("%s: expected status code of 200 but got %d", e.name, rr.Code)
		}
		if !e.isAuth && rr.Code != http.StatusTemporaryRedirect {
			t.Errorf("%s: expected status code 307 but got %d", e.name, rr.Code)
		}
	}
}
