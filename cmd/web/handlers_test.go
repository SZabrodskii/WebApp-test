package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"webapp/pkg/data"
)

func Test_application_handlers(t *testing.T) {
	var theTests = []struct {
		name                    string
		url                     string
		expectedStatusCode      int
		expectedURL             string
		expectedFirstStatusCode int
	}{
		{name: "home", url: "/", expectedStatusCode: http.StatusOK, expectedURL: "/", expectedFirstStatusCode: http.StatusOK},
		{name: "404", url: "/fish", expectedStatusCode: http.StatusNotFound, expectedURL: "/fish", expectedFirstStatusCode: http.StatusNotFound},
		{name: "profile", url: "/user/profile", expectedStatusCode: http.StatusOK, expectedURL: "/", expectedFirstStatusCode: http.StatusTemporaryRedirect},
	}

	routes := app.routes()

	ts := httptest.NewTLSServer(routes)
	defer ts.Close()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, e := range theTests {
		resp, err := ts.Client().Get(ts.URL + e.url)
		if err != nil {
			t.Log(err)
			t.Fatal(err)
		}

		if resp.StatusCode != e.expectedStatusCode {
			t.Errorf("for %s: expected status %d, but got %d", e.name, e.expectedStatusCode, resp.StatusCode)
		}

		if resp.Request.URL.Path != e.expectedURL {
			t.Errorf("%s: expected final URL of %s but got %s", e.name, e.expectedURL, resp.Request.URL.Path)
		}

		resp2, _ := client.Get(ts.URL + e.url)
		if resp2.StatusCode != e.expectedFirstStatusCode {
			t.Errorf("%s: expected first returned Status Code to be %d, but got %d", e.name, e.expectedFirstStatusCode, resp2.StatusCode)
		}
	}
}

func TestAppHome(t *testing.T) {
	var tests = []struct {
		name         string
		putInSession string
		expectedHTML string
	}{
		{name: "first session", putInSession: "", expectedHTML: "<small>From Session:"},
		{name: "second session", putInSession: "hello, world!", expectedHTML: "<small>From Session:hello, world!"},
	}
	for _, e := range tests {
		req, _ := http.NewRequest("GET", "/", nil)

		req = addContextAndSessionToRequest(req, app)
		_ = app.Session.Destroy(req.Context())

		if e.putInSession != "" {
			app.Session.Put(req.Context(), "test", e.putInSession)
		}

		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(app.Home)

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("TestAppHome returned wrong status code; expected 200, but got %v", rr.Code)
		}
		body, _ := io.ReadAll(rr.Body)
		if !strings.Contains(string(body), e.expectedHTML) {
			t.Errorf("%s : did not find %s in response body", e.name, e.expectedHTML)
		}

	}

}

func TestApp_renderWithBadTemplate(t *testing.T) {
	//set pathToTemplates to a location with a bad template
	pathToTemplates = "./testdata/"
	req, _ := http.NewRequest("GET", "/", nil)
	//add the context and session
	req = addContextAndSessionToRequest(req, app)
	//create a response recorder
	rr := httptest.NewRecorder()
	//set err to app.render
	err := app.render(rr, req, "bad.page.gohtml", &TemplateData{})
	if err == nil {
		t.Errorf("expected error from bad template but did not get one")
	}

	pathToTemplates = "./../../templates/"

}

func getCtx(req *http.Request) context.Context {
	return context.WithValue(req.Context(), contextUserKey, "any context")
}

func addContextAndSessionToRequest(req *http.Request, app application) *http.Request {
	req = req.WithContext(getCtx(req))
	ctx, _ := app.Session.Load(req.Context(), req.Header.Get("X-Session"))

	return req.WithContext(ctx)
}

func Test_app_Login(t *testing.T) {
	var tests = []struct {
		name               string
		postedData         url.Values
		expectedStatusCode int
		expectedLoc        string
	}{
		{
			name: "valid login",
			postedData: url.Values{
				"email":    {"admin@example.com"},
				"password": {"secret"},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLoc:        "/user/profile",
		},
		{
			name: "missing form data",
			postedData: url.Values{
				"email":    {""},
				"password": {""},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLoc:        "/",
		},
		{
			name: "user not found",
			postedData: url.Values{
				"email":    {"kicked@ass.com"},
				"password": {"truth"},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLoc:        "/",
		},
		{
			name: "bad credentials",
			postedData: url.Values{
				"email":    {"admin@example.com"},
				"password": {"imperium"},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLoc:        "/",
		},
		{
			name: "valid login",
			postedData: url.Values{
				"email":    {"admin@example.com"},
				"password": {"secret"},
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLoc:        "/user/profile",
		},
	}

	for _, e := range tests {
		req, _ := http.NewRequest("POST", "/login", strings.NewReader(e.postedData.Encode()))
		req = addContextAndSessionToRequest(req, app)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(app.Login)
		handler.ServeHTTP(rr, req)

		if rr.Code != e.expectedStatusCode {
			t.Errorf("%s returned wrong status code: expected %d, but got %d", e.name, e.expectedStatusCode, rr.Code)
		}

		//test for location
		actualLoc, err := rr.Result().Location()
		if err == nil {
			if actualLoc.String() != e.expectedLoc {
				t.Errorf("%s returned wrong status code: expected %s, but got %s", e.name, e.expectedLoc, actualLoc.String())
			}
		} else {
			t.Errorf("%s: no location header set", e.name)
		}
	}
}

func Test_app_UploadFiles(t *testing.T) {
	// set up some pipes
	pipeRead, pipeWrite := io.Pipe()

	// create a new writer, of type *io,Writer
	writer := multipart.NewWriter(pipeWrite)

	// create a wg and add 1 to it
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// simulate uploading a file, using a goroutine and our writer, after we created it
	go simulatePingUpload("./testdata/img.png", writer, t, wg)
	// read from the pipe, which receives data
	request := httptest.NewRequest("POST", "/", pipeRead)
	request.Header.Add("Content-type", writer.FormDataContentType())

	// call app.UploadFiles
	uploadedFiles, err := app.UploadFiles(request, "./testdata/uploads/")
	if err != nil {
		if strings.Contains(err.Error(), "the file is too big for upload. Max size is 5Mb") {
			fmt.Println("File is too big. Test passed.")
		} else {
			t.Error("Unexpected error:", err)
		}
	}
	// perform the tests
	if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].OriginalFileName)); os.IsNotExist(err) {
		t.Errorf("expected file to exist %s", err.Error())
	}
	// clean up
	_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].OriginalFileName))
}

func simulatePingUpload(fileToUpload string, writer *multipart.Writer, t *testing.T, wg *sync.WaitGroup) {
	defer writer.Close()
	defer wg.Done()

	// create the form data field "file" with value being fileName
	part, err := writer.CreateFormFile("file", path.Base(fileToUpload))
	if err != nil {
		t.Error(err)
	}

	// open the actual file

	f, err := os.Open(fileToUpload)
	if err != nil {
		t.Error(err)
	}
	defer f.Close()

	// decode the image
	img, _, err := image.Decode(f)
	if err != nil {
		t.Error(err)
	}

	// write the png to our io.Writer
	err = png.Encode(part, img)
	if err != nil {
		t.Error(err)
	}

}

func Test_app_UploadProfilePic(t *testing.T) {
	uploadPath = "./testdata/uploads"
	filePath := "./testdata/img.png"

	// specify a field name for the form
	fieldName := "file"

	// create a bytes.Bufferto act as the request body
	body := new(bytes.Buffer)

	// create a new writer
	multiWriter := multipart.NewWriter(body)

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}

	// create a form file
	w, err := multiWriter.CreateFormFile(fieldName, filePath)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(w, file); err != nil {
		t.Fatal(err)
	}

	multiWriter.Close()

	request := httptest.NewRequest(http.MethodPost, "/upload", body)
	request = addContextAndSessionToRequest(request, app)
	app.Session.Put(request.Context(), "user", data.User{ID: 1})
	request.Header.Add("Content-type", multiWriter.FormDataContentType())

	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(app.UploadProfilePic)
	handler.ServeHTTP(rr, request)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("wrong status code")
	}

}
