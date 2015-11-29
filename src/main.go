package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

var sqlString string = "gimvic:GimVicServer@/gimvic"

func main() {
	parseScheduleToSql()
}

func parseScheduleToSql() {
	//text gets downloaded and splitet into relevant parts
	all := getTextFromUrl("https://dl.dropboxusercontent.com/u/16258361/urnik/data.js")
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
	db.Close()
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

func check(err error) {
	if err != nil {
		panic(err)
	}
}
