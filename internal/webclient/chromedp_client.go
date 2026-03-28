package webclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/raysh454/moku/internal/logging"
)

type ChromeDPClient struct {
	baseCtx     context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	closed      bool
	wg          sync.WaitGroup
	idleTimeout time.Duration
	logger      logging.Logger
}

func NewChromedpClient(cfg Config, logger logging.Logger) (WebClient, error) {
	componentLogger := logger.With(logging.Field{Key: "backend", Value: "chromedp"})

	ctx, cancel := chromedp.NewContext(context.Background())

	if err := chromedp.Run(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("starting chromedp: %w", err)
	}

	componentLogger.Info("created chromedp webclient")

	return &ChromeDPClient{
		baseCtx:     ctx,
		cancel:      cancel,
		idleTimeout: 2 * time.Second,
		logger:      componentLogger,
	}, nil
}

func (cdc *ChromeDPClient) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet {
		return nil, fmt.Errorf("chromedp client: method %q not supported", method)
	}

	cdc.mu.Lock()
	if cdc.closed {
		cdc.mu.Unlock()
		return nil, fmt.Errorf("chromedp client closed")
	}
	cdc.wg.Add(1)
	cdc.mu.Unlock()
	defer cdc.wg.Done()

	cdc.logger.Debug("chromedp request",
		logging.Field{Key: "method", Value: method},
		logging.Field{Key: "url", Value: req.URL})

	tabCtx, tabCancel := chromedp.NewContext(cdc.baseCtx)
	defer tabCancel()

	taskCtx, taskCancel := context.WithTimeout(tabCtx, 60*time.Second)
	defer taskCancel()

	go func() {
		select {
		case <-ctx.Done():
			taskCancel()
		case <-taskCtx.Done():
		}
	}()

	if err := chromedp.Run(taskCtx, network.Enable()); err != nil {
		return nil, fmt.Errorf("enable network: %w", err)
	}

	var mainResp *network.Response
	var mainRespMu sync.Mutex
	chromedp.ListenTarget(tabCtx, func(ev any) {
		if e, ok := ev.(*network.EventResponseReceived); ok && e.Type == network.ResourceTypeDocument {
			mainRespMu.Lock()
			mainResp = e.Response
			mainRespMu.Unlock()
		}
	})

	if err := cdc.setHeaders(taskCtx, req.Headers); err != nil {
		return nil, err
	}

	idleCh := cdc.waitForNetworkIdle(tabCtx)

	if err := chromedp.Run(taskCtx, chromedp.Navigate(req.URL)); err != nil {
		return nil, fmt.Errorf("navigating to %s: %w", req.URL, err)
	}

	select {
	case <-idleCh:
	case <-taskCtx.Done():
		return nil, fmt.Errorf("waiting for network idle: %w", taskCtx.Err())
	}

	var html string
	if err := chromedp.Run(taskCtx, chromedp.OuterHTML("html", &html)); err != nil {
		return nil, fmt.Errorf("extracting html: %w", err)
	}

	mainRespMu.Lock()
	defer mainRespMu.Unlock()

	var statusCode int
	responseHeaders := http.Header{}
	if mainResp != nil {
		statusCode = int(mainResp.Status)
		responseHeaders = cdc.assembleHeaders(&mainResp.Headers)
	}

	return &Response{
		Request:    req,
		StatusCode: statusCode,
		Headers:    responseHeaders,
		Body:       []byte(html),
		FetchedAt:  time.Now(),
	}, nil
}

func (cdc *ChromeDPClient) Get(ctx context.Context, url string) (*Response, error) {
	return cdc.Do(ctx, &Request{
		Method: "GET",
		URL:    url,
	})
}

func (cdc *ChromeDPClient) waitForNetworkIdle(ctx context.Context) <-chan struct{} {
	idleCh := make(chan struct{})
	var activeReqs int32
	var timer *time.Timer
	var timerMu sync.Mutex
	var once sync.Once

	resetTimer := func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(cdc.idleTimeout, func() {
			if atomic.LoadInt32(&activeReqs) == 0 {
				once.Do(func() { close(idleCh) })
			}
		})
	}

	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev.(type) {
		case *network.EventRequestWillBeSent:
			atomic.AddInt32(&activeReqs, 1)
		case *network.EventLoadingFinished, *network.EventLoadingFailed:
			if atomic.AddInt32(&activeReqs, -1) == 0 {
				resetTimer()
			}
		}
	})

	return idleCh
}

func (cdc *ChromeDPClient) setHeaders(taskCtx context.Context, headers http.Header) error {
	if headers == nil {
		return nil
	}

	nh := network.Headers{}
	for k, vs := range headers {
		nh[k] = strings.Join(vs, ", ")
	}

	if err := chromedp.Run(taskCtx, network.SetExtraHTTPHeaders(nh)); err != nil {
		return fmt.Errorf("setting headers: %w", err)
	}
	return nil
}

func (cdc *ChromeDPClient) assembleHeaders(src *network.Headers) http.Header {
	h := http.Header{}
	if src == nil {
		return h
	}

	for k, v := range *src {
		switch val := v.(type) {
		case string:
			h.Add(k, val)
		case []string:
			for _, sv := range val {
				h.Add(k, sv)
			}
		default:
			h.Add(k, fmt.Sprintf("%v", val))
		}
	}
	return h
}

func (cdc *ChromeDPClient) Close() error {
	cdc.mu.Lock()
	if cdc.closed {
		cdc.mu.Unlock()
		return nil
	}
	cdc.closed = true
	cdc.mu.Unlock()

	cdc.logger.Info("closing chromedp webclient")
	cdc.wg.Wait()
	cdc.cancel()
	return nil
}
