// main_test.go
package main


/*

CREATE TABLE users
(
  ID 			int(11) NOT NULL AUTO_INCREMENT,
  LastName 		varchar(255) NOT NULL,
  FirstName 	varchar(255) DEFAULT NULL,
  Email 		varchar(255) DEFAULT NULL,
  PRIMARY KEY (`ID`)
);

create table skills (
	ID int NOT NULL AUTO_INCREMENT,
	name varchar(255),
    count int,
    CONSTRAINT PK_skill PRIMARY KEY (ID)
);

CREATE TABLE user_skills (
    UserID int NOT NULL,
    SkillID int NOT NULL,
    Skillname varchar(255),
    CONSTRAINT PK_userskill PRIMARY KEY (UserID,SkillID),
    FOREIGN KEY (UserID) REFERENCES Users(ID),
    FOREIGN KEY (SkillID) REFERENCES Skills(ID)
);


CREATE TABLE user_endorsements (
    UserID int NOT NULL,
    SkillID int NOT NULL,
    EndorsedBy int NOT NULL,
    CONSTRAINT PK_userendorsement PRIMARY KEY (UserID,SkillID,EndorsedBy),
    FOREIGN KEY (UserID) REFERENCES Users(ID),
    FOREIGN KEY (SkillID) REFERENCES Skills(ID),
    FOREIGN KEY (EndorsedBy) REFERENCES Users(ID)
);

create table friendships (
	User1 int NOT NULL,
    User2 int NOT NULL,
    CONSTRAINT PK_friendship PRIMARY KEY (User1,User2),
    FOREIGN KEY (User1) REFERENCES Users(ID),
    FOREIGN KEY (User2) REFERENCES Users(ID)
);

 */


import (
	"os"
	"log"
	"testing"
	"net/http"
	"net/http/httptest"
	"encoding/json"
	"bytes"
	"fmt"
	"strconv"
)

var a App

func TestMain(m *testing.M) {
	a = App{}
	a.Initialize("root", "sql123", "social_net")
	ensureTableExists()
	log.Println("Before Run")
	code := m.Run()
	log.Println("After Run")
	clearTable()
	os.Exit(code)
}

func ensureTableExists() {
	if _, err := a.DB.Exec(userTableCreationQuery); err != nil {
		log.Fatal(err)
	}

	if _, err := a.DB.Exec(skillTableCreationQuery); err != nil {
		log.Fatal(err)
	}

	if _, err := a.DB.Exec(userEndorsementsTableCreationQuery); err != nil {
		log.Fatal(err)
	}

	if _, err := a.DB.Exec(userSkillsTableCreationQuery); err != nil {
		log.Fatal(err)
	}

	if _, err := a.DB.Exec(friendshipsTableCreationQuery); err != nil {
		log.Fatal(err)
	}
}

func clearTable() {
	a.DB.Exec("DELETE FROM user_skills")
	a.DB.Exec("DELETE FROM user_endorsements")
	a.DB.Exec("DELETE FROM friendships")
	a.DB.Exec("DELETE FROM users")
	a.DB.Exec("ALTER TABLE users AUTO_INCREMENT = 1")
	a.DB.Exec("DELETE FROM skills")
	a.DB.Exec("ALTER TABLE skills AUTO_INCREMENT = 1")
}



func TestEmptyTable(t *testing.T) {
	clearTable()
	req, _ := http.NewRequest("GET", "/users", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
	if body := response.Body.String(); body != "[]" {
		t.Errorf("Expected an empty array. Got %s", body)
	}
}


func TestGetUser(t *testing.T) {
	clearTable()
	addUsers(2)
	req, _ := http.NewRequest("GET", "/user/1", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
}


func TestCreateUser(t *testing.T) {
	clearTable()
	payload := []byte(`{"firstname":"Jay","lastname":"Elu", "email":"jelumala@cs.nmsu.edu"}`)
	req, _ := http.NewRequest("POST", "/user", bytes.NewBuffer(payload))
	response := executeRequest(req)
	checkResponseCode(t, http.StatusCreated, response.Code)
	var m map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &m)
	if m["firstname"] != "Jay" {
		t.Errorf("Expected firstname to be 'Jay'. Got '%v'", m["firstname"])
	}
	if m["lastname"] != "Elu" {
		t.Errorf("Expected user age to be 'Elu'. Got '%v'", m["lastname"])
	}
	// the id is compared to 1.0 because JSON unmarshaling converts numbers to
	// floats, when the target is a map[string]interface{}
	if m["id"] != 1.0 {
		t.Errorf("Expected user ID to be '1'. Got '%v'", m["id"])
	}
}


func TestRequestFriend(t *testing.T) {
	clearTable()
	addUsers(2)
	req, _ := http.NewRequest("GET", "/user/1", nil)
	response := executeRequest(req)
	var originalUser map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &originalUser)

	// 2 sends a friend request to 1
	payload := []byte(`{"id":2}`)
	req, _ = http.NewRequest("PUT", "/user/addfriend/1", bytes.NewBuffer(payload))
	response = executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)

	var m map[string][]interface{}
	json.Unmarshal(response.Body.Bytes(), &m)

	if (m["pending_friend_requests"][len(m["pending_friend_requests"])-1] != 2) {
		t.Errorf("Expected pending_friend_requests to have 2")
	}
}





func TestApproveFriend(t *testing.T) {


}


func TestEndorseSkill(t *testing.T) {


}


func TestGetNonExistentUser(t *testing.T) {
	clearTable()
	req, _ := http.NewRequest("GET", "/user/45", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)
	log.Println("Response code is %s", response.Code)
	var m map[string]string
	json.Unmarshal(response.Body.Bytes(), &m)

	if m["error"] != "User not found" {
		t.Errorf("Expected the 'error' key of the response to be set to 'User not found'. Got '%s'", m["error"])
	}
}


func addUsers(count int) {
	if count < 1 {
		count = 1
	}
	for i := 0; i < count; i++ {
		statement := fmt.Sprintf("INSERT INTO users(firstname,lastname,email) VALUES('%s', %s, %s)", ("Firstname" + strconv.Itoa(i+1)), "Lastname" + strconv.Itoa(i+1), "Email" + strconv.Itoa(i+1) + "@Email.com")
		a.DB.Exec(statement)
	}
}


func executeRequest(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	a.Router.ServeHTTP(rr, req)

	return rr
}

func checkResponseCode(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Errorf("Expected response code %d. Got %d\n", expected, actual)
	}
}



const userTableCreationQuery = `
CREATE TABLE IF NOT EXISTS users
(
  ID 			int(11) NOT NULL AUTO_INCREMENT,
  LastName 		varchar(255) NOT NULL,
  FirstName 	varchar(255) DEFAULT NULL,
  Email 		varchar(255) DEFAULT NULL,
  PRIMARY KEY (ID)
)`

const skillTableCreationQuery = `
CREATE TABLE IF NOT EXISTS skills
(
	ID int NOT NULL AUTO_INCREMENT,
	name varchar(255),
    count int,
    CONSTRAINT PK_skill PRIMARY KEY (ID)
)`

const userEndorsementsTableCreationQuery = `
CREATE TABLE IF NOT EXISTS user_endorsements
(
    UserID int NOT NULL,
    SkillID int NOT NULL,
    EndorsedBy int NOT NULL,
    CONSTRAINT PK_userendorsement PRIMARY KEY (UserID,SkillID,EndorsedBy),
    FOREIGN KEY (UserID) REFERENCES Users(ID),
    FOREIGN KEY (SkillID) REFERENCES Skills(ID),
    FOREIGN KEY (EndorsedBy) REFERENCES Users(ID)
)`


const userSkillsTableCreationQuery = `
CREATE TABLE IF NOT EXISTS user_skills
(
    UserID int NOT NULL,
    SkillID int NOT NULL,
    Skillname varchar(255),
    CONSTRAINT PK_userskill PRIMARY KEY (UserID,SkillID),
    FOREIGN KEY (UserID) REFERENCES Users(ID),
    FOREIGN KEY (SkillID) REFERENCES Skills(ID)
)`

const friendshipsTableCreationQuery = `
CREATE TABLE IF NOT EXISTS friendships
(
	User1 int NOT NULL,
    User2 int NOT NULL,
    CONSTRAINT PK_friendship PRIMARY KEY (User1,User2),
    FOREIGN KEY (User1) REFERENCES Users(ID),
    FOREIGN KEY (User2) REFERENCES Users(ID)
)`
