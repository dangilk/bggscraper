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
	"os"
	"bufio"
	"strconv"
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

	arg := os.Args[1]
	if arg == "scraper" {
		log.Println("starting scraper")
		startScraperService()
	} else if arg == "service" {
		log.Println("starting query service")
		startQueryService()
	} else {
		log.Println("no commands found, shutting down")
	}


	//closeDb()
}

// SERVICE SECTION
func topSuggestions(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()  // parse arguments, you have to call this by yourself
	userId := r.FormValue("userId")
	count, _ := strconv.Atoi(r.FormValue("count"))
	fetchTopSuggestions(userId, count)
	fmt.Fprintf(w, "Hello dan!") // send data to client side
}

func startQueryService() {
	http.HandleFunc("/topSuggestions", topSuggestions) // set router
	err := http.ListenAndServe(":9090", nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// SCRAPER SECTION

func startScraperService() {
	currentForumListId = getCurrentForumList()
	for {
		logToFile(false, "start scraper iteration for forumList: " + strconv.Itoa(currentForumListId))
		exploredUsers = make(map[int]bool)
		getUsersFromForumList(currentForumListId)
		currentForumListId++
		updateCurrentForumList(currentForumListId)
	}
}

func updateCurrentForumList(forumId int) {
	_, err := sqlDb.Exec("REPLACE INTO current_forumlist(id, forumId) VALUES(?,?)", 1, forumId)
	if err != nil {
		logToFile(true, err)
	}
}

func getCurrentForumList() int {
	ret := 0;
	var (
		forumId int
	)
	rows, err := sqlDb.Query("select forumId from current_forumlist where id = 1")
	if err != nil {
		logToFile(true, err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&forumId)
		if err != nil {
			logToFile(true, err)
		}
		ret = forumId
	}
	err = rows.Err()
	if err != nil {
		logToFile(true, err)
	}
	return ret
}

// get a forumlist, then get its forums, then get its threads, then get its articles, and finally get users from articles
func getUsersFromForumList(forumId int) {
	logToFile(false, "processing forumlist with id: " + strconv.Itoa(forumId))
	forumListUrl := fmt.Sprintf(baseUrlApi2 + "/forumlist?id=%d&type=thing", forumId)
	getXml(forumListUrl, processForumList)
}

func processForumList(bytes []byte) {
	var forumList ForumList
	err := xml.Unmarshal(bytes, &forumList)
	if err != nil {
		logToFile(false, "error unmarshalling forumlist xml, aborting")
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
		logToFile(false, "error unmarshalling forum xml, aborting")
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
		logToFile(false, "error unmarshalling thread xml, aborting")
		return
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
		logToFile(false, "error unmarshalling user xml, aborting")
		return
	}
	if len(user.Name) < 1 {
		logToFile(false, "skipping empty user")
		return
	}
	if _, exists := exploredUsers[user.Id]; exists {
		return
	} else {
		exploredUsers[user.Id] = true
	}

	// get the users collection
	logToFile(false, "process user: " + user.Name)
	collectionUrl := fmt.Sprintf(baseUrlApi1 + "/collection/%s", user.Name)
	getXml(collectionUrl, createCollectionProcessor(user))


	// explore user friends
	for _,buddy := range user.Buddies.Buddies {
		logToFile(false, "get buddies xml")
		buddyUrl := fmt.Sprintf(baseUrlApi2 + "/user?name=%s", buddy.Name)
		getXml(buddyUrl, processUser)
	}
}

func createCollectionProcessor(user User) XmlProcessor {
	return func (bytes []byte) {
		var collectionItems CollectionItems
		err := xml.Unmarshal(bytes, &collectionItems)
		if err != nil {
			logToFile(false, "error unmarshalling collection xml, aborting")
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
			logToFile(false, "received error 400 - aborting")
		} else {
			retryGetXml(err, fmt.Sprintf("server error %d - waiting for retry", statusCode), url, processor, 30)
		}
	}
}

func retryGetXml(err error, retryMsg string, url string, processor XmlProcessor, sleepSeconds int) {
	if err != nil {
		logToFile(false, err, retryMsg)
	}
	time.Sleep(time.Duration(sleepSeconds) * time.Second)
	getXml(url, processor)
}
func openDb() {
	userPw := "root"
	file, err := os.Open("/root/work/mysqlpw.txt")
	if err == nil {
		// Create a new Scanner for the file.
		scanner := bufio.NewScanner(file)
		// Loop over all lines in the file and print them.
		for scanner.Scan() {
			line := scanner.Text()
			userPw += ":" + line
		}
	}
	open := userPw + "@tcp(127.0.0.1:3306)/hello"
	println(open)


	//file, err := ioutil.ReadFile("/root/work/mysqlpw.txt")
	//userPw := "root"
	//if err == nil {
	//	userPw += ":" + string(file)
	//}
	db, err := sql.Open("mysql", open)
	if err != nil {
		logToFile(true, err)
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
		logToFile(false, err)
		return
	}
	// update metadata
	_, err = gameMetaInsertStmt.Exec(collection.ObjectId, collection.Name, collection.YearPublished, collection.SubType,
		collection.Stats.MinPlayers, collection.Stats.MaxPlayers,
		collection.Stats.MinPlaytime, collection.Stats.MaxPlaytime, collection.Stats.PlayingTime, collection.Stats.NumOwned,
		collection.Stats.Rating.UsersRated.Value, collection.Stats.Rating.AverageRating.Value,
		collection.Stats.Rating.BayesAverageRating.Value, collection.Stats.Rating.StdDevRating.Value, collection.Stats.Rating.MedianRating.Value)
	if err != nil {
		logToFile(false, err)
		return
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
		logToFile(true, err)
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS current_forumlist(id INT NOT NULL PRIMARY KEY, forumId INT NOT NULL);")
	if err != nil {
		logToFile(true, err)
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
		logToFile(true, err)
	}
	collectionInsertStmt, err = sqlDb.Prepare("REPLACE INTO user_collections(id, userId, userName, gameName," +
		"numPlays, own, prevOwned, forTrade, want, wantToPlay, wantToBuy, " +
		"wishList, wishListPriority, preOrdered, lastModified, " +
		"userRating) " +
		"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		logToFile(true, err)
	}
	gameMetaInsertStmt, err = sqlDb.Prepare("REPLACE INTO game_metadata(id, gameName, yearPublished," +
		"subType, " +
		"minPlayers, maxPlayers, minPlaytime, maxPlaytime," +
		"playingTime, numOwned, ratingCount, averageRating, bayesAverageRating, stdDevRating, medianRating) " +
		"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		logToFile(true, err)
	}
}

func fetchTopSuggestions(userId string, limit int) {
	log.Println("fetch top suggestions for " + userId)
	var (
		gameName string
		userRating float64
	)
	stmt, err := sqlDb.Prepare("select gameName, userRating from user_collections where userId = ? order by userRating desc limit ?")
	if err != nil {
		//log.Fatal(err)
		logToFile(true, err)
	}
	defer stmt.Close()
	rows, err := stmt.Query(userId, limit)
	if err != nil {
		//log.Fatal(err)
		logToFile(true, err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&gameName, &userRating)
		if err != nil {
			//log.Fatal(err)
			logToFile(true, err)
		}
		log.Println(gameName, userRating)
	}
	err = rows.Err()
	if err != nil {
		//log.Fatal(err)
		logToFile(true, err)
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

func logToFile(isFatal bool, s ...interface{}) {
	log.SetOutput(os.Stdout)
	log.Println(s)
	currentDate := time.Now().UTC()
	dateString := fmt.Sprintf("%d-%02d-%02d", currentDate.Year(), currentDate.Month(), currentDate.Day())

	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", os.FileMode(0777))
	}
	f, err := os.OpenFile("logs/log-"+dateString + ".txt", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
	if isFatal {
		log.Fatal("FATAL: ", s)
	} else {
		log.Println(s)
	}
}