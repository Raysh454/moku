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
	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
)

// ChromeDPClient is a chromedp-backed implementation of the WebClient interface.
// It currently supports GET semantics only
type ChromeDPClient struct {
	baseCtx     context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc

	mu     sync.Mutex
	closed bool

	wg sync.WaitGroup

	idleAfter time.Duration
	logger    interfaces.Logger
}

func NewChromedpClient(cfg *app.Config, logger interfaces.Logger) (interfaces.WebClient, error) {
	// Create component-scoped logger
	componentLogger := logger.With(interfaces.Field{Key: "backend", Value: "chromedp"})

	// Note: chromedp backend is not fully implemented in dev branch
	componentLogger.Warn("chromedp webclient is not fully implemented in dev branch")

	idleAfter := 2 * time.Second

	// If no allocator options were provided, use the simpler NewContext directly.
	// This avoids a code path in NewExecAllocator that has proven brittle in some test environments.
	ctx, cancel := chromedp.NewContext(context.Background())

	if err := chromedp.Run(ctx); err != nil {
		cancel()
		componentLogger.Warn("failed to start chromedp client", interfaces.Field{Key: "error", Value: err.Error()})
		return nil, fmt.Errorf("starting chromedp client: %w", err)
	}

	componentLogger.Info("created chromedp webclient",
		interfaces.Field{Key: "idle_after", Value: idleAfter.String()})

	return &ChromeDPClient{
		baseCtx:   ctx,
		cancel:    cancel,
		idleAfter: idleAfter,
		logger:    componentLogger,
	}, nil
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

	if cdc.cancel != nil {
		cdc.cancel()
	}

	cdc.wg.Wait()

	if cdc.allocCancel != nil {
		cdc.allocCancel()
	}

	return nil
}

func (cdc *ChromeDPClient) waitNetworkIdle(ctx context.Context) chan struct{} {
	idleChan := make(chan struct{})
	var activeReqs int32
	var timer *time.Timer
	var timerMutex sync.Mutex
	var once sync.Once

	startTimer := func() {
		timerMutex.Lock()
		defer timerMutex.Unlock()

		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(cdc.idleAfter, func() {
			if atomic.LoadInt32(&activeReqs) == 0 {
				once.Do(func() {
					close(idleChan)
				})
			}
		})
	}

	chromedp.ListenTarget(ctx,
		func(ev any) {
			switch ev.(type) {
			case *network.EventRequestWillBeSent:
				atomic.AddInt32(&activeReqs, 1)
			case *network.EventLoadingFinished, *network.EventLoadingFailed:
				if atomic.AddInt32(&activeReqs, -1) == 0 {
					startTimer()
				}
			}
		})

	return idleChan
}

func (cdc *ChromeDPClient) SetHeaders(taskCtx context.Context, headers http.Header) error {
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

// attachMainResponseListener attaches a listener to the provided target context that captures
// the main document response (if any) into mainRespPtr. The listener is scoped to the target ctx.
func (cdc *ChromeDPClient) attachMainResponseListener(targetCtx context.Context, mainRespPtr **network.Response, mu *sync.Mutex) {
	chromedp.ListenTarget(targetCtx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e.Type == network.ResourceTypeDocument {
				mu.Lock()
				*mainRespPtr = e.Response
				mu.Unlock()
			}
		}
	})
}

// AssembleHeaders converts chromedp network headers to http.Header
func (cdc *ChromeDPClient) AssembleHeaders(src *network.Headers, dest *http.Header) {
	if src == nil || dest == nil {
		return
	}

	for k, v := range *src {
		switch vv := v.(type) {
		case string:
			dest.Add(k, vv)
		case []string:
			for _, sv := range vv {
				dest.Add(k, sv)
			}
		default:
			dest.Add(k, fmt.Sprintf("%v", vv))
		}
	}
}

// Make a request using headless chrome (chromedp).
// Currently only supports GET
func (cdc *ChromeDPClient) Do(ctx context.Context, req *model.Request) (*model.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet {
		cdc.logger.Warn("chromedp client only supports GET",
			interfaces.Field{Key: "method", Value: method})
		return nil, fmt.Errorf("chromedp client: method %q not supported", method)
	}

	cdc.logger.Debug("chromedp request",
		interfaces.Field{Key: "method", Value: method},
		interfaces.Field{Key: "url", Value: req.URL})

	cdc.mu.Lock()
	if cdc.closed {
		cdc.mu.Unlock()
		return nil, fmt.Errorf("chromedp client closed")
	}
	cdc.wg.Add(1)
	cdc.mu.Unlock()
	defer cdc.wg.Done()

	rctx, rcancel := chromedp.NewContext(cdc.baseCtx)
	defer rcancel()

	taskCtx, taskCancel := context.WithTimeout(rctx, 60*time.Second)
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

	if err := cdc.SetHeaders(taskCtx, req.Headers); err != nil {
		return nil, err
	}

	waitIdleChan := cdc.waitNetworkIdle(rctx)

	var mainRespMu sync.Mutex
	var mainResp *network.Response
	cdc.attachMainResponseListener(rctx, &mainResp, &mainRespMu)

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(req.URL),
	)

	if err != nil {
		return nil, fmt.Errorf("error navigating to %s: %w", req.URL, err)
	}

	select {
	case <-waitIdleChan:
	case <-taskCtx.Done():
		return nil, fmt.Errorf("waiting for network idle: %w", taskCtx.Err())
	}

	var html string

	err = chromedp.Run(taskCtx,
		chromedp.OuterHTML("html", &html),
	)

	if err != nil {
		return nil, fmt.Errorf("error fetching html: %w", err)
	}

	// Assemble headers from mainResp
	mainRespMu.Lock()
	responseHeaders := http.Header{}
	var statusCode int

	if mainResp != nil {
		statusCode = int(mainResp.Status)
		cdc.AssembleHeaders(&mainResp.Headers, &responseHeaders)
		mainRespMu.Unlock()

	}

	return &model.Response{
		Request:    req,
		StatusCode: statusCode,
		Headers:    responseHeaders,
		Body:       []byte(html),
		FetchedAt:  time.Now(),
	}, nil

}

// Get is a convenience method for simple GET requests
func (cdc *ChromeDPClient) Get(ctx context.Context, url string) (*model.Response, error) {
	req := &model.Request{
		Method: "GET",
		URL:    url,
	}
	return cdc.Do(ctx, req)
}
