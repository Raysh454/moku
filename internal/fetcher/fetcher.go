package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/raysh454/moku/internal/utils"
)

// Module: fetcher
// Fetches, Normalizes and stores pages

type Page struct {
	Path       string
	Data       string
	Headers    *http.Header
	StatusCode int
}

type Fetcher struct {
	MaxConcurrency int
}

func NewFetcher(MaxCouncurrency int) *Fetcher {
	if MaxCouncurrency > 4 {
		fmt.Printf("Warning!! high concurrency level: %d, Writing too many files will cause problems for your machine!", MaxCouncurrency)
	}
	return &Fetcher{
		MaxConcurrency: MaxCouncurrency,
	}
}

// Gets and stores all given HTTP urls to file system
func (f *Fetcher) Fetch(pageUrls []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, f.MaxConcurrency)

	for _, pageUrl := range pageUrls {
		wg.Add(1)

		go func(pageUrl string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			page, err := f.HTTPGet(pageUrl)
			if err != nil {
				fmt.Printf("error while fetching %s: %v. Skipping...", pageUrl, err)
				return
			}
		
			if err := f.Store(page); err != nil {
				fmt.Printf("error while storing %s: %v. Skipping...", pageUrl, err)
			}
		}(pageUrl)
	}

	wg.Wait()
}

// Makes an HTTP GET Request to the given parameter and returns reference Page struct
func (f *Fetcher) HTTPGet(page string) (*Page, error) {
	resp, err := http.Get(page)
	if err != nil {
		return nil, fmt.Errorf("error GETting %s: %w", page, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading response body %s: %w", page, err)
	}

	bodyStr := string(body)	
	//bodyStr = f.Normalize(bodyStr) Need to normalize repeating stuff, unless I come up with a better solution, cause this seems hard

	urlTools, err := utils.NewURLTools(page)
	if err != nil {
		return nil, fmt.Errorf("error while parsing url %s: %w", page, err)
	}

	return &Page{
		Path: urlTools.GetPath(),
		Data: bodyStr,
		Headers: &resp.Header,
		StatusCode: resp.StatusCode,
	}, nil
}

// Helper function: stores page data to file
func (f *Fetcher) storePageData(page *Page) error {
	pageDataFile, err := os.Create(page.Path + "/.page_data")
	if err != nil {
		return err
	}
	defer pageDataFile.Close()

	_, err = pageDataFile.WriteString(page.Data)
	if err != nil {
		return err
	}

	return nil
}

// Helper function: stores header data to file
func (f *Fetcher) storeHeaderData(page *Page) error {
	headerFile, err := os.Create(page.Path + "/.page_headers") 
	if err != nil {
		return err
	}
	defer headerFile.Close()

	for key, val := range *page.Headers {
		headerFile.WriteString(fmt.Sprintf("%s: %s", key, val))
	}

	return nil
}

// Stores a Page struct to the file system
func (f *Fetcher) Store(page *Page) error {
	err := os.MkdirAll(page.Path, 0755)
	if err != nil {
		return err
	}

	if err = f.storePageData(page); err != nil {
		return err
	}

	if err = f.storeHeaderData(page); err != nil {
		return err
	}

	return nil
}
