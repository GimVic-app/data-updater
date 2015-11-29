package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var sqlString string = "gimvic:GimVicServer@/gimvic"
var api_key string = "ede5e730-8464-11e3-baa7-0800200c9a66"

func main() {
	updateSchedule()
}

func updateSchedule() {
	//text gets downloaded and splitet into relevant parts
	all := getTextFromUrl("https://dl.dropboxusercontent.com/u/16258361/urnik/data.js")
	allHash := hash(all)
	if isNew("schedule", allHash) {
		scheduleDataStr := all[strings.Index(all, "podatki[0][0]") : strings.Index(all, "razredi")-1]
		//classesDataStr := all[strings.Index(all, "razredi") : strings.Index(all, "ucitelji")-1]
		//teachersDataStr := all[strings.Index(all, "ucitelji") : strings.Index(all, "ucilnice")-1]

		//schedule data parsing
		scheduleSections := strings.Split(scheduleDataStr, ";")
		db, err := sql.Open("mysql", sqlString)
		check(err)
		_, err = db.Exec("truncate table schedule;")
		check(err)

		for _, section := range scheduleSections {
			lines := strings.Split(section, "\n")
			lines = clearUselessScheduleLines(lines)
			class := extractValueFromScheduleLine(lines[1], true)
			teacher := extractValueFromScheduleLine(lines[2], true)
			subject := extractValueFromScheduleLine(lines[3], true)
			classroom := extractValueFromScheduleLine(lines[4], true)
			dayStr := extractValueFromScheduleLine(lines[5], false)
			lessonStr := extractValueFromScheduleLine(lines[5], false)
			day, err := strconv.Atoi(dayStr)
			check(err)
			lesson, err := strconv.Atoi(lessonStr)
			check(err)

			_, err = db.Exec("insert into schedule(class, teacher, subject, classroom, day, lesson) values ('" + class + "', '" + teacher + "', '" + subject + "', '" + classroom + "', " + strconv.Itoa(day) + ", " + strconv.Itoa(lesson) + ");")
			check(err)
		}

		//classes parsing
		_, err = db.Exec("update hash set hash='" + allHash + "' where source='schedule';")
		check(err)
		db.Close()
	}
}

func isNew(source, hash string) bool {
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

func updateSubstitutions() {
	nonsense := randStr(32)
	params := "func=gateway&call=suplence&datum=2015-11-30&nonsense=" + nonsense
	signature_string := "solsis.gimvic.org" + "||" + params + "||" + api_key
	signature := hash(signature_string)
	url := "https://solsis.gimvic.org/?" + params + "&signature=" + signature

	fmt.Println(getTextFromUrl(url))
}

func clearUselessScheduleLines(lines []string) []string {
	start := 0
	stop := len(lines)
	if !strings.HasPrefix(lines[0], "podatki") {
		start = 1
	}
	if strings.Contains(lines[len(lines)-1], "new Array(") {
		stop--
	}
	return lines[start:stop]
}

func extractValueFromScheduleLine(line string, quoted bool) string {
	if quoted {
		return line[strings.Index(line, "\"")+1 : len(line)-2]
	} else {
		return line[strings.LastIndex(line, " ")+1 : len(line)-1]
	}
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
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func randStr(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, randSrc.Int63(), letterIdxMax; i >= 0; {
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

func check(err error) {
	if err != nil {
		panic(err)
	}
}
