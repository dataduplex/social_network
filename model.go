package main

import (
	"database/sql"
	"fmt"
	"strings"
	"strconv"
	"log"
	"reflect"
)

type user struct {

	Id int64							`json:"id"`
	FirstName string					`json:"firstname"`
	LastName string						`json:"lastname"`
	Email string						`json:"email"`
	Sex	string							`json:"sex"`
	UserSkills []userskill				`json:"userskills"`
	Friends []int64						`json:"friends"`
	PendingFriendApprovals []int64 		`json:"pending_friend_approvals"`
	PendingFriendRequests  []int64 		`json:"pending_friend_requests"`
	Password				string		`json:"password"`

}

type viewuser struct {
	LoggedInUserId 	int64		`json:"id"`
	ViewUserInfo 	user		`json:"userview"`
	isFriend		bool		`json:"isfriend"`
}

type userskill struct {
	SkillId int64					`json:"skillid"`
	UserId int64					`json:"userid"`
	SkillName string				`json:"skillname"`
	EndorsementCount int64			`json:"endorsementcount"`
	EndorsedBy []int64				`json:"endorsedby"`
}


type skill struct {
	Id int64					`json:"id"`
	Name string					`json:"name"`
	UserCount int				`json:"usercount"`
	RelatedUsers []interface{}	`json:"relatedusers"`
}

type friendship struct {
	User1 int64
	User2 int64
}

type JwtToken struct {
	Token string `json:"token"`
	User user `json:"user"`
}

// NullInt64 is an alias for sql.NullInt64 data type
type NullInt64 sql.NullInt64
type NullString sql.NullString

func (ni *NullInt64) Scan(value interface{}) error {
	var i sql.NullInt64
	if err := i.Scan(value); err != nil {
		return err
	}

	// if nil then make Valid false
	if reflect.TypeOf(value) == nil {
		*ni = NullInt64{i.Int64, false}
	} else {
		*ni = NullInt64{i.Int64, true}
	}
	return nil
}

func (ns *NullString) Scan(value interface{}) error {
	var s sql.NullString
	if err := s.Scan(value); err != nil {
		return err
	}

	// if nil then make Valid false
	if reflect.TypeOf(value) == nil {
		*ns = NullString{s.String, false}
	} else {
		*ns = NullString{s.String, true}
	}

	return nil
}

func (u *user) addUser(db *sql.DB) error {

	statement := fmt.Sprintf("insert into users(firstname, lastname, email, sex, status, password) " +
									"values('%s','%s','%s','%s','%s','%s')",
									u.FirstName, u.LastName, u.Email, u.Sex, "ACTIVE", u.Password)

	result,err := db.Exec(statement)

	if err != nil {
		return err
	}

	u.Id,err = result.LastInsertId()

	if err != nil {
		return err
	}

	return nil
}


func (u *user) deactivateUser(db *sql.DB) error {

	statement := fmt.Sprintf("update users set status='%s' where id=%d", "INACTIVE",u.Id )
	_,err := db.Exec(statement)
	if err != nil {
		return err
	}
	return nil
}


/*
	TODO: better error handling
	We make two queries to the database to get data from 4 tables
	In the first query, we get user's personal info and his/her skills, endorsements and their counts
	In the second query, we get user's friends and any pending requests/approvals
*/
func (u *user) getUser(db *sql.DB) error {

	//var endorsers string
	// Query 1
	statement := fmt.Sprintf("select u.firstname,u.lastname,u.email, u.password, " +
									"s.SkillID, s.Skillname, s.EndorsementCount," +
									"(select GROUP_CONCAT(endorsedby SEPARATOR ',') from user_endorsements e " +
											"where e.UserID=u.ID and e.SkillID=s.SkillID group by e.UserID, e.SkillID) " +
									"endorsedby " +
									"from users u left join user_skills s on s.UserID=u.ID where u.ID=%d and u.status='%s' " +
									"order by EndorsementCount desc", u.Id, "ACTIVE")

	log.Println("Running statement: %s", statement)
	rows, err := db.Query(statement)


	if err != nil {
		return err
	}

	var endorsedBy, skillName NullString
	var endorsedByStr string
	var skillId, endorsementCount NullInt64

	userSkills := []userskill{}
	var count int = 0
	for rows.Next() {
		count++
		s := userskill{}
		if err := rows.Scan(&u.FirstName,&u.LastName, &u.Email, &u.Password, &skillId, &skillName, &endorsementCount, &endorsedBy); err != nil {
			return err
		}

		var e []int64
		for _, i := range strings.Split(endorsedByStr, ",") {
			j, _ := strconv.ParseInt(i, 10, 64)
			//log.Println("Appending ", j)
			if j>0 {
				e = append(e, j)
			}
		}
		s.EndorsedBy = e

		if endorsedBy.Valid {
			endorsedByStr =  endorsedBy.String
		} else {
			endorsedByStr = ""
		}

		if skillId.Valid {
			s.SkillId = skillId.Int64
		} else {
			s.SkillId = 0
		}

		if skillName.Valid {
			s.SkillName = skillName.String
		} else {
			s.SkillName = ""
		}

		if endorsementCount.Valid {
			s.EndorsementCount = endorsementCount.Int64
		} else {
			s.EndorsementCount = 0
		}

		userSkills = append(userSkills, s)

	}

	if count == 0 {
		return sql.ErrNoRows
	}

	u.UserSkills = userSkills

	// Query 2
	statement = fmt.Sprintf("select RequestedFrom, RequestedTo, Status from friendships where RequestedFrom=%d or RequestedTo=%d", u.Id, u.Id)
	log.Println ("Running statement2: %s", statement)
	rows, err = db.Query(statement)

	if err != nil {
		return err
	}

	var RequestedFrom, RequestedTo int64
	var status string

	for rows.Next() {
		if err := rows.Scan(&RequestedFrom,&RequestedTo,&status); err != nil {
			return err
		}

		if (RequestedFrom == u.Id) {
			// RequestedFrom is me
			// RequestedTo is someone
			// I (var u) requested this friendship
			if (status=="APPROVED") {
				// friendship approved, he/she is my friend
				u.Friends = append(u.Friends, RequestedTo)
			} else {
				// he/she hasn't approved my request yet
				u.PendingFriendRequests = append(u.PendingFriendRequests, RequestedTo)
			}
		} else {
			// RequestedFrom is someone
			// RequestedTo is me
			// I (var u) Received this friendship
			if (status=="APPROVED") {
				// I approved it, he/she is my friend
				u.Friends = append(u.Friends, RequestedFrom)
			} else {
				// I need to approve or reject it
				u.PendingFriendApprovals = append(u.PendingFriendApprovals, RequestedFrom)
			}
		}
	}

	defer rows.Close()
	return nil
}


func (s *userskill) addSkill(db *sql.DB) error {

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Rollback()

	statement := fmt.Sprintf("select id from skills where lower(name) = lower('%s')", s.SkillName)
	err = tx.QueryRow(statement).Scan(&s.SkillId)

	if err == sql.ErrNoRows {
		statement = fmt.Sprintf("insert into skills(name, count) values('%s',%d)", s.SkillName, 0)
		result, err := tx.Exec(statement)
		if err != nil {
			return err
		}

		s.SkillId, err = result.LastInsertId()
		if err != nil {
			return err
		}
	} else {
		return err
	}

	statement = fmt.Sprintf("insert user_skills values(%d,%d,'%s',%d)", s.UserId, s.SkillId, s.SkillName, 0)
	_,err = tx.Exec(statement)
	if err != nil {
		return err
	}

	statement = fmt.Sprintf("update skills set count=count+1 where id=%d",s.SkillId)
	_,err = tx.Exec(statement)
	if err != nil {
		return err
	}

	tx.Commit()
	s.EndorsementCount = 0
	return nil
}


func (s *userskill) endorseSkill(endorserId int64, db *sql.DB) error {

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Rollback()

	statement := fmt.Sprintf("insert into user_endorsements(userid,skillid,endorsedby) values(%d,%d,%d)", s.UserId, s.SkillId, endorserId)
	_,err=tx.Exec(statement)
	if err != nil {
		return err
	}

	statement = fmt.Sprintf("update user_skills set endorsementcount = endorsementcount + 1 " +
									"where userid=%d and skillid=%d",
									s.UserId, s.SkillId)
	_,err=tx.Exec(statement)
	if err != nil {
		return err
	}

	tx.Commit()
	return nil
}


func (u *user) removeEndorsement(skillId int64, endorserId int64, endorseeId int64, db *sql.DB) error {


	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Rollback()

	statement := fmt.Sprintf("delete from user_endorsements where userid=%d and endorsedby=%d and skillid=%d", endorseeId, endorserId, skillId)
	_,err=tx.Exec(statement)
	if err != nil {
		return err
	}

	statement = fmt.Sprintf("update user_skills set endorsementcount = endorsementcount - 1 where userid=%d and skillid=%d", endorseeId, skillId)
	_,err=tx.Exec(statement)
	if err != nil {
		return err
	}

	tx.Commit()

	return nil
}


func (s *userskill) removeSkill(db *sql.DB) error {

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Rollback()

	statement := fmt.Sprintf("delete from user_skills where userid=%d and skillid=%d", s.UserId, s.SkillId)
	_,err = tx.Exec(statement)
	if err != nil {
		return err
	}


	statement = fmt.Sprintf("delete from user_endorsements where userid=%d and skillid=%d", s.UserId, s.SkillId)
	_,err = tx.Exec(statement)
	if err != nil {
		return err
	}

	statement = fmt.Sprintf("update skills set count=count-1 where id=%d",s.SkillId)
	_,err = tx.Exec(statement)
	if err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (u *user) requestFriendship(toUser int64, db *sql.DB) error {

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Rollback()

	statement := fmt.Sprintf("insert into friendships values(%d,%d,'%s')", u.Id,toUser,"PENDING")
	_,err = tx.Exec(statement)
	if err != nil {
		return err
	}

	tx.Commit()

	return nil
}



func (u *user) approveFriendship(fromUser int64, db *sql.DB) error {

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Rollback()

	statement := fmt.Sprintf("update friendships set status='%s' where requestedfrom=%d and requestedto=%d",
		"ACCEPTED",fromUser,u.Id)
	_,err = tx.Exec(statement)
	if err != nil {
		return err
	}

	tx.Commit()

	return nil
}




/*
select s.name, s.count, l.EndorsementCount, u.ID, u.FirstName
from skills s, user_skills l, users u
where s.ID=1 and l.SkillID=s.ID
and u.ID=l.UserID
order by EndorsementCount desc
 */

func (u *user) getSkills(skillName string, userId int64, db *sql.DB) error {
	return nil
}
