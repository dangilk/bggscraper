package main

// collection items
type UsersRated struct {
	Value int `xml:"value,attr"`
}
type AverageRating struct {
	Value float32 `xml:"value,attr"`
}
type BayesAverageRating struct {
	Value float32 `xml:"value,attr"`
}
type StdDevRating struct {
	Value float32 `xml:"value,attr"`
}
type MedianRating struct {
	Value int `xml:"value,attr"`
}
type Rating struct {
	Value              int                `xml:"value,attr"`
	UsersRated         UsersRated         `xml:"usersrated"`
	AverageRating      AverageRating      `xml:"average"`
	BayesAverageRating BayesAverageRating `xml:"bayesaverage"`
	StdDevRating       StdDevRating       `xml:"stddev"`
	MedianRating       MedianRating       `xml:"median"`
}
type Stats struct {
	MinPlayers  int    `xml:"minplayers,attr"`
	MaxPlayers  int    `xml:"maxplayers,attr"`
	MinPlaytime int    `xml:"minplaytime,attr"`
	MaxPlaytime int    `xml:"maxplaytime,attr"`
	PlayingTime int    `xml:"playingtime,attr"`
	NumOwned    int    `xml:"numowned,attr"`
	Rating      Rating `xml:"rating"`
}
type Status struct {
	Own              int    `xml:"own,attr"`
	PrevOwned        int    `xml:"prevowned,attr"`
	ForTrade         int    `xml:"fortrade,attr"`
	Want             int    `xml:"want,attr"`
	WantToPlay       int    `xml:"wanttoplay,attr"`
	WantToBuy        int    `xml:"wanttobuy,attr"`
	WishList         int    `xml:"wishlist,attr"`
	WishListPriority int    `xml:"wishlistpriority,attr"`
	PreOrdered       int    `xml:"preordered,attr"`
	LastModified     string `xml:"lastmodified,attr"`
}
type CollectionItem struct {
	Id            int    `xml:"collid,attr"`
	ObjectId      int    `xml:"objectid,attr"`
	Name          string `xml:"name"`
	Status        Status `xml:"status"`
	Stats         Stats  `xml:"stats"`
	NumPlays      int    `xml:"numplays"`
	YearPublished int    `xml:"yearpublished"`
	Image         string `xml:"image"`
	SubType       string `xml:"subtype,attr"`
}
type CollectionItems struct {
	Items []CollectionItem `xml:"item"`
}

// forum list
type ForumList struct {
	Id     int     `xml:"id,attr"`
	Forums []Forum `xml:"forum"`
}

type Forum struct {
	Id         int     `xml:"id,attr"`
	NumThreads int     `xml:"numthreads,attr"`
	NumPosts   int     `xml:"numposts,attr"`
	Threads    Threads `xml:"threads"`
}

type Threads struct {
	Threads []Thread `xml:"thread"`
}

// Thread info
type Thread struct {
	Id          int      `xml:"id,attr"`
	NumArticles int      `xml:"numarticles,attr"`
	Articles    Articles `xml:"articles"`
}

type Articles struct {
	Articles []Article `xml:"article"`
}

type Article struct {
	Id     int    `xml:"id,attr"`
	Author string `xml:"username,attr"`
}

// User info
type User struct {
	Id      int     `xml:"id,attr"`
	Name    string  `xml:"name,attr"`
	Buddies Buddies `xml:"buddies"`
	Guilds  Guilds  `xml:"guilds"`
}

type Buddies struct {
	Buddies []Buddy `xml:"buddy"`
}

type Buddy struct {
	Id   int    `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

type Guilds struct {
	Guilds []Guild `xml:"guild"`
}

type Guild struct {
	Id   int    `xml:"id,attr"`
	Name string `xml:"name,attr"`
}
