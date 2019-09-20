package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	hn "github.com/hn"
)

func main() {

	tpl := template.Must(template.ParseFiles("./index.gohtml"))
	http.HandleFunc("/", handdler(300, tpl))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", 3000), nil))
}

//IsStory Check story
func IsStory(story Item) bool {
	if story.Type == "story" && story.URL != "" {
		return true
	}
	return false
}

/*
Convert hn.item to Item
*/
func ParseToItemSoory(hnitem hn.Item) Item {
	storyItem := Item{
		Item: hnitem,
		Host: "Error",
	}
	URL, err := url.Parse(storyItem.URL)

	if err == nil {
		storyItem.Host = strings.TrimPrefix(URL.Hostname(), "www.")
	}

	return storyItem
}

func handdler(numStory int, tpl *template.Template) http.HandlerFunc {
	c := StoryCache{
		numStory: numStory,
		Duration: 5 * time.Second,
	}
	/*
		After 3 second cache will update stories
	*/
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for {

			temp := StoryCache{
				numStory: numStory,
				Duration: 5 * time.Second,
			}
			temp.GetStoriesFromCache()
			c.mutex.Lock()
			c.cache = temp.cache
			c.Expiration = temp.Expiration
			c.mutex.Unlock()
			<-ticker.C
		}
	}()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()
		stories, err := c.GetStoriesFromCache()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := TemPlateData{
			Stories: stories,
			Time:    time.Now().Sub(start),
		}
		err = tpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to process the template", http.StatusInternalServerError)
			return
		}
	})
}

//GetTopStories a
func GetTopStories(numStory int) ([]Item, error) {
	var client hn.Client
	idStories, err := client.TopItems()
	if err != nil {
		return nil, errors.New("Failed to load top stories")
	}
	return GetStories(numStory, idStories)
}

type StoryCache struct {
	numStory   int
	cache      []Item
	Expiration time.Time
	Duration   time.Duration
	mutex      sync.Mutex
}

/*
	If cache doesn't have data => get data by hn_api
	If cache  have data (expiration cache < time.Now() => update data for cache)
	else reuturn data from cache
*/
func (c *StoryCache) GetStoriesFromCache() ([]Item, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.Expiration.Sub(time.Now()) > 0 {
		return c.cache, nil
	}
	stories, err := GetTopStories(c.numStory)
	if err != nil {
		return nil, err
	}
	c.cache = stories
	c.Expiration = time.Now().Add(5 * time.Second)
	return c.cache, nil

}

//GetStories Get top story
func GetStories(numStory int, idStories []int) ([]Item, error) {
	type ChanResult struct {
		idx  int
		data hn.Item
		err  error
	}

	changRes := make(chan ChanResult)
	for idx, id := range idStories {
		go func(idx int, id int) {
			var client hn.Client
			hnItem, err := client.GetItem(id)
			if err != nil {
				changRes <- ChanResult{err: err, idx: idx}
			} else {
				changRes <- ChanResult{data: hnItem, idx: idx}
			}
		}(idx, id)
		if idx == numStory {
			break
		}
	}
	var Result []ChanResult
	var ArrStories []Item
	for i := 0; i < numStory; i++ {
		Result = append(Result, <-changRes)
	}
	sort.Slice(Result, func(i, j int) bool {
		return Result[i].idx < Result[j].idx
	})
	for _, item := range Result {
		if item.err != nil {
			continue
		}
		story := ParseToItemSoory(item.data)
		if IsStory(story) {
			ArrStories = append(ArrStories, story)
		}
	}
	return ArrStories, nil
}

//Item item
type Item struct {
	hn.Item
	Host string
}

//TemPlateData TemPlateData
type TemPlateData struct {
	Stories []Item
	Time    time.Duration
}
