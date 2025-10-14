package enumerate

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/raysh454/moku/internal/utils"
	"golang.org/x/net/html"
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
		re: regexp.MustCompile(`https?://[^\s"'<>]+`),
	}, nil
}

func (sh *spiderHelper) resolveFullUrls(baseUrl string, links []string) ([]string, error) {
	base, err := utils.NewURLTools(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("error while converting %s to URLTools: %w", baseUrl, err)
	}

	var result []string

	for _, v := range links {
		resolved, err := base.ResolveFullUrlString(v)
		if err != nil {
			fmt.Printf("Couldn't resolve full url for %s: %v", v, err)
			continue
		}

		result = append(result, resolved)
	}

	return result, nil
} 

func (sh *spiderHelper) extractLinksHTML(node *html.Node, baseUrl string, links *[]string) error {
	if node.Type == html.ElementNode {
		hasSrc := false
		var cLinks []string

		for _, attr := range node.Attr {
			if attr.Key == "href" || attr.Key == "src" {
				cLinks = append(cLinks, attr.Val)
				hasSrc = true
			}
		}

		if node.Data == "script" && !hasSrc && node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
			cLinks = append(cLinks, sh.re.FindAllString(node.FirstChild.Data, -1)...)
		}

		rLinks, err := sh.resolveFullUrls(baseUrl, cLinks)
		if err != nil {
			return fmt.Errorf("error while resolving full urls: %w", err)
		}

		*links = append(*links, rLinks...)
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if err := sh.extractLinksHTML(c, baseUrl, links); err != nil {
			return err
		}	
	}

	return nil
}

func (sh *spiderHelper) crawlPage(target string) ([]string, error) {
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
	var links []string

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		doc, err := html.Parse(strings.NewReader(bodyStr))
		if err != nil {
			return nil, fmt.Errorf("couldn't parse %s: %w", target, err)
		}
		sh.extractLinksHTML(doc, target, &links)
	} else {
		links = sh.re.FindAllString(bodyStr, -1)
	}

	return links, nil
}

func (sh *spiderHelper) appendPages(pages []string, lastDepth int) {
		for _, page := range pages {
			
			pageUrlTools, err := utils.NewURLTools(page)
			if err != nil {
				fmt.Printf("%v\n", err)
				continue
			}

			if !sh.root.DomainIsSame(pageUrlTools) {
				continue
			}

			pageStr := pageUrlTools.URL.String()

			if _, exists := sh.depth[pageStr]; !exists {
				sh.depth[pageStr] = lastDepth + 1
				sh.results = append(sh.results, pageStr)
			}
		}
}

func (sh *spiderHelper) run() error {
	currPage := 0

	for currPage < len(sh.results) {
		if depth, exists := sh.depth[sh.results[currPage]]; exists && depth > sh.spider.MaxDepth {
			break
		}
		crawledPages, err := sh.crawlPage(sh.results[currPage])
		if err != nil {
			fmt.Printf("error while crawling %s: %v\n", sh.results[currPage], err)
		}

		currDepth := sh.depth[sh.results[currPage]] 

		sh.appendPages(crawledPages, currDepth)
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
