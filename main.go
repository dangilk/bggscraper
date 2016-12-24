package main

import (
	"database/sql"
	_ "mysql"
	"log"
	"net/http"
	"io/ioutil"
	"encoding/xml"
	"fmt"
)

var sqlDb *sql.DB
var baseUrl = "https://www.boardgamegeek.com/xmlapi2"

func main() {
	openDb()
	setupDb()
	//insert()
	fetch()
	closeDb()

	getUsersFromForumList(1)
}

// get a forumlist, then get its forums, then get its threads, then get its articles, and finally get users from articles
func getUsersFromForumList(forumId int) {
	baseForumListUrl := baseUrl + "/forumlist?id=%d&type=thing"
	forumListUrl := fmt.Sprintf(baseForumListUrl, forumId)
	println(forumListUrl)
	getXml(forumListUrl, processForumList)
}

func processForumList(bytes []byte) {
	var forumList ForumList
	err := xml.Unmarshal(bytes, &forumList)
	if err != nil {
		log.Fatal(err)
	}

	for _,forum := range forumList.Forums {
		baseForumUrl := baseUrl + "/forum?id=%d"
		forumUrl := fmt.Sprintf(baseForumUrl, forum.Id)
		getXml(forumUrl, processForum)
	}
}

func processForum(bytes []byte) {
	var forum Forum
	err := xml.Unmarshal(bytes, &forum)
	if err != nil {
		log.Fatal(err)
	}

	for _,thread := range forum.Threads.Threads {
		baseThreadUrl := baseUrl + "/thread?id=%d"
		threadUrl := fmt.Sprintf(baseThreadUrl, thread.Id)
		getXml(threadUrl, processThread)
	}
}

func processThread(bytes []byte) {
	var thread Thread
	err := xml.Unmarshal(bytes, &thread)
	if err != nil {
		log.Fatal(err)
	}

	for _,article := range thread.Articles.Articles {
		fmt.Println(article.Author)
	}
}

type XmlProcessor func(bytes []byte)

func getXml(url string, processor XmlProcessor) {
	response, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	} else {
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		} else {
			processor(body)
		}
	}
}

func openDb() {
	db, err := sql.Open("mysql",
		"root@tcp(127.0.0.1:3306)/hello")
	if err != nil {
		log.Fatal(err)
	}
	//defer db.Close()

	err = db.Ping()
	if err != nil {
		// do something here
	}
	sqlDb = db
}

func closeDb() {
	sqlDb.Close()
}

func setupDb() {
	_, err := sqlDb.Exec("CREATE TABLE IF NOT EXISTS users(id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, name VARCHAR(20));")
	if err != nil {
		log.Fatal(err)
	}
}

func insert() {
	stmt, err := sqlDb.Prepare("INSERT INTO users(name) VALUES(?)")
	if err != nil {
		log.Fatal(err)
	}
	res, err := stmt.Exec("Dolly")
	if err != nil {
		log.Fatal(err)
	}
	lastId, err := res.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	rowCnt, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("ID = %d, affected = %d\n", lastId, rowCnt)
}

func fetch() {
	var (
		id int
		name string
	)
	stmt, err := sqlDb.Prepare("select id, name from users where id = ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	rows, err := stmt.Query(1)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&id, &name)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(id, name)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
}
