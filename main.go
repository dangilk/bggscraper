package main

import (
	"database/sql"
	_ "mysql"
	"log"
	"net/http"
	"io/ioutil"
	"encoding/xml"
	"fmt"
	"time"
)

var sqlDb *sql.DB
var baseUrlApi2 = "https://www.boardgamegeek.com/xmlapi2"
var baseUrlApi1 = "https://www.boardgamegeek.com/xmlapi"
var exploredUsers = make(map[int]bool)

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
	forumListUrl := fmt.Sprintf(baseUrlApi2 + "/forumlist?id=%d&type=thing", forumId)
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
		forumUrl := fmt.Sprintf(baseUrlApi2 + "/forum?id=%d", forum.Id)
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
		threadUrl := fmt.Sprintf(baseUrlApi2 + "/thread?id=%d", thread.Id)
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
		userUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", article.Author)
		getXml(userUrl, processUser)
	}
}

func processUser(bytes []byte) {
	var user User
	err := xml.Unmarshal(bytes, &user)
	if err != nil {
		log.Fatal(err)
	}
	if _, exists := exploredUsers[user.Id]; exists {
		return
	} else {
		exploredUsers[user.Id] = true
	}

	// get the users collection
	fmt.Println(user.Name)
	collectionUrl := fmt.Sprintf(baseUrlApi1 + "/collection/%s", user.Name)
	getXml(collectionUrl, createCollectionProcessor(user))


	// explore user friends
	for _,buddy := range user.Buddies.Buddies {
		buddyUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", buddy.Name)
		getXml(buddyUrl, processUser)
	}


}

func createCollectionProcessor(user User) XmlProcessor {
	return func (bytes []byte) {
		var collectionItems CollectionItems
		err := xml.Unmarshal(bytes, &collectionItems)
		if err != nil {
			log.Fatal(err)
		}
		for _,item := range collectionItems.Items {
			info := fmt.Sprintf("user %s has item %s in collection", user.Name, item.Name)
			fmt.Println(info)
		}
	}
}

type XmlProcessor func(bytes []byte)

func getXml(url string, processor XmlProcessor) {
	response, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	} else {
		defer response.Body.Close()
		if response.StatusCode == 202 {
			fmt.Println("received 202 - waiting for retry")
			time.Sleep(5 * time.Second)
			getXml(url, processor)
			return
		}
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
	_, err := sqlDb.Exec("CREATE TABLE IF NOT EXISTS users(id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, " +
		"userId INT NOT NULL, userName VARCHAR(200) NOT NULL);")
	if err != nil {
		log.Fatal(err)
	}
}

func insertUser(userId int, userName string) {
	stmt, err := sqlDb.Prepare("INSERT INTO users(userId, userName) VALUES(?)")
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
