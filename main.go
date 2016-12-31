package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"io/ioutil"
	"encoding/xml"
	"fmt"
	"time"
	//"github.com/muesli/regommend"
)

var sqlDb *sql.DB
const baseUrlApi2 = "https://www.boardgamegeek.com/xmlapi2"
const baseUrlApi1 = "https://www.boardgamegeek.com/xmlapi"
var exploredUsers map[int]bool
var collectionInsertStmt *sql.Stmt
var gameMetaInsertStmt *sql.Stmt
var currentForumListId int

func main() {
	openDb()
	setupDb()
	//fetch()
	currentForumListId = getCurrentForumList()
	for {
		exploredUsers = make(map[int]bool)
		getUsersFromForumList(currentForumListId)
		currentForumListId++
		updateCurrentForumList(currentForumListId)
	}

	//closeDb()
}

func updateCurrentForumList(forumId int) {
	_, err := sqlDb.Exec("REPLACE INTO current_forumlist(id, forumId) VALUES(?,?)", 1, forumId)
	if err != nil {
		log.Fatal(err)
	}
}

func getCurrentForumList() int {
	ret := 0;
	var (
		forumId int
	)
	rows, err := sqlDb.Query("select forumId from current_forumlist where id = 1")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&forumId)
		if err != nil {
			log.Fatal(err)
		}
		ret = forumId
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return ret
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
		fmt.Println("error unmarshalling xml, aborting")
		return
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
		fmt.Println("error unmarshalling xml, aborting")
	}
	if forum.Id < 1 {
		// reset forum list
		currentForumListId = 0
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
		fmt.Println("error unmarshalling xml, aborting")
		return
	}
	for _,article := range thread.Articles.Articles {
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
		statusCode := response.StatusCode
		if statusCode == 200 {
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				retryGetXml(err, "error reading response - waiting for retry", url, processor, 30)
			} else {
				processor(body)
			}
		} else if statusCode == 202 {
			retryGetXml(err, "received 202 - waiting for retry", url, processor, 5)
		} else if statusCode == 400 {
			fmt.Println("received error 400 - aborting")
		} else {
			retryGetXml(err, fmt.Sprintf("server error %d - waiting for retry", statusCode), url, processor, 30)
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
	file, err := ioutil.ReadFile("/root/work/mysqlpw.txt")
	userPw := "root"
	if err == nil {
		userPw += ":" + string(file)
	}
	db, err := sql.Open("mysql",
		userPw + "@tcp(127.0.0.1:3306)/hello")
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
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS current_forumlist(id INT NOT NULL PRIMARY KEY, forumId INT NOT NULL);")
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

type GameRating struct {
	Rating int
}

//func recommend() {
//	// Accessing a new regommend table for the first time will create it.
//	games := regommend.Table("games")
//
//	games1 := make(map[interface{}]GameRating)
//	games1["1984"] = GameRating(1)
//	games1["Robinson Crusoe"] = GameRating(2)
//	games1["Moby-Dick"] = GameRating(3)
//	games.Add("Chris", games1)
//
//	booksJayRead := make(map[interface{}]float64)
//	booksJayRead["1984"] = 5.0
//	booksJayRead["Robinson Crusoe"] = 4.0
//	booksJayRead["Gulliver's Travels"] = 4.5
//	books.Add("Jay", booksJayRead)
//
//	recs, _ := books.Recommend("Chris")
//	for _, rec := range recs {
//		fmt.Println("Recommending", rec.Key, "with score", rec.Distance)
//	}
//
//	neighbors, _ := books.Neighbors("Chris")
//}
