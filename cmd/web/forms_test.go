package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestForm_Has(t *testing.T) {
	form := NewForm(nil)
	has := form.Has("whatever")
	if has {
		t.Error("the form contains the data it should not have")
	}

	postedData := url.Values{}
	postedData.Add("a", "b")
	form = NewForm(postedData)

	has = form.Has("a")
	if !has {
		t.Error("the form does not contain the data in should have")
	}
}

func TestForm_Required(t *testing.T) {
	r := httptest.NewRequest("POST", "/whatever", nil)
	form := NewForm(r.PostForm)

	form.Required("a", "b", "c")

	if form.Valid() {
		t.Error("form shows valid while required fields are missing")
	}

	postedData := url.Values{}
	postedData.Add("a", "a")
	postedData.Add("b", "b")
	postedData.Add("c", "c")

	r, _ = http.NewRequest("POST", "/whatever", nil)
	r.PostForm = postedData

	form = NewForm(r.PostForm)
	form.Required("a", "b", "c")
	if !form.Valid() {
		t.Error("form expected to be valid but its not")
	}
}

func TestForm_Check(t *testing.T) {
	form := NewForm(nil)
	form.Check(false, "password", "password is required")
	if form.Valid() {
		t.Error("Valid() returns true and it should be true when calling Check()")
	}
}

func TestForm_ErrorGet(t *testing.T) {
	form := NewForm(nil)
	form.Check(false, "password", "password is required")
	s := form.Errors.Get("password")
	if len(s) == 0 {
		t.Error("should have returned error from Get() but got nothing")
	}
	s = form.Errors.Get("whatever")
	if len(s) != 0 {
		t.Error("should not have an error by Get() but got one")
	}

}

func TestForm_Valid(t *testing.T) {
	formData := url.Values{}
	form := NewForm(formData)
	if !form.Valid() {
		t.Error("expected form to be valid")
	}

	form.Errors.Add("username", "username is required")
	if form.Valid() {
		t.Error("expected form to be invalid")
	}
}
