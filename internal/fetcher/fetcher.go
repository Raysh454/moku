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
	RootPath string
	MaxConcurrency int
}

func NewFetcher(RootPath string, MaxCouncurrency int) (*Fetcher, error) {
	if RootPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error: root directory not given and failed to get working directory: %v", err)
		}

		RootPath = wd
	}

	return &Fetcher{
		RootPath: RootPath,
		MaxConcurrency: MaxCouncurrency,
	}, nil
}

// Gets and stores all given HTTP urls to file system
func (f *Fetcher) Fetch(pageUrls []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, f.MaxConcurrency)
	diskWriter := make(chan *Page)

	// Write pages to disk
	go func() {
		for page := range diskWriter {
			if err := f.StorePage(page); err != nil {
				fmt.Printf("error while storing %s: %v. Skipping...", page.Path, err)
			}
		}
	}()

	// Fetch pages concurrently
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
		
			diskWriter <- page
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
	pageDataFile, err := os.Create(f.RootPath + page.Path + "/.page_data")
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
	headerFile, err := os.Create(f.RootPath + page.Path + "/.page_headers") 
	if err != nil {
		return err
	}
	defer headerFile.Close()

	for key, val := range *page.Headers {
		headerFile.WriteString(fmt.Sprintf("%s: %s\n", key, val))
	}

	return nil
}

// Stores a Page struct to the file system
func (f *Fetcher) StorePage(page *Page) error {
	err := os.MkdirAll(f.RootPath + page.Path, 0755)
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

// Returns the directory as a Page struct. Will throw an error if .page_headers or .page_data don't exist (Maybe will change)
// Do we even need this?
/*
func (f *Fetcher) GetDir(path string) (*Page, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error while opening dir %s: %w", path, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a dir", path)
	}

	pageData, err := os.ReadFile(path + "/.page_data")
	if err != nil {
		return nil, fmt.Errorf("%s doesn't exist", path + "/.page_data")
	}
	
	headers, err := f.parseHeaderFile(path + "/.page_headers")
	if err != nil {
		return nil, fmt.Errorf("error while parsing %s", path + "/.page_headers")
	}
	
}

// Parses given file to http.Header and returns a pointer to it
func (f *Fetcher) parseHeaderFile(path string) (*http.Header, error) {
	headerData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s doesn't exist", path)
	}

	headers := http.Header{}
	
	headerDataStr := string(headerData)
	for _, val := range strings.Split(headerDataStr, "\n") {
		header := strings.Split(val, ":")
		if len(header) == 1 {
			fmt.Printf("malformed header: %s in %s. Skipping...", header, path)
		}
		
		for _, values := range header[1:] {
			headers.Add(header[0], values)
		}
	}

	return &headers, nil
}
*/
