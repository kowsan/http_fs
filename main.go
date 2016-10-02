// http_fs project main.go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fogcreek/mini"
	"github.com/kardianos/osext"

	"database/sql"

	"github.com/kowsan/libsync/synclib"

	_ "github.com/lib/pq"
)

var (
	dirpath   string
	asdir     string
	addr      string
	use_csumm bool
	maindb    *sql.DB
)

func export() {

}
func init() {
	log.Println("App init")
	//read ini cfg file
	folderPath, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(folderPath)
	cfg, cfge := mini.LoadConfiguration(folderPath + "/DBConfig.ini")
	if cfge != nil {
		log.Fatalln("Could not find config file", cfge)
	}

	host := cfg.StringFromSection("GLOBAL", "dbhost", "127.0.0.1")
	port := cfg.StringFromSection("GLOBAL", "port", "5432")
	login := cfg.StringFromSection("GLOBAL", "login", "postgres")
	password := cfg.StringFromSection("GLOBAL", "password", "postgres")
	dbname := cfg.StringFromSection("GLOBAL", "maindbname", "strizh")
	connstr := "user=" + login + " password=" + password + " dbname=" + dbname + " port=" + port + " host=" + host + " sslmode=disable"
	db, err := sql.Open("postgres", connstr)
	if err != nil {
		log.Fatal("Could not open db")
	}
	e := db.Ping()
	if e != nil {
		log.Fatalln("db opened fail", e.Error())
	} else {
		log.Println("Database ok opened", db.Stats())
		db.SetMaxIdleConns(2)
		db.SetMaxOpenConns(10)

		maindb = db
	}

}

type User struct {
	Login     string `json:"login"`
	IsAdmin   bool   `json:"is_admin"`
	SurName   string `json:"surname"`
	FirstName string `json:"firstname"`
	PatrName  string `json:"patrname"`
	GroupName string `json:"groupname"`
}

type QTask struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group_name"`
	Quiz  string `json:"quiz"`
}
type Quiz struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	SecretWord string `json:"secret_word"`
	TaskName   string `json:"task_name"`
	Content    string `json:"content"`
}

type Course struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	InputPlanId         *int64 `json:"input_plan_id"`
	OutputPlanId        *int64 `json:"output_plan_id"`
	InputPlan           *QTask `json:"input_plan"`
	OutputPlan          *QTask `json:"output_plan"`
	SkipInputPlanResult bool   `json:"skip"`
}
type Plan struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}
type CoursePlan struct {
	InputPlanId         interface{} `json:"input_plan_id"`
	OutputPlanId        interface{} `json:"output_plan_id"`
	SkipInputPlanResult bool        `json:"skip"`
}
type fsHandler struct {
}

func (fs *fsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {

	case "/index.go":
		log.Println("Begin build file structure")
		out := synclib.BuildFileStructure(dirpath, use_csumm)
		log.Println("End build file structure")
		log.Println("Begin send response file structure")
		b, _ := json.Marshal(out)

		w.WriteHeader(200)
		w.Write(b)
		b = nil

		log.Println("response sended")

	case "/plans":
		plans(w, r)

	case "/tasks":
		tasks(w, r)
	case "/users":
		users(w, r)
	default:
		http.ServeFile(w, r, dirpath+r.URL.Path)
	}

	return
	if r.URL.Path == "/index.go" {

		log.Println("Begin build file structure")
		out := synclib.BuildFileStructure(dirpath, use_csumm)
		log.Println("End build file structure")
		log.Println("Begin send response file structure")
		b, _ := json.Marshal(out)

		w.WriteHeader(200)
		w.Write(b)
		b = nil

		log.Println("response sended")
	} else {
		if r.URL.Path == "/plans" {
			plans(w, r)
		} else {
			http.ServeFile(w, r, dirpath+r.URL.Path)
		}

	}

}

func users(w http.ResponseWriter, r *http.Request) {
	user_id := r.URL.Query().Get("id")
	log.Println("Request user  with id ", user_id)
	row := maindb.QueryRow(`select  "LOGIN","IS_ADMIN","SURNAME","FIRST_NAME","PATR_NAME","GROUPS"."Name" from "USERS","GROUPS" where "USERS"."ID"=$1 and "GROUP_ID"="GROUPS"."ID" `, user_id)
	user := User{}
	e := row.Scan(&user.Login, &user.IsAdmin, &user.SurName, &user.FirstName, &user.PatrName, &user.GroupName)
	if e != nil {
		log.Println("User find error", e)
		w.WriteHeader(404)
	} else {

		w.WriteHeader(200)
		b, _ := json.Marshal(user)
		w.Write(b)

	}
}
func tasks(w http.ResponseWriter, r *http.Request) {
	qtask_id := r.URL.Query().Get("id")
	log.Println("Request task content with id ", qtask_id)
	row := maindb.QueryRow(`select  "Content" from "QTASKS" where "ID"=$1 `, qtask_id)
	var content string
	e := row.Scan(&content)
	if e != nil {
		w.WriteHeader(404)
	} else {

		w.WriteHeader(200)
		w.Write([]byte(content))

	}
}
func savePlansForCourse(w http.ResponseWriter, r *http.Request) {
	courseid := r.URL.Query().Get("id")
	if r.Method != "POST" {
		log.Println("Could not accept request")
		w.WriteHeader(422)
		return
	}
	if courseid == "" {
		w.WriteHeader(404)
		return
	}
	e := r.ParseForm()
	if e != nil {
		log.Println("could not parse input params ", e)
		w.WriteHeader(422)
		return
	}
	var input_plan_id interface{}
	var output_plan_id interface{}
	input_plan_id = r.FormValue("input_plan_id")
	output_plan_id = r.FormValue("output_plan_id")
	if input_plan_id == "" {
		input_plan_id = nil
	}
	if output_plan_id == "" {
		output_plan_id = nil
	}
	skip := r.FormValue("skip")
	log.Println(input_plan_id, output_plan_id, skip)
	update_req, err := maindb.Exec(`update "Cources" set "input_plan_id"=$1,  "output_plan_id"=$2 , "skip_input_plan_result"=$3 where "ID"=$4`, input_plan_id, output_plan_id, skip, courseid)
	if err != nil {
		log.Println("Could not update course ", courseid, " Error : ", err.Error())
		w.WriteHeader(500)
	} else {
		r, _ := update_req.RowsAffected()
		log.Println("Updated rows: ", r)
		w.WriteHeader(204)
	}

}
func plansForCourse(w http.ResponseWriter, r *http.Request) {
	//get course by id
	courseid := r.URL.Query().Get("id")
	if courseid == "" {
		w.WriteHeader(404)
		return
	}
	log.Println("Request info abiut course ", courseid)
	res := maindb.QueryRow(`select "input_plan_id","output_plan_id","skip_input_plan_result" from "Cources" where "ID"=($1)`, courseid)
	var input_plan_id interface{}
	var output_plan_id interface{}
	var skip bool
	err := res.Scan(&input_plan_id, &output_plan_id, &skip)
	if err != nil {
		log.Println("cannot scan row ", err)
		w.WriteHeader(404)
	} else {
		p := CoursePlan{}
		p.InputPlanId = input_plan_id
		p.OutputPlanId = output_plan_id
		p.SkipInputPlanResult = skip
		b, _ := json.Marshal(p)
		w.Write(b)
	}

	//		var out []Plan
	//		defer res.Close()
	//		for res.Next() {
	//			var id int64
	//			var name string
	//			if err := res.Scan(&id, &name); err != nil {
	//				log.Println("cannot scan row ", err)
	//			} else {
	//				p := Plan{}
	//				p.Id = id
	//				p.Name = name
	//				out = append(out, p)

	//			}
	//		}
	//		b, e := json.Marshal(out)
	//		if e != nil {
	//			log.Println("could not serialize plans", e.Error())
	//		} else {

	//			w.Header().Set("Content-Type", "application/json; charset=utf-8")
	//			w.Write(b)
	//			//w.WriteHeader(200)
	//		}
	//	}
}

func plans(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("id") != "" && r.Method == "GET" {
		plansForCourse(w, r)
		return
	} else if r.URL.Query().Get("id") != "" && r.Method == "POST" {
		savePlansForCourse(w, r)
		return
	}

	res, e := maindb.Query(`select "ID","Name" from "QTASKS"`)
	if e != nil {
		log.Println("Could not get plans from db", e)
		w.WriteHeader(500)
	} else {
		var out []Plan
		defer res.Close()
		for res.Next() {
			var id int64
			var name string
			if err := res.Scan(&id, &name); err != nil {
				log.Println("cannot scan row ", err)
			} else {
				p := Plan{}
				p.Id = id
				p.Name = name
				out = append(out, p)

			}
		}
		b, e := json.Marshal(out)
		if e != nil {
			log.Println("could not serialize plans", e.Error())
		} else {

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(b)
			//w.WriteHeader(200)
		}
	}
}
func main() {
	log.Println("Starting server")
	flag.StringVar(&addr, "addr", ":8888", "http addr (:port or host:port) ")
	flag.StringVar(&dirpath, "dirpath", "", "set directory to serve files ")
	flag.StringVar(&asdir, "asdir", "/", "serve as")
	flag.BoolVar(&use_csumm, "csumm", false, "Use file control summ")
	flag.Parse()

	dirpath = strings.TrimSuffix(dirpath, "/")
	log.Println("directory set to :", dirpath)

	log.Println("Serve as  :", asdir)
	log.Println("Csumm  :", use_csumm)
	myHandler := &fsHandler{}

	//fs := http.FileServer(http.Dir(dirpath))
	//	http.Handle(asdir, http.StripPrefix(asdir, fs))
	s := &http.Server{
		WriteTimeout: 50 * time.Minute,
		Addr:         addr,
		Handler:      myHandler,
	}

	http.HandleFunc(asdir, func(w http.ResponseWriter, r *http.Request) {

		http.ServeFile(w, r, dirpath+r.URL.Path)
	})
	//	http.HandleFunc(asdir+"index.go", func(w http.ResponseWriter, r *http.Request) {
	//		log.Println("Begin build file structure")
	//		out := synclib.BuildFileStructure(dirpath, use_csumm)
	//		log.Println("End build file structure")
	//		log.Println("Begin send response file structure")
	//		b, _ := json.Marshal(out)

	//		w.WriteHeader(200)
	//		w.Write(b)
	//		log.Println("response sended")
	//	})
	log.Println("start http server at : ", addr)

	s.ListenAndServe()

}
