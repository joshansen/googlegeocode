// Package googlegeocode is used to make queries to the Google Geocode API.
//
// Initilizing the package will create one file called .geocoder-data on your machine that stores your api key and information necessary to ensure that api limits arean't exceed. It will prompt you to enter your google geocode api key for later use. If you enter an invalid api key or need to change it at a future date, delete the file named .geocoder-data and you will be prompted to enter the information again.
package googlegeocode

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	geocodeURL = "https://maps.googleapis.com/maps/api/geocode/json?address="
)

type geocoder struct {
	mutex                sync.Mutex
	apiKey               string
	lastQuery            time.Time
	queryLimitReached    bool
	queryLimitExpiration time.Time
	fileStore            *os.File
}

// Results returned by the Google Geocode API.
type Results struct {
	Results []struct {
		AddressComponents []struct {
			LongName  string   `json:"long_name"`
			ShortName string   `json:"short_name"`
			Types     []string `json:"types"`
		} `json:"address_components"`
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
			LoctionType string `json:"location_type"`
			Viewport    struct {
				Northeast struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"northeast"`
				Southwest struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"southwest"`
			} `json:"viewport"`
		} `json:"geometry"`
		PlaceID string   `json:"place_id"`
		Types   []string `json:"types"`
	} `json:"results"`
	Status string `json:"status"`
}

var geocoderInstance geocoder

// Initiailize geocoderInstance with data from the file .geocoder-data.
func init() {
	var err error
	geocoderInstance, err = newGeocoderFromFile(".geocoder-data")
	if err != nil {
		panic(err)
	}
}

// GetResults executes a query to the Google Geocode API
func GetResults(address string) (results Results, err error) {
	// Store geocoderInstance before the function returns
	defer geocoderInstance.store()

	// Lock the mutex so GetResults can't be called concurrently, preventing api limit issues.
	geocoderInstance.mutex.Lock()
	defer geocoderInstance.mutex.Unlock()

	// If the query limit has been reached, return an error
	if geocoderInstance.queryLimitReached {
		// Check if the query limit has
		if time.Now().Before(geocoderInstance.queryLimitExpiration) {
			return Results{}, fmt.Errorf("the maximum daily queries have been exceeded, and the limit will be reset at midnight pacific time")
		}
		// If the query limit has expired, set queryLimitReached to false
		geocoderInstance.queryLimitReached = false
	}

	// Requests cannot be made faster than every 20 miliseconds. The function will sleep so the rate limit isn't exceeded.
	time.Sleep(20*time.Millisecond - time.Since(geocoderInstance.lastQuery))

	// Send the request to google.
	resp, err := http.Get(geocodeURL + url.QueryEscape(address) + "&key=" + geocoderInstance.apiKey)
	if err != nil {
		return Results{}, fmt.Errorf("Error geocoding address: <%v>", err)
	}
	defer resp.Body.Close()

	// Record the time now as the last query time.
	geocoderInstance.lastQuery = time.Now()

	// Read the response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Results{}, fmt.Errorf("Error decoding geocoder result: <%v>", err)
	}

	// Unmarshall the JSON values into results.
	err = json.Unmarshal(body, &results)
	if err != nil {
		return Results{}, fmt.Errorf("Error unmarshaling geocoder result: <%v>", err)
	}

	// The query limit can be exceeded either because of the request rate or the total number of requests. Because we are limiting the request rate, only the total number of requests will trigger this error.
	if results.Status == "OVER_QUERY_LIMIT" {
		// Set queryLimitReached to true
		geocoderInstance.queryLimitReached = true

		// Find the pacific timezone name to determine if it's PST or PDT
		pacificTimezone, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			panic(fmt.Sprintf("Could not find the timezone for 'America/Los_Angeles' which is needed to set limit expiration (%v)", err))
		}

		// Use format and parse to remove hour information from the current time in order to get the begining of the day.
		beginingOfDayPacific, err := time.ParseInLocation("Jan 2 2006", time.Now().In(pacificTimezone).Format("Jan 2 2006"), pacificTimezone)
		if err != nil {
			panic(fmt.Sprintf("Could not parse date when calculateding query (%v)", err))
		}
		// Add one day to that time to get midnight Pacific time, the time when the the query limit will expire.
		geocoderInstance.queryLimitExpiration = beginingOfDayPacific.Add(time.Hour * 24)

		return Results{}, fmt.Errorf("the maximum daily queries have been exceeded, and the limit will be reset at midnight pacific time")
	}

	return results, nil
}

// NewGeocoderFromFile initializes a geocoder from the data present in the file with filename.
func newGeocoderFromFile(filename string) (geocoderInstance geocoder, err error) {
	// Read file
	content, err := ioutil.ReadFile(filename)
	// If the file doesn't exist, proceed. We can't return until after we try to create the file though
	if err != nil && !os.IsNotExist(err) {
		return geocoder{}, err
	}

	// Open file so that it can be used for writing
	geocoderInstance.fileStore, err = os.Create(filename)
	if err != nil {
		return geocoder{}, err
	}

	// Split content
	splitConent := strings.Split(string(content), "\n")
	splitContentLength := len(splitConent)

	// Set the values that were supplied in the file. The file may not have all of the required lines, so we structure these statements as follows.
	switch {
	case splitContentLength > 3:
		geocoderInstance.queryLimitExpiration, err = time.Parse(time.RFC3339Nano, splitConent[3])
		fallthrough
	case splitContentLength == 3:
		geocoderInstance.queryLimitReached, err = strconv.ParseBool(splitConent[2])
		fallthrough
	case splitContentLength == 2:
		geocoderInstance.lastQuery, err = time.Parse(time.RFC3339Nano, splitConent[1])
		fallthrough
	case splitContentLength == 1:
		geocoderInstance.apiKey = splitConent[0]
	}

	// If an API key wasn't supplied in the file, ask the user to input one.
	for geocoderInstance.apiKey == "" {
		fmt.Print("Enter your API key for the Google Geocoding API: ")
		_, err = fmt.Scanln(&geocoderInstance.apiKey)
		if err != nil {
			return geocoder{}, fmt.Errorf("An error occured reading response from the prompt 'Enter your API key for the Google Geocoding API:' (%v)", err)
		}
	}

	return geocoderInstance, nil
}

// This method stringifys geocoder in a readable format that can be stored in a file.
func (g geocoder) string() string {
	outputLines := []string{
		g.apiKey,
		g.lastQuery.Format(time.RFC3339Nano),
		strconv.FormatBool(g.queryLimitReached),
		g.queryLimitExpiration.Format(time.RFC3339Nano),
	}

	return strings.Join(outputLines, "\n")
}

// This method stores the data in geocoder into a file on the local machine for future use.
func (g geocoder) store() {
	var err error

	// Seek to the begining of fileStore before writing.
	_, err = g.fileStore.Seek(0, 0)
	if err != nil {
		panic(fmt.Sprintf("Error writing buffered geocode information to file (%v)", err))
	}

	// Write geocoder data to fileStore.
	_, err = g.fileStore.Write([]byte(g.string()))
	if err != nil {
		panic(fmt.Sprintf("Error writing geocode information to file (%v)", err))
	}
}
