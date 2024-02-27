package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"webapp/pkg/data"
	"webapp/templates"
)

var pathToTemplates = "../../templates/"
var uploadPath = "./static/img"

func (app *application) Home(w http.ResponseWriter, r *http.Request) {
	var td = make(map[string]any)
	if app.Session.Exists(r.Context(), "test") {
		msg := app.Session.GetString(r.Context(), "test")
		td["test"] = msg
	} else {
		app.Session.Put(r.Context(), "test", "Hit this page at "+time.Now().UTC().String())
	}
	_ = app.render(w, r, "home.page.gohtml", &TemplateData{Data: td})
}
func (app *application) Profile(w http.ResponseWriter, r *http.Request) {
	_ = app.render(w, r, "profile.page.gohtml", &TemplateData{})
}

type TemplateData struct {
	IP    string
	Data  map[string]any
	Error string
	Flash string
	User  data.User
}

func (app *application) render(w http.ResponseWriter, r *http.Request, t string, td *TemplateData) error {

	parsedTemplate, err := template.ParseFS(templates.Html, "html/"+t, "html/base.layout.gohtml")
	if err != nil {
		log.Println(err)
		http.Error(w, "rendering bad request", http.StatusBadRequest)
		return err
	}

	td.IP = app.ipFromContext(r.Context())

	td.Error = app.Session.PopString(r.Context(), "error")
	td.Flash = app.Session.PopString(r.Context(), "flash")

	if app.Session.Exists(r.Context(), "user") {
		td.User = app.Session.Get(r.Context(), "user").(data.User)
	}

	err = parsedTemplate.Execute(w, td)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return err
	}
	return nil
}

func (app *application) Login(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	//validate data

	form := NewForm(r.PostForm)
	form.Required("email", "password")

	if !form.Valid() {
		//redirect to a login page with an error message
		app.Session.Put(r.Context(), "error", "invalid login credentials")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return

	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	user, err := app.DB.GetUserByEmail(email)
	if err != nil {
		app.Session.Put(r.Context(), "error", "invalid login")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if !app.authenticate(r, user, password) {
		app.Session.Put(r.Context(), "error", "invalid login")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	//authenticate the user
	//if not authenticated - then redirect to the home page with an error

	//renew the user's token
	_ = app.Session.RenewToken(r.Context())

	//redirect to some other page
	app.Session.Put(r.Context(), "flash message", "successfully logged in")
	http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
}

func (app *application) authenticate(r *http.Request, user *data.User, password string) bool {
	if valid, err := user.PasswordMatches(password); err != nil || !valid {
		return false
	}
	app.Session.Put(r.Context(), "user", user)
	return true
}

func (app *application) UploadProfilePic(w http.ResponseWriter, r *http.Request) {
	// call a function that extracts a file from an upload (request)
	files, err := app.UploadFiles(r, uploadPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// get the user from the session
	user := app.Session.Get(r.Context(), "user").(data.User)

	// create a var of type data.UserImage
	userImage := data.UserImage{
		UserID:   user.ID,
		FileName: files[0].OriginalFileName,
	}
	// insert UserImage into user_images
	_, err = app.DB.InsertUserImage(userImage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// refresh the session variable "user"
	updatedUser, err := app.DB.GetUser(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	app.Session.Put(r.Context(), "user", updatedUser)
	//redirect back to profile page
	http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
}

type UploadedFile struct {
	OriginalFileName string
	FileSize         int64
}

func (app *application) UploadFiles(r *http.Request, uploadDirectory string) ([]*UploadedFile, error) {
	var uploadedFiles []*UploadedFile
	// parse the form, so we could have access to the file
	err := r.ParseMultipartForm(int64(1024 * 1024 * 5))
	if err != nil {
		return nil, fmt.Errorf("%v: the file is to big for upload. Max size is 5Mb", err)
	}

	for _, fileHeaders := range r.MultipartForm.File {
		for _, header := range fileHeaders {
			uploadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				var uploadedFile UploadedFile
				infile, err := header.Open()
				if err != nil {
					return nil, err
				}
				defer infile.Close()

				uploadedFile.OriginalFileName = header.Filename

				var outfile *os.File
				defer outfile.Close()

				if outfile, err = os.Create(filepath.Join(uploadDirectory, uploadedFile.OriginalFileName)); err != nil {
					return nil, err
				} else {
					fileSize, err := io.Copy(outfile, infile)
					if err != nil {
						return nil, err
					}
					uploadedFile.FileSize = fileSize
				}
				uploadedFiles = append(uploadedFiles, &uploadedFile)
				return uploadedFiles, nil
			}(uploadedFiles)
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}
