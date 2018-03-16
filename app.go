package main

import (
	cryptorand "crypto/rand"
	"database/sql"
	"github.com/gorilla/mux"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"encoding/json"
	"github.com/gorilla/handlers"
	"os"
	//"github.com/dgrijalva/jwt-go"
	//"github.com/gorilla/context"
	//"github.com/mitchellh/mapstructure"
	"github.com/dgrijalva/jwt-go"
	//"go/token"
	//"encoding/base64"
	"time"
	//"github.com/satori/go.uuid"
	//"go/token"
	"crypto/rsa"
	"encoding/pem"
	"crypto/x509"
	"bytes"
	//"go/token"
	"github.com/davecgh/go-spew/spew"
)


var signingKey, verificationKey []byte


type App struct {
	Router *mux.Router
	DB     *sql.DB
}


func initKeys() {
	var (
		err         error
		privKey     *rsa.PrivateKey
		pubKey      *rsa.PublicKey
		pubKeyBytes []byte
	)

	privKey, err = rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		log.Fatal("Error generating private key")
	}
	pubKey = &privKey.PublicKey //hmm, this is stdlib manner...

	// Create signingKey from privKey
	// prepare PEM block
	var privPEMBlock = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey), // serialize private key bytes
	}
	// serialize pem
	privKeyPEMBuffer := new(bytes.Buffer)
	pem.Encode(privKeyPEMBuffer, privPEMBlock)
	//done
	signingKey = privKeyPEMBuffer.Bytes()

	fmt.Println(string(signingKey))

	// create verificationKey from pubKey. Also in PEM-format
	pubKeyBytes, err = x509.MarshalPKIXPublicKey(pubKey) //serialize key bytes
	if err != nil {
		// heh, fatality
		log.Fatal("Error marshalling public key")
	}

	var pubPEMBlock = &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	}
	// serialize pem
	pubKeyPEMBuffer := new(bytes.Buffer)
	pem.Encode(pubKeyPEMBuffer, pubPEMBlock)
	// done
	verificationKey = pubKeyPEMBuffer.Bytes()

	fmt.Println(string(verificationKey))
}

func (a *App) Initialize(user, password, dbname string) {
	log.Println("Initializing the app")
	initKeys()
	connectionString := fmt.Sprintf("%s:%s@/%s", user, password, dbname)
	var err error
	a.DB, err = sql.Open("mysql", connectionString)
	if err != nil {
		log.Fatal(err)
	}
	a.Router = mux.NewRouter()
	a.initializeRoutes()
}


func (a *App) initializeRoutes() {

	a.Router.HandleFunc("/login",a.loginEndpoint).Methods("POST")
	a.Router.HandleFunc("/user", a.createUser).Methods("POST")
	a.Router.Handle("/user/{id:[0-9]+}/add_skill",a.AuthMiddleware(http.HandlerFunc(a.addSkill)))
	a.Router.Handle("/user/{id:[0-9]+}/view_user/{targetUserId:[0-9]+}", a.AuthMiddleware(http.HandlerFunc(a.getUserView))).Methods("GET")

	a.Router.Handle("/user/{id:[0-9]+}/add_skill", a.AuthMiddleware(http.HandlerFunc(a.addSkill))).Methods("POST")
	a.Router.Handle("/user/{id:[0-9]+}/endorse_skill", a.AuthMiddleware(http.HandlerFunc(a.endorseSkill))).Methods("POST")
	a.Router.Handle("/user/{id:[0-9]+}/remove_skill/{skillid:[0-9]+}", a.AuthMiddleware(http.HandlerFunc(a.removeSkill))).Methods("DELETE")
	a.Router.Handle("/user/{id:[0-9]+}/deactivate", a.AuthMiddleware(http.HandlerFunc(a.deactivateUser))).Methods("PUT")
	a.Router.Handle("/user/{id:[0-9]+}/request_friend/{targetId:[0-9]+}", a.AuthMiddleware(http.HandlerFunc(a.requestFriendship))).Methods("PUT")
	a.Router.Handle("/user/{id:[0-9]+}/approve_friend/{targetId:[0-9]+}", a.AuthMiddleware(http.HandlerFunc(a.approveFriendship))).Methods("PUT")
	//a.Router.HandleFunc("/user/{id:[0-9]+}", a.updateUser).Methods("PUT")
	//a.Router.HandleFunc("/user/{id:[0-9]+}", a.deleteUser).Methods("DELETE")
	//a.Router.Handle("/", handleAll).Methods("GET")
}


func (a *App) Run(addr string) {
	//log.Fatal(http.ListenAndServe(addr, a.Router))
	http.ListenAndServe(addr,handlers.LoggingHandler(os.Stdout, a.Router))
}

func handleAll(w http.ResponseWriter, r *http.Request) {
	log.Println("Got request")
}


func (a *App) loginEndpoint(w http.ResponseWriter, req *http.Request) {
	u := user{}
	_ = json.NewDecoder(req.Body).Decode(&u)
	password := u.Password


	if err := u.getUser(a.DB); err != nil {
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "UserID not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if password != u.Password {
		respondWithError(w, http.StatusForbidden, "Authentication failed")
		return
	} else {
		token := jwt.New(jwt.SigningMethodHS256)
		claims := make(jwt.MapClaims)
		claims["exp"] = time.Now().Add(time.Hour * time.Duration(1)).Unix()
		claims["iat"] = time.Now().Unix()
		claims["userInfo"] = u
		token.Claims = claims

		tokenString, error := token.SignedString(signingKey)

		if error != nil {
			respondWithError(w, http.StatusInternalServerError, error.Error())
		}
		respondWithJSON(w, http.StatusOK, JwtToken{Token: tokenString, User:u})
	}

}

func (a *App) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Session-Token")

		if token == "" {
			respondWithError(w, http.StatusForbidden, "Authentication failed")
		}

		_,err := a.validateToken(token)

		if err != nil {
			respondWithError(w, http.StatusForbidden, "Authentication failed")
		} else {
			next.ServeHTTP(w, r)
		}
	})
}


func (a *App) validateToken(tokenStr string) (jwt.MapClaims, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.

	fmt.Printf("Validating token: %s",tokenStr)

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return signingKey, nil
	})

	spew.Dump(token)

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims,nil
	} else {
		return nil,err
	}
}

func (a *App) getUserView(w http.ResponseWriter, r *http.Request) {
	log.Println("Got request to get user")
	vars := mux.Vars(r)
	targetUserId, err := strconv.ParseInt(vars["targetUserId"],10,64)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	id, err := strconv.ParseInt(vars["id"],10,64)

	u := user{Id: targetUserId}
	if err := u.getUser(a.DB); err != nil {
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "User not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	u.Password=""
	u.PendingFriendRequests=nil
	u.PendingFriendApprovals=nil
	resultView := viewuser{LoggedInUserId: id, ViewUserInfo: u, isFriend:false}

	for i:=0; i<len(u.Friends); i++ {
		if id == u.Friends[i] {
			resultView.isFriend=true
			break
		}
	}

	respondWithJSON(w, http.StatusOK, resultView)
}


func (a *App) createUser(w http.ResponseWriter, r *http.Request) {
	var u user
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&u); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	if err := u.addUser(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusCreated, u)
}


func (a *App) addSkill(w http.ResponseWriter, r *http.Request) {

	var s userskill

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"],10,64)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	//u := user{Id: id}
	s.UserId = id
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&s); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	if err := s.addSkill(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusCreated, s)

}


func (a *App) requestFriendship(w http.ResponseWriter, r *http.Request) {

}

func (a *App) approveFriendship(w http.ResponseWriter, r *http.Request) {

}


func (a *App) endorseSkill(w http.ResponseWriter, r *http.Request) {

	var s userskill

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"],10,64)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&s); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	defer r.Body.Close()
	if err := s.endorseSkill(id,a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})

}


func (a *App) removeSkill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"],10,64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid User ID")
		return
	}

	skillid, err := strconv.ParseInt(vars["skillid"],10,64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Skill ID")
		return
	}

	s := userskill{ UserId:id, SkillId:skillid }
	if err := s.removeSkill(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) deactivateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"],10,64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid User ID")
		return
	}

	u := user{Id: id}

	if err := u.deactivateUser(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}


func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}


		/*
		//jwt.
		//if user, found := amw.tokenUsers[token]; found {
			// We found the token in our map
			//log.Printf("Authenticated user %s\n", user)
			// Pass down the request to the next middleware (or final handler)
			///next.ServeHTTP(w, r)
		//} else {
			// Write an error and stop the handler chain
			///http.Error(w, "Forbidden", 403)
		//}
 		*/