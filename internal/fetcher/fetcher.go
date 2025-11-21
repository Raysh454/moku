package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/raysh454/moku/internal/interfaces"
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
	RootPath       string
	MaxConcurrency int
	wc             interfaces.WebClient
	logger         interfaces.Logger
}

// New creates a new Fetcher with the given webclient and logger.
// TODO: Update call sites to pass wc and logger when wiring modules in cmd/main or composition root.
func New(RootPath string, MaxConcurrency int, wc interfaces.WebClient, logger interfaces.Logger) (*Fetcher, error) {
	if RootPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error: root directory not given and failed to get working directory: %v", err)
		}

		RootPath = wd
	}

	return &Fetcher{
		RootPath:       RootPath,
		MaxConcurrency: MaxConcurrency,
		wc:             wc,
		logger:         logger,
	}, nil
}

// Gets and stores all given HTTP urls to file system
func (f *Fetcher) Fetch(pageUrls []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, f.MaxConcurrency)
	diskWriter := make(chan *Page)
	writerDone := make(chan struct{})

	// Write pages to disk
	go func() {
		defer close(writerDone)
		for page := range diskWriter {
			if err := f.StorePage(page); err != nil {
				if f.logger != nil {
					f.logger.Error("error while storing page", 
						interfaces.Field{Key: "path", Value: page.Path},
						interfaces.Field{Key: "error", Value: err})
				}
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

			page, err := f.HTTPGet(context.Background(), pageUrl)
			if err != nil {
				if f.logger != nil {
					f.logger.Error("error while fetching page", 
						interfaces.Field{Key: "url", Value: pageUrl},
						interfaces.Field{Key: "error", Value: err})
				}
				return
			}
		
			diskWriter <- page
		}(pageUrl)
	}

	wg.Wait()
	close(diskWriter)
	<-writerDone
}

// Makes an HTTP GET Request to the given parameter and returns reference Page struct
func (f *Fetcher) HTTPGet(ctx context.Context, page string) (*Page, error) {
	if f.wc == nil {
		return nil, fmt.Errorf("fetcher: webclient is nil")
	}

	resp, err := f.wc.Get(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("error GETting %s: %w", page, err)
	}

	bodyStr := string(resp.Body)	
	//bodyStr = f.Normalize(bodyStr) Need to normalize repeating stuff, unless I come up with a better solution, cause this seems hard

	urlTools, err := utils.NewURLTools(page)
	if err != nil {
		return nil, fmt.Errorf("error while parsing url %s: %w", page, err)
	}

	return &Page{
		Path:       urlTools.GetPath(),
		Data:       bodyStr,
		Headers:    &resp.Headers,
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
		if _, err := headerFile.WriteString(fmt.Sprintf("%s: %s\n", key, val)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: write header file: %v\n", err)
		}
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
			if f.logger != nil {
				f.logger.Warn("malformed header, skipping",
					interfaces.Field{Key: "header", Value: header},
					interfaces.Field{Key: "path", Value: path})
			}
		}
		
		for _, values := range header[1:] {
			headers.Add(header[0], values)
		}
	}

	return &headers, nil
}
*/
