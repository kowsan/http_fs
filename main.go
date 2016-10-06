// http_fs project main.go
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fogcreek/mini"
	"github.com/kardianos/osext"

	"database/sql"

	"github.com/kowsan/libsync/synclib"

	_ "github.com/lib/pq"
)

const tf = "2006-01-02_150405"

var (
	dirpath   string
	asdir     string
	addr      string
	use_csumm bool
	maindb    *sql.DB
)

type User struct {
	Login     string `json:"login"`
	IsAdmin   bool   `json:"is_admin"`
	SurName   string `json:"surname"`
	FirstName string `json:"firstname"`
	PatrName  string `json:"patrname"`
	GroupName string `json:"groupname"`
}

type QTask struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	//Group    string `json:"group_name"`
	Content  string `json:"content"`
	QuizTask Quiz   `json:"quiz_task"`
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
	SecretWord          string `json:"secret_word"`
	TaskName            string `json:"task_name"`
	InputPlanId         *int64 `json:"input_plan_id"`
	OutputPlanId        *int64 `json:"output_plan_id"`
	InputPlan           *QTask `json:"input_plan"`
	OutputPlan          *QTask `json:"output_plan"`
	SkipInputPlanResult bool   `json:"skip"`
}

func loadQuizTaskById(id *int64) *QTask {
	row := maindb.QueryRow(`select t."ID" as task_id,t."Name" as task_name,t."Content"  as task_content,
q."ID" as quiz_id,q."Name" as quiz_name,q."SecretWord",
q."TaskName",q."Content" from "QTASKS" t 
left join "Quiz" q on q."ID"=t."SRC_ID" where t."ID"=$1`, id)
	qt := QTask{}

	se := row.Scan(&qt.ID, &qt.Name, &qt.Content, &qt.QuizTask.ID, &qt.QuizTask.Name, &qt.QuizTask.SecretWord, &qt.QuizTask.TaskName, &qt.QuizTask.Content)
	if se != nil {
		log.Println("could not get tasks by id ", id, " error : ", se)
	}
	return &qt
}
func showImportHtml(w http.ResponseWriter, r *http.Request) {
	ef, _ := osext.ExecutableFolder()
	b, _ := ioutil.ReadFile(ef + "/import.html")
	w.Write(b)

}

func maitenance(w http.ResponseWriter, r *http.Request) {
	if validateQTaskContrains() == false {

		es := "Doublicates found\n"
		res, e := maindb.Query(`select "TaskName",count("TaskName") from "QTASKS" group by "TaskName" having count(*)>1`)
		if e != nil {
			log.Println("critical erro get uniq")
			w.WriteHeader(500)
			return
		}
		defer res.Close()
		for res.Next() {
			var name string
			var count string
			res.Scan(&name, &count)
			es += "TaskName :  '" + name + "' Times : " + count + "\n"
		}
		es += "Please rename Qtasks(kontrolnye) and reload this page"
		w.Write([]byte(es))

	} else {
		w.Write([]byte("DB contrains ok"))

		return
	}
}
func importFromJson(w http.ResponseWriter, r *http.Request) {
	//load group_id

	log.Println("Begin file import")
	r.ParseMultipartForm(150000000)
	file, _, err := r.FormFile("json_file")
	if err != nil {

		log.Println("Unacessible file ", err)
		w.WriteHeader(406)
		return
	}
	defer file.Close()
	log.Println("File ok : read it")
	x, e := ioutil.ReadAll(file)
	if e != nil {
		log.Println("could not read file , ", e)
		w.WriteHeader(500)
		return
	}
	log.Println("file ok : ")
	cx := []Course{}
	json.Unmarshal(x, &cx)
	//log.Println("cx", cx)
	tx, _ := maindb.Begin()
	for _, v := range cx {
		var course_id int64
		course_id = 0
		//log.Println("input plan for course : ", v.ID, v.InputPlan)
		//log.Println("output plan for course : ", v.ID, v.OutputPlan)
		log.Println("Testing course for exists", v.ID, v.Name, v.SecretWord, v.TaskName)
		log.Println("Search course by taskName ", v.TaskName) //name is unique
		crow := tx.QueryRow(`select "ID" from "Cources" where "TaskName"=$1`, v.TaskName)

		crow.Scan(&course_id)
		log.Println("Getting course id : ", course_id)
		if course_id == 0 {
			log.Println("Course not found by taskname : ", v.TaskName, "Skip it")
		} else {
			if v.InputPlan == nil {
				log.Println("Input plan not found : ")
			} else {
				InPlanRestore(v)
			}
			if v.OutputPlan == nil {
				log.Println("Output plan not found : ")
			} else {

				OutPlanRestore(v)

			}

		}

	}
	tx.Commit()
	SelfUpdateQtasks()
	w.Write([]byte("OK"))

}
func SelfUpdateQtasks() {

	rx_id := regexp.MustCompile(`\sid=\"\d{1,}\"`)
	rx_quiz := regexp.MustCompile(`\scoursequizid=\"\d{1,}\"`)

	tx, _ := maindb.Begin()
	//	c := `<quizTask label="БЧ-5-4" id="7" slf_ctrl="0" time="60" mark5="90" groupId="2" coursequizid="22" mark3="50" mark4="70" canReturn="0" random="0">
	//    <uuids/>
	//</quizTask>
	//`
	rows, e := maindb.Query(`select "ID","SRC_ID","Content" from "QTASKS"`)

	if e != nil {
		log.Println("Erro getting qtasks ", e)
		tx.Rollback()
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var src_id int64
		var content string
		qte := rows.Scan(&id, &src_id, &content)
		log.Println("get qtask error ", qte)
		content = rx_id.ReplaceAllString(content, ` id="`+strconv.FormatInt(id, 10)+`"`)
		content = rx_quiz.ReplaceAllString(content, ` coursequizid="`+strconv.FormatInt(src_id, 10)+`"`)

		_, upp_res := tx.Exec(`update "QTASKS" set "Content"=$1 where "ID"=$2`, content, id)
		if upp_res != nil {
			log.Println("Could not update qtask ", id, upp_res)
		}
		log.Println("replace qtask to :", content)
	}

	//select each qtask and set <quiztask coursequizid="SRC_ID" id="ID"
	tx.Commit()

}
func OutPlanRestore(v Course) {
	group_id := 0
	grow := maindb.QueryRow(`select "GROUP_ID" from "USERS" where "IS_ADMIN" =false limit 1`)
	grow.Scan(&group_id)
	quizid_out, er := restorePlan(v.Name, *v.OutputPlan)
	log.Println("Process quizid , ", quizid_out)
	if er == nil {
		log.Println("Linking plan")
		log.Println("Try find existing plan and attach quiz")
		plan_name := ""
		var plan_id int64
		plan_id = 0
		row := maindb.QueryRow(`select "ID","Name" from "QTASKS" where "Name"=$1`, v.OutputPlan.Name)
		nfp := row.Scan(&plan_id, &plan_name)
		if nfp != nil {
			log.Println("Plan not found by Name ", v.OutputPlan.Name)
			log.Println("Create plan by name and link to quiz id")
			qtask_id := 0
			nc := strings.Replace(v.OutputPlan.Content, ` coursequizid="`+strconv.FormatInt(v.OutputPlan.ID, 10)+`"`, ` coursequizid="`+strconv.FormatInt(quizid_out, 10)+`"`, -1)
			//quizid==outputplan.id
			result := maindb.QueryRow(`insert into "QTASKS" ("Name","SRC_ID","Content","GROUP_ID") values($1,$2,$3,$4) returning "ID"`, v.OutputPlan.Name, quizid_out, nc, group_id)
			f := result.Scan(&qtask_id)
			log.Println("Could not add plan task : ", f)
			//assign qtask to course
			//nc = strings.Replace(nc, `coursequizid="`+strconv.FormatInt(course_id, 10)+`"`, `coursequizid="`+strconv.FormatInt(v.ID, 10)+`"`, -1)

		} else {
			//if plan exists
			log.Println("QTask exists")
			//nc2 := strings.Replace(v.OutputPlan.Content, ` coursequizid="`+strconv.FormatInt(v.OutputPlan.ID, 10)+`"`, ` coursequizid="`+strconv.FormatInt(quizid, 10)+`"`, -1)
			//						log.Println("Replacing to : ", nc2)
			_, ue := maindb.Exec(`update "QTASKS" set "SRC_ID"=$1,"Content"=$2 where "Name"=$3`, quizid_out, v.OutputPlan.Content, v.OutputPlan.Name)
			log.Println("errro updating existang QTASK", ue)

		}

	}
	log.Println("Assign OUT  plan to course")
	log.Println("Select Qtask by name")
	out_qtask_id := 0
	prow := maindb.QueryRow(`select "ID" from "QTASKS" where "Name"=$1`, v.OutputPlan.Name)
	xx := prow.Scan(&out_qtask_id)
	log.Println("error getting out_qtask_id", xx)
	if xx == nil {
		_, ue := maindb.Exec(`update "Cources" set "output_plan_id"=$1,"skip_input_plan_result"=$2,"Name"=$3  where "TaskName"=$4`, out_qtask_id, v.SkipInputPlanResult, v.Name, v.TaskName)
		log.Println("Error updating cource ", ue)
	}

	//	log.Println("Assign IN  plan to course")
	//	log.Println("Select IN Qtask by name")
	//	in_qtask_id := 0
	//	prow_in := maindb.QueryRow(`select "ID" from "QTASKS" where "Name"=$1`, v.InputPlan.Name)
	//	xx_in := prow_in.Scan(&in_qtask_id)
	//	log.Println("error getting out_qtask_id", xx)
	//	if xx_in == nil {
	//		_, ue := maindb.Exec(`update "Cources" set "input_plan_id"=$1  where "Name"=$2`, in_qtask_id, v.Name)
	//		log.Println("Error updating cource ", ue)
	//	}
}
func InPlanRestore(v Course) {
	group_id := 0
	grow := maindb.QueryRow(`select "GROUP_ID" from "USERS" where "IS_ADMIN" =false limit 1`)
	grow.Scan(&group_id)
	quizid_out, er := restorePlan(v.Name, *v.InputPlan)
	log.Println("Process quizid , ", quizid_out)
	if er == nil {
		log.Println("Linking plan")
		log.Println("Try find existing plan and attach quiz")
		plan_name := ""
		var plan_id int64
		plan_id = 0
		row := maindb.QueryRow(`select "ID","Name" from "QTASKS" where "Name"=$1`, v.InputPlan.Name)
		nfp := row.Scan(&plan_id, &plan_name)
		if nfp != nil {
			log.Println("Plan not found by Name ", v.InputPlan.Name)
			log.Println("Create plan by name and link to quiz id")
			qtask_id := 0
			nc := strings.Replace(v.InputPlan.Content, ` coursequizid="`+strconv.FormatInt(v.InputPlan.ID, 10)+`"`, ` coursequizid="`+strconv.FormatInt(quizid_out, 10)+`"`, -1)
			//quizid==outputplan.id
			result := maindb.QueryRow(`insert into "QTASKS" ("Name","SRC_ID","Content","GROUP_ID") values($1,$2,$3,$4) returning "ID"`, v.InputPlan.Name, quizid_out, nc, group_id)
			f := result.Scan(&qtask_id)
			log.Println("Could not add plan task : ", f)
			//assign qtask to course
			//nc = strings.Replace(nc, `coursequizid="`+strconv.FormatInt(course_id, 10)+`"`, `coursequizid="`+strconv.FormatInt(v.ID, 10)+`"`, -1)

		} else {
			//if plan exists
			log.Println("QTask exists")
			//nc2 := strings.Replace(v.OutputPlan.Content, ` coursequizid="`+strconv.FormatInt(v.OutputPlan.ID, 10)+`"`, ` coursequizid="`+strconv.FormatInt(quizid, 10)+`"`, -1)
			//						log.Println("Replacing to : ", nc2)
			_, ue := maindb.Exec(`update "QTASKS" set "SRC_ID"=$1,"Content"=$2 where "Name"=$3`, quizid_out, v.InputPlan.Content, v.InputPlan.Name)
			log.Println("errro updating existang QTASK", ue)

		}

	}
	log.Println("Assign OUT  plan to course")
	log.Println("Select Qtask by name")
	out_qtask_id := 0
	prow := maindb.QueryRow(`select "ID" from "QTASKS" where "Name"=$1`, v.InputPlan.Name)
	xx := prow.Scan(&out_qtask_id)
	log.Println("error getting out_qtask_id", xx)
	if xx == nil {
		_, ue := maindb.Exec(`update "Cources" set "input_plan_id"=$1  where "TaskName"=$2`, out_qtask_id, v.TaskName)
		log.Println("Error updating cource ", ue)
	}

}
func validateQTaskContrains() bool {
	q := maindb.QueryRow(`SELECT 1 FROM pg_constraint WHERE conname = 'Qtask_unique_name'`)
	name := -1
	q.Scan(&name)
	if name == 1 {
		log.Println("Qtask already unique")
		return true
	} else {
		_, e := maindb.Exec(`ALTER TABLE public."QTASKS" ADD CONSTRAINT "Qtask_unique_name" UNIQUE ("Name");`)

		if e != nil {
			log.Println("Could not add contrains", e)
			return false
		}
		return true
	}

	//ALTER TABLE public."QTASKS"
	//  ADD CONSTRAINT "Qtask_unique_name" UNIQUE ("Name");

}
func restorePlan(course_name string, t QTask) (int64, error) {
	log.Println("plan are : ", t.Name, t.QuizTask.Name)

	//restore quiz and link it to Task
	q := t.QuizTask
	//select if quiz exists
	//q.Name
	qname := ""
	var qid int64
	qid = 0
	row := maindb.QueryRow(`select "ID","TaskName" from "Quiz" where "TaskName"=$1`, q.TaskName)
	err := row.Scan(&qid, &qname)

	if err != nil {
		log.Println("could not get Quiz err ", err, "Try create quiz")
		//return
	}
	if qname == "" {
		log.Println("Quiz not found, try create it and patch id")
		result := maindb.QueryRow(`insert into "Quiz" ("Name","SecretWord","TaskName","Content") values($1,$2,$3,$4) returning "ID"`, q.Name, q.SecretWord, q.TaskName, q.Content)
		f := result.Scan(&qid)
		log.Println("quiz inserted : ", qid)
		if f != nil {
			log.Println("Could not update quiz by name, ", f)
			return 0, errors.New("Invalid quiz!!!")
		}

	} else {
		log.Println("Quiz already exists,just update it", qid)
	}
	//update quiz content with normal coursequizid=="currentid"
	log.Println("Finally update coure")
	log.Println("update quiz content with normal coursequizid==''", qid)
	rc := strings.Replace(q.Content, " id=\""+strconv.FormatInt(q.ID, 10)+"\">", " id=\""+strconv.FormatInt(qid, 10)+"\">", -1)
	//	log.Println("Replcae quiz content to : ", rc)
	_, ex := maindb.Exec(`update "Quiz" set "Name"=$1,"SecretWord"=$2,"TaskName"=$3,"Content"=$4 where "ID"=$5`, q.Name, q.SecretWord, q.TaskName, rc, qid)
	log.Println("Error quiz updating", ex)
	return qid, nil

}

func courcesList(w http.ResponseWriter, r *http.Request) {
	rows, e := maindb.Query(`select "ID","Name","SecretWord","TaskName","input_plan_id","output_plan_id","skip_input_plan_result" from "Cources" where input_plan_id is not null or output_plan_id is not null`)
	if e != nil {
		log.Println("Could not load cources")
		w.WriteHeader(500)
		return
	} else {
		cources := []Course{}
		log.Println("Cources loaded")
		defer rows.Close()
		for rows.Next() {
			c := Course{}
			e := rows.Scan(&c.ID, &c.Name, &c.SecretWord, &c.TaskName, &c.InputPlanId, &c.OutputPlanId, &c.SkipInputPlanResult)
			if e != nil {
				log.Println("Read row error ,", e)
			}
			//			log.Println("Read row error ,", e)
			if c.InputPlanId != nil {
				c.InputPlan = loadQuizTaskById(c.InputPlanId)
			}
			if c.OutputPlanId != nil {
				c.OutputPlan = loadQuizTaskById(c.OutputPlanId)
			}
			cources = append(cources, c)

		}

		w.Header().Set("Content-Disposition", "attachment; filename=cources_export_"+(time.Now().Format(tf))+".json")
		w.WriteHeader(200)

		b, _ := json.Marshal(cources)
		w.Write(b)
	}

}

func init() {

	log.Println("Current time", time.Now().Format(tf))
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
		validateQTaskContrains()
	}

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

	case "/cources.list":
		courcesList(w, r)

	case "/import.html":
		showImportHtml(w, r)

	case "/maitenance":
		maitenance(w, r)

	case "/import.json":
		importFromJson(w, r)

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
