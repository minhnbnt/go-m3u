package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func ReadDomains(url string) (map[string]string, error) {

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	domainMap := make(map[string]string)
	if err := json.NewDecoder(resp.Body).Decode(&domainMap); err != nil {
		return nil, err
	}

	return domainMap, nil
}

func ReadWebsite(url string) (string, error) {

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Response returned with status %s", resp.Status)
	}

	result := string(buf)
	if !strings.HasPrefix(result, "#EXTM3U") {
		result = "#EXTM3U\n" + result
	}

	return result, nil
}

func GetRouter() *http.ServeMux {

	gistUrl := os.Getenv("GIST_URL")

	domainMap, err := ReadDomains(gistUrl)
	if err != nil {
		log.Panic(err)
	}

	lock := new(sync.RWMutex)
	cache := make(map[string]string)

	mux := http.NewServeMux()

	for prefix, url := range domainMap {

		ticker := time.NewTicker(30 * time.Second)

		go func(prefix, url string) {

			updateCache := func(key, value string) {

				lock.Lock()
				defer lock.Unlock()

				cache[key] = value
			}

			for {

				<-ticker.C

				response, err := ReadWebsite(url)
				if err != nil {
					log.Println(err)
					continue
				}

				if len(response) == 0 {
					continue
				}

				updateCache(prefix, response)
			}

		}(prefix, url)

		pattern := fmt.Sprintf("GET /%s/get.php", prefix)

		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {

			lock.RLock()
			defer lock.RUnlock()

			fmt.Fprintln(w, cache[prefix])
		})

		log.Printf("Added: %s\n", pattern)
	}

	return mux
}

var mux = GetRouter()

func Handler(w http.ResponseWriter, r *http.Request) {
	mux.ServeHTTP(w, r)
}
