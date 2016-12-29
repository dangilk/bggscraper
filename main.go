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
var collectionInsertStmt *sql.Stmt
var gameMetaInsertStmt *sql.Stmt

func main() {
	openDb()
	setupDb()
	fetch()


	for i := 1; i < 100; i++ {
		getUsersFromForumList(i)
	}

	fmt.Println("all done!")
	closeDb()
}

// get a forumlist, then get its forums, then get its threads, then get its articles, and finally get users from articles
func getUsersFromForumList(forumId int) {
	forumListUrl := fmt.Sprintf(baseUrlApi2 + "/forumlist?id=%d&type=thing", forumId)
	println(forumListUrl)
	getXml(forumListUrl, processForumList)
}

func processForumList(bytes []byte) {
	fmt.Println("process forum list")
	var forumList ForumList
	err := xml.Unmarshal(bytes, &forumList)
	if err != nil {
		fmt.Println("error unmarshalling xml, aborting")
		return
	}
	for _,forum := range forumList.Forums {
		fmt.Println("get forum xml")
		forumUrl := fmt.Sprintf(baseUrlApi2 + "/forum?id=%d", forum.Id)
		getXml(forumUrl, processForum)
	}
}

func processForum(bytes []byte) {
	fmt.Println("process forum")
	var forum Forum
	err := xml.Unmarshal(bytes, &forum)
	if err != nil {
		fmt.Println("error unmarshalling xml, aborting")
		return
	}
	for _,thread := range forum.Threads.Threads {
		fmt.Println("get thread xml")
		threadUrl := fmt.Sprintf(baseUrlApi2 + "/thread?id=%d", thread.Id)
		getXml(threadUrl, processThread)
	}
}

func processThread(bytes []byte) {
	fmt.Println("process thread")
	var thread Thread
	err := xml.Unmarshal(bytes, &thread)
	if err != nil {
		fmt.Println("error unmarshalling xml, aborting")
		return
	}
	for _,article := range thread.Articles.Articles {
		fmt.Println("get user xml")
		userUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", article.Author)
		getXml(userUrl, processUser)
	}
}

func processUser(bytes []byte) {
	fmt.Println("process user")
	var user User
	err := xml.Unmarshal(bytes, &user)
	if err != nil {
		fmt.Println("error unmarshalling xml, aborting")
		return
	}
	if len(user.Name) < 1 {
		fmt.Println("skipping empty user")
		return
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
		fmt.Println("get buddies xml")
		buddyUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", buddy.Name)
		getXml(buddyUrl, processUser)
	}
}

func createCollectionProcessor(user User) XmlProcessor {
	return func (bytes []byte) {
		var collectionItems CollectionItems
		err := xml.Unmarshal(bytes, &collectionItems)
		if err != nil {
			fmt.Println("error unmarshalling xml, aborting")
			return
		}
		for _,item := range collectionItems.Items {
			insertCollection(user, item)
		}
	}
}

type XmlProcessor func(bytes []byte)

func getXml(url string, processor XmlProcessor) {
	// throttle requests a little
	time.Sleep(5 * time.Second)
	response, err := http.Get(url)
	if err != nil {
		retryGetXml(err, "error getting response - waiting for retry", url, processor, 30)
		return
	} else {
		defer response.Body.Close()
		if response.StatusCode == 202 {
			retryGetXml(err, "received 202 - waiting for retry", url, processor, 5)
			return
		} else if response.StatusCode != 200 {
			retryGetXml(err, fmt.Sprintf("server error %d - waiting for retry", response.StatusCode), url, processor, 30)
			return
		}
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			retryGetXml(err, "error reading response - waiting for retry", url, processor, 30)
			return
		} else {
			processor(body)
		}
	}
}

func retryGetXml(err error, retryMsg string, url string, processor XmlProcessor, sleepSeconds int) {
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(retryMsg)
	time.Sleep(time.Duration(sleepSeconds) * time.Second)
	getXml(url, processor)
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

func insertCollection(user User, collection CollectionItem) {
	_, err := collectionInsertStmt.Exec(collection.Id, user.Id, user.Name, collection.Name, collection.NumPlays,
	collection.Status.Own, collection.Status.PrevOwned, collection.Status.ForTrade, collection.Status.Want,
	collection.Status.WantToPlay, collection.Status.WantToBuy, collection.Status.WishList, collection.Status.WishListPriority,
	collection.Status.PreOrdered, collection.Status.LastModified,
	collection.Stats.Rating.Value)
	if err != nil {
		log.Fatal(err)
	}
	// update metadata
	_, err = gameMetaInsertStmt.Exec(collection.ObjectId, collection.Name, collection.YearPublished, collection.SubType,
		collection.Stats.MinPlayers, collection.Stats.MaxPlayers,
		collection.Stats.MinPlaytime, collection.Stats.MaxPlaytime, collection.Stats.PlayingTime, collection.Stats.NumOwned,
		collection.Stats.Rating.UsersRated.Value, collection.Stats.Rating.AverageRating.Value,
		collection.Stats.Rating.BayesAverageRating.Value, collection.Stats.Rating.StdDevRating.Value, collection.Stats.Rating.MedianRating.Value)
	if err != nil {
		log.Fatal(err)
	}
}

func setupDb() {
	_, err := sqlDb.Exec("CREATE TABLE IF NOT EXISTS user_collections(" +
		"id INT NOT NULL PRIMARY KEY, " +
		"userId INT NOT NULL, " +
		"userName VARCHAR(200) NOT NULL, " +
		"gameName VARCHAR(1000) NOT NULL, " +
		"numPlays INT, " +
		"own BOOL, " +
		"prevOwned BOOL, " +
		"forTrade BOOL, " +
		"want BOOL, " +
		"wantToPlay BOOL, " +
		"wantToBuy BOOL, " +
		"wishList BOOL, " +
		"wishListPriority INT, " +
		"preOrdered BOOL, " +
		"lastModified VARCHAR(100), " +
		"userRating DOUBLE);")
	if err != nil {
		log.Fatal(err)
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS game_metadata(" +
		"id INT NOT NULL PRIMARY KEY, " +
		"gameName VARCHAR(1000) NOT NULL, " +
		"yearPublished INT, " +
		"subType VARCHAR(100), " +
		"minPlayers INT, " +
		"maxPlayers INT, " +
		"minPlaytime INT, " +
		"maxPlaytime INT, " +
		"playingTime INT, " +
		"numOwned INT, " +
		"ratingCount INT, " +
		"averageRating DOUBLE, " +
		"bayesAverageRating DOUBLE, " +
		"stdDevRating DOUBLE, " +
		"medianRating DOUBLE);")
	if err != nil {
		log.Fatal(err)
	}
	collectionInsertStmt, err = sqlDb.Prepare("REPLACE INTO user_collections(id, userId, userName, gameName," +
		"numPlays, own, prevOwned, forTrade, want, wantToPlay, wantToBuy, " +
		"wishList, wishListPriority, preOrdered, lastModified, " +
		"userRating) " +
		"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		log.Fatal(err)
	}
	gameMetaInsertStmt, err = sqlDb.Prepare("REPLACE INTO game_metadata(id, gameName, yearPublished," +
		"subType, " +
		"minPlayers, maxPlayers, minPlaytime, maxPlaytime," +
		"playingTime, numOwned, ratingCount, averageRating, bayesAverageRating, stdDevRating, medianRating) " +
		"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		log.Fatal(err)
	}
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
