package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func getConfigdir() string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	path := filepath.Join(dir, ".config", "gcal")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			log.Fatalf("Could not create config directory '%v'", path)
		}
	}
	return path
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(tokFile string, config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func getCalendarService() *calendar.Service {
	configDir := getConfigdir()
	credPath := filepath.Join(configDir, "credentials.json")
	tokenPath := filepath.Join(configDir, "token.json")

	ctx := context.Background()
	b, err := os.ReadFile(credPath)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(tokenPath, config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	return srv
}

type Event struct {
	Summary string
	AllDay  bool
	Start   time.Time
	Type    string
}

func getEvents(srv *calendar.Service, start time.Time, end time.Time) []Event {
	events := make([]Event, 0)

	t := start.Format(time.RFC3339)
	t2 := end.Format(time.RFC3339)

	items, err := srv.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(t).
		TimeMax(t2).
		OrderBy("startTime").
		Do()

	if err != nil {
		log.Fatalf("Unable to retrieve the user's events: %v", err)
	}

	for _, item := range items.Items {
		var allDay bool
		var start time.Time

		date := item.Start.DateTime

		if date == "" { // Whole day event
			allDay = true
			start, err = time.Parse("2006-01-02", item.Start.Date)
			if err != nil {
				log.Fatalf("Could not parse date '%v'", item.Start.Date)
			}
		}
		if date != "" {
			start, err = time.Parse(time.RFC3339, date)
			if err != nil {
				log.Fatalf("Could not parse datetime '%v'", date)
			}
		}

		events = append(events, Event{
			Summary: item.Summary,
			AllDay:  allDay,
			Start:   start,
			Type:    item.EventType,
		})

	}

	return events
}

func beginning_of_day(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func beginning_of_week(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day-int(t.Weekday())+1, 0, 0, 0, 0, t.Location())
}

func main() {
	// - [ ] Add a flag to show the current work week

	var ndays int
	flag.IntVarP(&ndays, "ndays", "n", 10, "Number of days to show")
	flag.Parse()

	fmt.Println()

	var start time.Time
	var end time.Time
	now := time.Now()

	if flag.Lookup("ndays").Changed {
		start = beginning_of_day(time.Now())
		end = start.AddDate(0, 0, 7)
	} else {
		start = beginning_of_week(now)
		end = start.AddDate(0, 0, 7)

	}

	srv := getCalendarService()
	events := getEvents(srv, start, end)

	if len(events) == 0 {
		fmt.Println("No upcoming events found.")
	} else {
		currentDay := start
		fmt.Printf("\n### %v\n\n", start.Format("Monday 02"))

		for _, event := range events {
			if event.Type == "focusTime" ||
				event.Type == "workingLocation" ||
				event.Type == "outOfOffice" ||
				event.Summary == "🥗 Lunch" {
				continue
			}
			if currentDay.Day() != event.Start.Day() {
				currentDay = event.Start
				fmt.Printf("\n### %v\n\n", currentDay.Format("Monday 02"))
			}
			if event.AllDay {
				fmt.Printf("- **%v**\n", event.Summary)
			} else {
				fmt.Printf("- [<] %v - %v\n", event.Start.Format("15:04"), event.Summary)
			}
		}
	}
	fmt.Println()
}
