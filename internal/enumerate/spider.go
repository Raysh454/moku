package enumerate

import (
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/raysh454/moku/internal/utils"
)

type Spider struct {
	MaxDepth int
}

type spiderHelper struct {
	spider *Spider;
	root *utils.URLTools;
	depth map[string]int;
	results []string;
	re *regexp.Regexp; //TODO: Need a better way to parse urls
}

func NewSpider(maxDepth int) *Spider {
	return &Spider{
		MaxDepth: maxDepth,
	}
}

func newSpiderHelper(spider *Spider, root string) (*spiderHelper, error) {
	rootUrl, err := utils.NewURLTools(root)
	if err != nil {
		return nil, err
	}

	return &spiderHelper{
		spider: spider,
		root: rootUrl,
		depth: map[string]int{root: 0},
		results: []string{root},
		re: regexp.MustCompile(`<a\s[^>]*href=(\"??)([^\" >]*?)\\1[^>]*>(.*)<\/a>`),
	}, nil
}

func (nsh *spiderHelper) crawlPage(target string) ([]string, error) {
	resp, err := http.Get(target)
	if err != nil {
		return nil, fmt.Errorf("error making http request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("received 404 from target")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %w", err)
	}

	bodyStr := string(body)
	matches := nsh.re.FindAllString(bodyStr, -1)

	return matches, nil
}

func (nsh *spiderHelper) appendPages(pages []string, lastDepth int) {
		for _, page := range pages {
			
			pageUrlTools, err := utils.NewURLTools(page)
			if err != nil {
				fmt.Printf("%v\n", err)
				continue
			}

			if !nsh.root.DomainIsSame(pageUrlTools) {
				continue
			}

			pageStr := pageUrlTools.URL.String()

			if _, exists := nsh.depth[pageStr]; !exists {
				nsh.depth[pageStr] = lastDepth + 1
				if nsh.depth[pageStr] <= nsh.spider.MaxDepth {
					nsh.results = append(nsh.results, pageStr)
				}
			}
		}
}

func (nsh *spiderHelper) run() error {
	currPage := 0

	for currPage < len(nsh.results) {
		crawledPages, err := nsh.crawlPage(nsh.results[currPage])
		if err != nil {
			fmt.Printf("error while crawling %s: %v\n", nsh.results[currPage], err)
		}

		currDepth := nsh.depth[nsh.results[currPage]] 

		nsh.appendPages(crawledPages, currDepth)
		currPage += 1
	}

	return nil
}

func (s *Spider) Enumerate(target string) ([]string, error) {
	helper, err := newSpiderHelper(s, target)	
	if err != nil {
		return nil, err
	}

	helper.run()
	return helper.results, nil
}
