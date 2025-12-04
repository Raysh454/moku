package enumerator

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
	"golang.org/x/net/html"
)

type Spider struct {
	MaxDepth int
	wc       webclient.WebClient
	logger   logging.Logger
}

type spiderHelper struct {
	spider  *Spider
	root    *utils.URLTools
	depth   map[string]int
	results []string
	re      *regexp.Regexp //TODO: Need a better way to parse urls
}

// NewSpider creates a new Spider with the given webclient and logger.
// TODO: Update call sites to pass wc and logger when wiring modules in cmd/main or composition root.
func NewSpider(maxDepth int, wc webclient.WebClient, logger logging.Logger) *Spider {
	return &Spider{
		MaxDepth: maxDepth,
		wc:       wc,
		logger:   logger,
	}
}

func newSpiderHelper(spider *Spider, root string) (*spiderHelper, error) {
	rootUrl, err := utils.NewURLTools(root)
	if err != nil {
		return nil, err
	}

	return &spiderHelper{
		spider:  spider,
		root:    rootUrl,
		depth:   map[string]int{root: 0},
		results: []string{root},
		re:      regexp.MustCompile(`https?://[^\s"'<>]+`),
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
			if sh.spider.logger != nil {
				sh.spider.logger.Warn("couldn't resolve full url",
					logging.Field{Key: "url", Value: v},
					logging.Field{Key: "error", Value: err})
			}
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

func (sh *spiderHelper) crawlPage(ctx context.Context, target string) ([]string, error) {
	if sh.spider.wc == nil {
		return nil, fmt.Errorf("spider: webclient is nil")
	}

	resp, err := sh.spider.wc.Get(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("error making http request: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("received 404 from target")
	}

	bodyStr := string(resp.Body)
	var links []string

	contentType := resp.Headers.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") {
		doc, err := html.Parse(strings.NewReader(bodyStr))
		if err != nil {
			return nil, fmt.Errorf("couldn't parse %s: %w", target, err)
		}
		if err := sh.extractLinksHTML(doc, target, &links); err != nil {
			fmt.Fprintf(os.Stderr, "extractLinksHTML: %v\n", err)
		}
	} else {
		links = sh.re.FindAllString(bodyStr, -1)
	}

	return links, nil
}

func (sh *spiderHelper) appendPages(pages []string, lastDepth int) {
	for _, page := range pages {

		pageUrlTools, err := utils.NewURLTools(page)
		if err != nil {
			if sh.spider.logger != nil {
				sh.spider.logger.Warn("error parsing page url",
					logging.Field{Key: "url", Value: page},
					logging.Field{Key: "error", Value: err})
			}
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

func (sh *spiderHelper) run(ctx context.Context) error {
	currPage := 0

	for currPage < len(sh.results) {
		if depth, exists := sh.depth[sh.results[currPage]]; exists && depth > sh.spider.MaxDepth {
			break
		}
		crawledPages, err := sh.crawlPage(ctx, sh.results[currPage])
		if err != nil {
			if sh.spider.logger != nil {
				sh.spider.logger.Error("error while crawling page",
					logging.Field{Key: "url", Value: sh.results[currPage]},
					logging.Field{Key: "error", Value: err})
			}
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

	// Use background context for now; TODO: pass ctx from caller
	if err := helper.run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "helper.run failed: %v\n", err)
	}
	return helper.results, nil
}
