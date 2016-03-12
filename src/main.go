package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/now"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var sqlString string = "gimvic:GimVicServer@/gimvic"
var api_key string = "ede5e730-8464-11e3-baa7-0800200c9a66"

var schTeachers []string

func main() {
	now.FirstDayMonday = true

	if len(os.Args) <= 1 {
		fmt.Println("Add argument sch, sub or menu!")
		os.Exit(1)
	}
	arg := os.Args[1]
	if arg == "sch" {
		updateSchedule()
	} else if arg == "sub" {
		updateSubstitutions()
	} else if arg == "menu" {
		parseMenu(os.Args)
	} else {
		fmt.Println("Add argument sch, sub or menu!")
		os.Exit(1)
	}
}

func updateSubstitutions() {
	currentDate := now.BeginningOfWeek()
	oneDay := time.Date(2015, 11, 30, 0, 0, 0, 0, time.UTC).Sub(time.Date(2015, 11, 29, 0, 0, 0, 0, time.UTC))

	//parsing each day od substitutions in next 10 days
	for i := 0; i < 10; i++ {
		data := getSubstitutionsForDate(currentDate)

		if data.DateStr != "" {
			db, err := sql.Open("mysql", sqlString)
			check(err)
			_, err = db.Exec("delete from substitutions where date='" + data.DateStr + "';")
			check(err)

			//parsing normal substitutions
			for _, substutution := range data.Substitutions {
				for _, substututionLesson := range substutution.SubstitutionLessons {
					stmtIns, err := db.Prepare("insert into substitutions(class, teacher, absent_teacher, subject, classroom, lesson, note, date) values (?, ?, ?, ?, ?, ?, ?, ?);")
					check(err)
					defer stmtIns.Close()

					_, err = stmtIns.Exec(
						parseSubstitutionsClass(substututionLesson.Class),
						substTeacherToSchTeacher(substututionLesson.Teacher),
						substTeacherToSchTeacher(substutution.AbsentTeacher),
						substututionLesson.Subject,
						substututionLesson.Classroom,
						strconv.Itoa(substututionLesson.Lesson()),
						substututionLesson.Note,
						data.DateStr,
					)
					check(err)
				}
			}

			//parsing subject exchange
			for _, exchange := range data.SubjectExchanges {
				stmtIns, err := db.Prepare("insert into substitutions(class, teacher, subject, classroom, lesson, note, date) values (?, ?, ?, ?, ?, ?, ?);")
				check(err)
				defer stmtIns.Close()
				_, err = stmtIns.Exec(
					parseSubstitutionsClass(exchange.Class),
					substTeacherToSchTeacher(exchange.Teacher),
					exchange.Subject,
					exchange.Classroom,
					strconv.Itoa(exchange.Lesson()),
					exchange.Note,
					data.DateStr,
				)
				check(err)

			}

			//parsing lesson exchange
			for _, exchange := range data.LessonExchanges {
				stmtIns, err := db.Prepare("insert into substitutions(class, teacher, absent_teacher, subject, classroom, lesson, note, date) values (?, ?, ?, ?, ?, ?, ?, ?);")
				check(err)
				defer stmtIns.Close()
				_, err = stmtIns.Exec(
					parseSubstitutionsClass(exchange.Class),
					substTeacherToSchTeacher(exchange.Teachers()[1]),
					substTeacherToSchTeacher(exchange.Teachers()[0]),
					exchange.Subject(),
					exchange.Classroom,
					strconv.Itoa(exchange.Lesson()),
					exchange.Note,
					data.DateStr,
				)
				check(err)
			}

			//parsing subject exchange
			for _, exchange := range data.ClassroomExchanges {
				stmtIns, err := db.Prepare("insert into substitutions(class, teacher, subject, classroom, lesson, note, date) values (?, ?, ?, ?, ?, ?, ?);")
				check(err)
				defer stmtIns.Close()
				_, err = stmtIns.Exec(
					parseSubstitutionsClass(exchange.Class),
					substTeacherToSchTeacher(exchange.Teacher),
					exchange.Subject,
					exchange.Classroom,
					strconv.Itoa(exchange.Lesson()),
					exchange.Note,
					data.DateStr,
				)
				check(err)
			}
		}

		//add 1 day
		currentDate = currentDate.Add(oneDay)
	}
}

func parseSubstitutionsClass(original string) string {
	if len(original) > 4 && strings.Contains(original, "-") {
		original = original[:strings.Index(original, "-") - 1]
	}
	original = strings.Replace(original, " ", "", -1)
	original = strings.Replace(original, ".", "", -1)
	return strings.ToUpper(original)
}

func getSubstitutionsForDate(date time.Time) SubstitutionsStruct {
	nonsense := randStr(32)
	dateStr := strconv.Itoa(date.Year()) + "-" + strconv.Itoa(int(date.Month())) + "-" + strconv.Itoa(date.Day())
	params := "func=gateway&call=suplence&datum=" + dateStr + "&nonsense=" + nonsense
	signature_string := "solsis.gimvic.org" + "||" + params + "||" + api_key
	signature := hash(signature_string)
	url := "https://solsis.gimvic.org/?" + params + "&signature=" + signature
	jsonStr := getTextFromUrl(url)

	data := SubstitutionsStruct{}
	err := json.Unmarshal([]byte(jsonStr), &data)
	check(err)

	jsonHash := hash(jsonStr)
	if isNew(data.DateStr, jsonHash) {
		//update hash
		db, err := sql.Open("mysql", sqlString)
		check(err)
		_, err = db.Exec("REPLACE into hash (hash, source) values('" + jsonHash + "', '" + data.DateStr + "')")
		check(err)
		return data
	}
	return SubstitutionsStruct{}
}

func updateSchedule() {
	//text gets downloaded and splitet into relevant parts
	all := getTextFromUrl("https://dl.dropboxusercontent.com/u/16258361/urnik/data.js")
	allHash := hash(all)
	if isNew("schedule", allHash) {
		scheduleDataStr := all[strings.Index(all, "podatki[0][0]") : strings.Index(all, "razredi") - 1]
		classesDataStr := all[strings.Index(all, "razredi") : strings.Index(all, "ucitelji") - 1]
		teachersDataStr := all[strings.Index(all, "ucitelji") : strings.Index(all, "ucilnice") - 1]

		//schedule data parsing
		scheduleSections := strings.Split(scheduleDataStr, ";")
		db, err := sql.Open("mysql", sqlString)
		check(err)
		_, err = db.Exec("truncate table schedule;")
		check(err)

		for _, section := range scheduleSections {
			lines := strings.Split(section, "\n")
			lines = clearUselessScheduleLines(lines)
			class := extractValueFromLine(lines[1], true)
			teacher := extractValueFromLine(lines[2], true)
			subject := extractValueFromLine(lines[3], true)
			classroom := extractValueFromLine(lines[4], true)
			dayStr := extractValueFromLine(lines[5], false)
			lessonStr := extractValueFromLine(lines[6], false)
			day, err := strconv.Atoi(dayStr)
			check(err)
			lesson, err := strconv.Atoi(lessonStr)
			check(err)

			stmtIns, err := db.Prepare("insert into schedule(class, teacher, subject, classroom, day, lesson) values (?, ?, ?, ?, ?, ?);")
			check(err)
			defer stmtIns.Close()
			_, err = stmtIns.Exec(
				class,
				teacher,
				subject,
				classroom,
				strconv.Itoa(day),
				strconv.Itoa(lesson),
			)
			check(err)

		}

		//classes parsing
		lines := strings.Split(classesDataStr, "\n")[1:]
		_, err = db.Exec("truncate table classes;")
		check(err)
		for _, line := range lines {
			class := extractValueFromLine(line, true)
			main := "0"
			if len(class) == 2 {
				main = "1"
			}

			stmtIns, err := db.Prepare("insert into classes(class, main) values (?, ?);")
			check(err)
			defer stmtIns.Close()
			_, err = stmtIns.Exec(class, main)
			check(err)
		}

		//teachers parsing
		lines = strings.Split(teachersDataStr, "\n")[1:]
		_, err = db.Exec("truncate table teachers;")
		check(err)
		for _, line := range lines {
			teacher := extractValueFromLine(line, true)

			stmtIns, err := db.Prepare("insert into teachers(teacher) values (?);")
			check(err)
			defer stmtIns.Close()
			_, err = stmtIns.Exec(teacher)
			check(err)
		}

		//update schedule hash
		_, err = db.Exec("REPLACE into hash (hash, source) values('" + allHash + "', 'schedule');")
		check(err)
		db.Close()
	}
}

func parseMenu(args []string) {
	//check for file argument
	if len(os.Args) < 3 {
		fmt.Println("Specify the file!")
	} else {
		fileArg := os.Args[2]

		if isMenuValid(fileArg) {
			csvfile, err := os.Open(fileArg)
			defer csvfile.Close()
			check(err)

			reader := csv.NewReader(csvfile)
			reader.Comma = ';'
			reader.FieldsPerRecord = -1
			rawCSVdata, err := reader.ReadAll()
			check(err)

			secNumbers, Snack := getSectionNumbers(rawCSVdata)

			if Snack {
				processSnack(rawCSVdata, secNumbers)
			} else {
				processLunch(rawCSVdata, secNumbers)
			}

		} else {
			panic("Invalid menu!")
		}
	}
}

func isNew(source, hash string) bool {
	//debug
	return true

	con, err := sql.Open("mysql", sqlString)
	check(err)
	defer con.Close()

	rows, err := con.Query("select hash from hash where source='" + source + "';")
	check(err)
	var temp string
	rows.Next()
	rows.Scan(&temp)
	if temp == hash {
		return false
	}
	return true
}

func clearUselessScheduleLines(lines []string) []string {
	start := 0
	stop := len(lines)
	if !strings.HasPrefix(lines[0], "podatki") {
		start = 1
	}
	if strings.Contains(lines[len(lines) - 1], "new Array(") {
		stop--
	}
	return lines[start:stop]
}

func extractValueFromLine(line string, quoted bool) string {
	if quoted {
		return line[strings.Index(line, "\"") + 1 : strings.LastIndex(line, "\"")]
	} else {
		return line[strings.LastIndex(line, " ") + 1 : len(line) - 1]
	}
}

func substTeacherToSchTeacher(substTeacher string) string {
	//fill schTeachers array from mysql
	if len(schTeachers) == 0 {
		con, err := sql.Open("mysql", sqlString)
		check(err)
		defer con.Close()
		rows, err := con.Query("select teacher from teachers;")
		check(err)

		for rows.Next() {
			var temp string
			rows.Scan(&temp)
			schTeachers = append(schTeachers, temp)
		}
	}

	for _, schTeacher := range schTeachers {
		if areTeachersSame(substTeacher, schTeacher) {
			return schTeacher
		}
	}
	return substTeacher
}

func areTeachersSame(substitutions, schedule string) bool {
	substitutions = strings.ToLower(substitutions)
	schedule = strings.ToLower(schedule)

	//for special case Saračevć
	if (strings.Contains(substitutions, "saračevič") || strings.Contains(substitutions, "saračević")) && (strings.Contains(schedule, "saračevič") || strings.Contains(schedule, "saračević")) {
		return true
	}

	substArr := strings.Split(substitutions, " ")
	schArr := strings.Split(schedule, " ")

	if len(substArr) == 2 {
		return compare2Teachers(substArr, schArr)
	} else {
		return compare3Teachers(substitutions, schedule)
	}
}

func compare2Teachers(substitutions, schedule []string) bool {
	//if any is same
	for _, temp := range substitutions {
		if temp == schedule[0] {
			return true
		}
	}

	temp := ""

	//if the first one is surname
	substring := substitutions[1][:1]
	temp = substring + substitutions[0]
	if temp == schedule[0] {
		return true
	}

	//and vice versa
	temp = substitutions[0] + substring
	if temp == schedule[0] {
		return true
	}

	//if the second one is surname
	substring = substitutions[0][:1]
	temp = substitutions[1] + substring
	if temp == schedule[0] {
		return true
	}

	//and vice versa
	temp = substring + substitutions[1]
	if temp == schedule[0] {
		return true
	}

	//and for Bajec and other special cases using spaces
	//if the first one is surname
	substring = substitutions[1][:1]
	temp = substring + " " + substitutions[0]
	if temp == schedule[0] {
		return true
	}

	//and vice versa
	temp = substitutions[0] + " " + substring
	if temp == schedule[0] {
		return true
	}

	//if the second one is surname
	substring = substitutions[0][:1]
	temp = substitutions[1] + " " + substring
	if temp == schedule[0] {
		return true
	}

	//and vice versa
	temp = substring + " " + substitutions[1]
	if temp == schedule[0] {
		return true
	}

	return false
}

func compare3Teachers(substitutions, schedule string) bool {
	return strings.Contains(substitutions, schedule)
}

func getTextFromUrl(url string) string {
	response, err := http.Get(url)
	check(err)
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	check(err)
	return string(contents)
}

func hash(str string) string {
	//convert string to byte slice
	converted := []byte(str)

	//hash the byte slice and return the resulting string
	hasher := sha256.New()
	hasher.Write(converted)
	return (hex.EncodeToString(hasher.Sum(nil))) //changed to hex and removed URLEncoding
}

var randSrc = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1 << letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func randStr(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n - 1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

//provides table sections and type (Snack = true, Lunch = false)
func getSectionNumbers(csv [][]string) ([]int, bool) {
	var result []int
	var Snack bool

	for i, line := range csv {
		if strings.Contains(strings.ToLower(line[1]), "navadna") || strings.Contains(strings.ToLower(line[1]), "kosilo") {
			result = append(result, i)
			if strings.Contains(line[1], "navadna") {
				Snack = true
			}
		}
	}

	return result, Snack
}

func processSnack(table [][]string, selNumbers []int) {

	for i, num := range selNumbers {
		//to make sure not to parse more than 5 days
		if i > 4 {
			break;
		}

		if i + 1 == len(selNumbers) {
			processSnackSel(table[num:len(table)])
		} else {
			processSnackSel(table[num:selNumbers[i + 1]])
		}
	}
}

func processSnackSel(sel [][]string) {
	date := findDate(sel)

	var normal, veg, veg_per, sadnozel string
	for i := 1; i < len(sel); i++ {
		if sel[i][1] != "" {
			if normal != "" {
				normal += ";"
			}
			normal += sel[i][1]
		}
		if sel[i][2] != "" {
			if veg_per != "" {
				veg_per += ";"
			}
			veg_per += sel[i][2]
		}
		if sel[i][3] != "" {
			if veg != "" {
				veg += ";"
			}
			veg += sel[i][3]
		}
		if sel[i][4] != "" {
			if sadnozel != "" {
				sadnozel += ";"
			}
			sadnozel += sel[i][4]
		}
	}

	con, err := sql.Open("mysql", sqlString)
	check(err)
	defer con.Close()

	_, err = con.Exec("insert into snack (date, normal,  veg_per, veg, sadnozel) values (?, ?, ?, ?, ?)", date, normal, veg_per, veg, sadnozel)
	check(err)

}

func processLunchSel(sel [][]string) {

	date := findDate(sel)

	var normal, veg string
	for i := 1; i < len(sel); i++ {
		if sel[i][1] != "" {
			if normal != "" {
				normal += ";"
			}
			normal += sel[i][1]
		}
		if sel[i][2] != "" {
			if veg != "" {
				veg += ";"
			}
			veg += sel[i][2]
		}
	}

	con, err := sql.Open("mysql", sqlString)
	check(err)
	defer con.Close()

	_, err = con.Exec("insert into lunch (date, normal, veg) values (?, ?, ?)", date, normal, veg)
	check(err)

}

func processLunch(table [][]string, selNumbers []int) {
	for i, num := range selNumbers {
		if i + 1 == len(selNumbers) {
			//to make sure not to parse more than 5 days
			if i > 4 {
				break;
			}
			processLunchSel(table[num:len(table)])
		} else {
			processLunchSel(table[num:selNumbers[i + 1]])
		}
	}
}

func findDate(sel [][]string) time.Time {
	var index int = 0
	for i, line := range sel {
		cell := strings.ToLower(line[0])
		if strings.Contains(cell, "pon") || strings.Contains(cell, "tor") || strings.Contains(cell, "sre") || strings.Contains(cell, "rtek") || strings.Contains(cell, "pet") {
			index = i + 1
		}
	}

	date, err := time.Parse("2.1.2006", sel[index][0])
	check(err)
	return date
}

func isMenuValid(fileName string) bool {
	csv, err := ioutil.ReadFile(fileName)
	check(err)

	//if strings.Count(string(csv), ";") == 240 && (strings.Contains(strings.ToLower(string(csv)), "navadna") || strings.Contains(strings.ToLower(string(csv)), "kosilo")) {
	if (strings.Contains(strings.ToLower(string(csv)), "navadna") || strings.Contains(strings.ToLower(string(csv)), "kosilo")) {
		return true
	}
	return false
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type SubstitutionsStruct struct {
	Substitutions      []Substitution      `json:"nadomescanja"`
	SubjectExchanges   []SubjectExchange   `json:"menjava_predmeta"`
	LessonExchanges    []LessonExchange    `json:"menjava_ur"`
	ClassroomExchanges []ClassroomExchange `json:"menjava_ucilnic"`
	DateStr            string              `json:"datum"`
}

type Substitution struct {
	AbsentTeacher       string               `json:"odsoten_fullname"`
	SubstitutionLessons []SubstitutionLesson `json:"nadomescanja_ure"`
}

type SubstitutionLesson struct {
	LessonStr string `json:"ura"`
	Classroom string `json:"ucilnica"`
	Class     string `json:"class_name"`
	Teacher   string `json:"nadomesca_full_name"`
	Subject   string `json:"predmet"`
	Note      string `json:"opomba"`
}

type SubjectExchange struct {
	LessonStr string `json:"ura"`
	Classroom string `json:"ucilnica"`
	Class     string `json:"class_name"`
	Teacher   string `json:"ucitelj"`
	Subject   string `json:"predmet"`
	Note      string `json:"opomba"`
}

type LessonExchange struct {
	Class           string `json:"class_name"`
	LessonStr       string `json:"ura"`
	TeacherExchange string `json:"zamenjava_uciteljev"`
	SubjectExchange string `json:"predmet"`
	Classroom       string `json:"ucilnica"`
	Note            string `json:"opomba"`
}

type ClassroomExchange struct {
	LessonStr string `json:"ura"`
	Classroom string `json:"ucilnica_to"`
	Class     string `json:"class_name"`
	Teacher   string `json:"ucitelj"`
	Subject   string `json:"predmet"`
	Note      string `json:"opomba"`
}

func (s SubstitutionLesson) Lesson() int {
	result, err := strconv.Atoi(s.LessonStr[:len(s.LessonStr) - 1])
	check(err)
	return result
}
func (s SubjectExchange) Lesson() int {
	result, err := strconv.Atoi(s.LessonStr[:len(s.LessonStr) - 1])
	check(err)
	return result
}
func (s LessonExchange) Lesson() int {
	result, err := strconv.Atoi(s.LessonStr[:len(s.LessonStr) - 1])
	check(err)
	return result
}
func (s ClassroomExchange) Lesson() int {
	result, err := strconv.Atoi(s.LessonStr[:len(s.LessonStr) - 1])
	check(err)
	return result
}

func (s LessonExchange) Teachers() []string {
	result := strings.Split(s.TeacherExchange, " -> ")
	if len(result) == 1 {
		result = append(result, result[0])
	}
	return result
}

func (s LessonExchange) Subject() string {
	return s.SubjectExchange[strings.LastIndex(s.SubjectExchange, "-> ") + 3:]
}
