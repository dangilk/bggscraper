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

func main() {
	openDb()
	setupDb()
	//insert()
	fetch()
	closeDb()

	getXml()
}

func getXml() {
	response, err := http.Get("http://boardgamegeek.com/xmlapi/collection/mkgray")
	if err != nil {
		log.Fatal(err)
	} else {
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		} else {
			var items Items
			err := xml.Unmarshal(body, &items)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(items)
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
