package dbrepo

import (
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"log"
	"os"
	"testing"
	"time"
	"webapp/pkg/data"
	"webapp/pkg/repository"
)

var host = "localhost"
var user = "postgres"
var password = "postgres"
var dbName = "users"
var port = "5432"
var dsn = "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable timezone=UTC connect_timeout=5"
var resource *dockertest.Resource
var pool *dockertest.Pool
var testDB *sql.DB
var testRepo repository.DatabaseRepo

func TestMain(m *testing.M) {
	var err error
	//connect to docker; fail if docker not running
	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("cannot connect the docker - is it running? %s", err)
	}
	//set up docker options, specifying the image and so forth
	opt := dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "14.5",
		Env: []string{
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + dbName,
		},
		ExposedPorts: []string{port},
		PortBindings: map[docker.Port][]docker.PortBinding{
			docker.Port(port): {{HostIP: "0.0.0.0", HostPort: "5432"}},
		},
	}

	//get the resource (docker image)
	resource, err := pool.RunWithOptions(&opt)
	if err != nil {
		_ = pool.Purge(resource)
		log.Fatalf("could not start resource: %s", err)
	}
	defer func(resource *dockertest.Resource) {
		_ = pool.Purge(resource)
	}(resource)

	//start the image and wait until it's ready
	if err := pool.Retry(func() error {
		var err error
		testDB, err = sql.Open("pgx", fmt.Sprintf(dsn, host, port, user, password, dbName))
		if err != nil {
			log.Println("Error", err)
			return err
		}
		return testDB.Ping()
	}); err != nil {
		log.Fatalf("could not connect the database: %s", err)
	}
	//populate the DB with empty table
	err = createTables()
	if err != nil {
		log.Fatalf("could not read the data from file: %s", err)
	}

	testRepo = &PostgresDBRepo{DB: testDB}

	//run the tests
	code := m.Run()
	//clean up - stop everything and purge the resource
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("could not purge resource: %s", err)
	}

	os.Exit(code)
}

func createTables() error {
	tableSQL, err := os.ReadFile("./testData/users.sql")
	if err != nil {
		fmt.Println(err)
		return err
	}
	_, err = testDB.Exec(string(tableSQL))
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func Test_pingDB(t *testing.T) {
	err := testDB.Ping()
	if err != nil {
		t.Error("cant ping db")
	}
}

func TestPostgresDBRepoInsertUser(t *testing.T) {
	testUser := data.User{
		FirstName: "Admin",
		LastName:  "User",
		Email:     "admin@example.com",
		Password:  "secret",
		IsAdmin:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	//insert a user to a DB
	id, err := testRepo.InsertUser(testUser)
	if err != nil {
		t.Errorf("inser user returned an error: %v", err)
	}

	if id != 1 {
		t.Errorf("insert user returned wrong: expected 1, but got %d", id)
	}
}

func TestPostgresDBRepoAllUsers(t *testing.T) {
	users, err := testRepo.AllUsers()
	if err != nil {
		t.Errorf("allUsers reports an error: %s", err)
	}
	if len(users) != 1 {
		t.Errorf("allUsers reports wrong size - expected 1, but got %d", len(users))
	}

	testUser := data.User{
		FirstName: "Jack",
		LastName:  "Smith",
		Email:     "Jack@smith.com",
		Password:  "secret",
		IsAdmin:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, _ = testRepo.InsertUser(testUser)

	users, err = testRepo.AllUsers()
	if err != nil {
		t.Errorf("allUsers reports an error: %s", err)
	}

	if len(users) != 2 {
		t.Errorf("allUsers reports wrong size after insert - expected 2, but got %d", len(users))
	}
}

func TestPostgresDBRepo_GetUser(t *testing.T) {
	user, err := testRepo.GetUser(1)
	if err != nil {
		t.Errorf("getUser reports wrong - expected id(1), but got %d", user.ID)
	}
	//check if we get a right user

	if user.Email != "admin@example.com" {
		t.Errorf("getUser returned a wrong user email: expected admin@example.com but got %v", user.Email)
	}

	user, err = testRepo.GetUser(3)
	if err == nil {
		t.Errorf("no error reported when get non-existing user by id: %v", user.ID)
	}
}

func TestPostgresDBRepo_GetUserByEmail(t *testing.T) {
	user, err := testRepo.GetUserByEmail("Jack@smith.com")
	if err != nil {
		t.Errorf("getUserByEmail reports wrong - expected Jaxk@smih.com, but got %s", user.Email)
	}
	//check if we get a right user

	if user.ID != 2 {
		t.Errorf("getUser returned a wrong ID: expected 2, but got %d", user.ID)
	}
}

func TestPostgresDBRepo_UpdateUser(t *testing.T) {
	//creating a fake user for updating
	updateUser := data.User{
		ID:        666,
		Email:     "upadate@example.com",
		FirstName: "John",
		LastName:  "Doe",
		IsAdmin:   0,
	}
	//calling UpdateUser func
	err := testRepo.UpdateUser(updateUser)
	if err != nil {
		t.Errorf("update user func got error: %v", err)
	}
	//checking that user was updated successfully by getting him from DB once more
	newUser, err := testRepo.GetUser(666)
	if err != nil {
		t.Errorf("getUser returned an error after updating: %v", err)
	}
	//checking that values updated properly
	if newUser.Email != updateUser.Email {
		t.Errorf("Updataed email doesn't match: expected upadate@example.com, but got %v", newUser.Email)

	}
	if newUser.FirstName != updateUser.FirstName {
		t.Errorf("Updataed First Name doesn't match: expected John, but got %v", newUser.FirstName)

	}
	if newUser.LastName != updateUser.LastName {
		t.Errorf("Updataed Last Name doesn't match: expected Doe, but got %v", newUser.LastName)

	}
	if newUser.IsAdmin != updateUser.IsAdmin {
		t.Errorf("Updataed admin status doesn't match: expected 0, but got %v", newUser.IsAdmin)

	}

}

//func TestPostgresDBRepo_UpdateUser2(t *testing.T) {
//	user, _ := testRepo.GetUser(2)
//	user.FirstName = "Alec"
//	user.LastName = "Boldwin"
//	user.Email = "Alec@smith.com"

//	err := testRepo.UpdateUser(*user)
//	if err != nil {
//		t.Errorf("UpdateUser got an error with updating user %d: %v", 2, err)
//	}
//user, _ = testRepo.GetUser(2)
//if user.FirstName != "Alec" || user.LastName != "Boldwin" {
//	t.Errorf("expected user %d to have updated first name of Alec, but got %v", 2, user.FirstName)
//}

//}

func TestPostgresDBRepo_DeleteUser2(t *testing.T) {
	testUser := data.User{
		FirstName: "ToDelete",
		LastName:  "User",
		Email:     "todelete@example.com",
		Password:  "secret",
		IsAdmin:   0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	insertedID, err := testRepo.InsertUser(testUser)
	if err != nil {
		log.Fatalf("Insertation failed: %v", err)
	}

	err = testRepo.DeleteUser(insertedID)
	if err != nil {
		t.Errorf("could not delete ")
	}

	deletedUser, err := testRepo.GetUser(insertedID)
	if deletedUser != nil {
		t.Errorf("expected %v to be deleted, but got him in the base", deletedUser)
	}
}

func TestPostgresDBRepo_DeleteUser(t *testing.T) {
	err := testRepo.DeleteUser(2)
	if err != nil {
		t.Error("error got while deleting user with ID 2", err)
	}
	_, err = testRepo.GetUser(2)
	if err == nil {
		t.Errorf("expected to have no user by getting user with ID 2")
	}
}

func TestPostgresDBRepo_ResetPassword(t *testing.T) {
	err := testRepo.ResetPassword(1, "another secret")
	if err != nil {
		t.Error("error reseting users password", err)
	}
	user, _ := testRepo.GetUser(1)
	matches, err := user.PasswordMatches("another password")
	if err != nil {
		t.Error(err)
	}
	if !matches {
		t.Errorf("password was not changed")
	}

}

func TestPostgresDBRepo_InsertUserImage(t *testing.T) {
	image := data.UserImage{}
	image.UserID = 1
	image.FileName = "test.jpg"
	image.CreatedAt = time.Now()
	image.UpdatedAt = time.Now()
	newID, err := testRepo.InsertUserImage(image)
	if err != nil {
		t.Error("inserting user image failed", err)
	}
	if newID != 1 {
		t.Errorf("got wrong ID for image - should be 1, but got %v", newID)
	}

	image.UserID = 100
	_, err = testRepo.InsertUserImage(image)
	if err == nil {
		t.Error("inserted a user image with non-existing user id")
	}
}
