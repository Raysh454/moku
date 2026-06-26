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
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/raysh454/moku/internal/logging"
)

// DOM-quiescence tuning: after network-idle, poll the page's mutation counter
// every domSettleInterval and treat the DOM as settled once it is unchanged for
// domSettleStablePolls consecutive polls, bounded by domSettleCeiling. This
// replaces a timing-dependent fixed delay with an explicit stability signal —
// the largest single reduction in run-to-run diff noise.
const (
	domSettleInterval     = 250 * time.Millisecond
	domSettleCeiling      = 10 * time.Second
	domSettleStablePolls  = 2
	chromedpRequestBudget = 60 * time.Second
)

// determinismScript runs at document-start (before any page script) to make the
// rendered DOM reproducible across fetches: it freezes the clock and RNG so
// timestamps/random ids stop churning, installs a mutation counter used for
// DOM-quiescence detection, and disables animations/transitions. Every override
// is wrapped in try/catch so a hardened page cannot break the capture.
const determinismScript = `(function () {
  try { Date.now = function () { return 1700000000000; }; } catch (e) {}
  try { if (window.performance) performance.now = function () { return 0; }; } catch (e) {}
  try { Math.random = function () { return 0.5; }; } catch (e) {}
  try {
    if (window.crypto && crypto.getRandomValues) {
      crypto.getRandomValues = function (a) { for (var i = 0; i < a.length; i++) a[i] = 0; return a; };
    }
  } catch (e) {}
  window.__mokuMutations = 0;
  try {
    var mo = new MutationObserver(function (m) { window.__mokuMutations += m.length; });
    var start = function () {
      try { mo.observe(document.documentElement || document, { subtree: true, childList: true, attributes: true, characterData: true }); } catch (e) {}
    };
    if (document.documentElement) start(); else document.addEventListener('DOMContentLoaded', start);
  } catch (e) {}
  var css = function () {
    try {
      var s = document.createElement('style');
      s.textContent = '*,*::before,*::after{animation:none !important;transition:none !important;caret-color:transparent !important;}';
      (document.head || document.documentElement).appendChild(s);
    } catch (e) {}
  };
  if (document.head) css(); else document.addEventListener('DOMContentLoaded', css);
})();`

// ChromeDPClient drives a long-lived headless browser over CDP and returns the
// final rendered HTML. It owns the browser lifecycle: one exec allocator, one
// browser recycled per RecyclePolicy to bound Chromium's memory growth, and a
// fresh tab per fetch. Captures are made deterministic via determinismScript and
// a DOM-quiescence wait so periodic diffs reflect real content changes.
type ChromeDPClient struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc

	mu            sync.Mutex
	closed        bool
	browserCtx    context.Context
	browserCancel context.CancelFunc
	browserBirth  time.Time
	browserUses   int
	inFlight      int
	wg            sync.WaitGroup

	recycle      RecyclePolicy
	idleTimeout  time.Duration
	maxBodyBytes int64
	logger       logging.Logger
}

func NewChromedpClient(cfg Config, logger logging.Logger) (WebClient, error) {
	componentLogger := logger.With(logging.Field{Key: "backend", Value: "chromedp"})

	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if cfg.ChromePath != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(cfg.ChromePath))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)

	// Verify the browser actually starts, bounded so an environment without
	// Chrome fails fast rather than hanging.
	startupCtx, startupCancel := context.WithTimeout(allocCtx, 10*time.Second)
	defer startupCancel()
	testCtx, testCancel := chromedp.NewContext(startupCtx)
	defer testCancel()
	if err := chromedp.Run(testCtx, network.Enable()); err != nil {
		allocCancel()
		return nil, fmt.Errorf("starting chromedp (timeout after 10s): %w", err)
	}

	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	componentLogger.Info("created chromedp webclient",
		logging.Field{Key: "chrome_path", Value: cfg.ChromePath})

	return &ChromeDPClient{
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
		browserBirth:  time.Now(),
		recycle:       DefaultRecyclePolicy(),
		idleTimeout:   2 * time.Second,
		maxBodyBytes:  maxBodyBytes,
		logger:        componentLogger,
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

	browserCtx, err := cdc.acquire()
	if err != nil {
		return nil, err
	}
	defer cdc.release()

	cdc.logger.Debug("chromedp request",
		logging.Field{Key: "method", Value: method},
		logging.Field{Key: "url", Value: req.URL})

	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer tabCancel()

	taskCtx, taskCancel := context.WithTimeout(tabCtx, chromedpRequestBudget)
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

	// Inject the determinism script before the page's own scripts run. Page must
	// be enabled for AddScriptToEvaluateOnNewDocument; enabling is idempotent.
	// Best-effort: if injection fails the capture proceeds without it (less
	// deterministic) rather than failing the whole fetch.
	if err := chromedp.Run(taskCtx,
		page.Enable(),
		chromedp.ActionFunc(func(c context.Context) error {
			_, scriptErr := page.AddScriptToEvaluateOnNewDocument(determinismScript).Do(c)
			return scriptErr
		}),
	); err != nil {
		cdc.logger.Warn("determinism script injection failed; capturing without it",
			logging.Field{Key: "url", Value: req.URL},
			logging.Field{Key: "error", Value: err.Error()})
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

	// Let late JS-driven DOM mutations (hydration, deferred renders) settle
	// before capture. Best-effort: on any error we fall through to extraction.
	cdc.waitForDOMStable(taskCtx)

	var html string
	if err := chromedp.Run(taskCtx, chromedp.OuterHTML("html", &html)); err != nil {
		return nil, fmt.Errorf("extracting html: %w", err)
	}

	// Reject oversized documents outright rather than truncating: a partial
	// body would corrupt downstream snapshots and diffs. Mirrors the nethttp
	// backend's body cap.
	if int64(len(html)) > cdc.maxBodyBytes {
		return nil, fmt.Errorf("%w: %s", ErrBodyTooLarge, req.URL)
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

// acquire admits one fetch onto the shared browser. It recycles the browser
// first when the policy says it is tired AND no fetch is in flight (so no live
// tab is torn down), then records the in-flight fetch and returns the current
// browser context for the caller to open its tab from.
func (cdc *ChromeDPClient) acquire() (context.Context, error) {
	cdc.mu.Lock()
	defer cdc.mu.Unlock()

	if cdc.closed {
		return nil, fmt.Errorf("chromedp client closed")
	}

	if cdc.inFlight == 0 && cdc.recycle.ShouldRecycle(cdc.browserUses, time.Since(cdc.browserBirth)) {
		cdc.recycleBrowserLocked()
	}

	cdc.browserUses++
	cdc.inFlight++
	cdc.wg.Add(1)
	return cdc.browserCtx, nil
}

func (cdc *ChromeDPClient) release() {
	cdc.mu.Lock()
	cdc.inFlight--
	cdc.mu.Unlock()
	cdc.wg.Done()
}

// recycleBrowserLocked tears down the current browser and starts a fresh one off
// the shared allocator, resetting the age/use counters. Callers must hold mu and
// must only call this when no fetch is in flight.
func (cdc *ChromeDPClient) recycleBrowserLocked() {
	cdc.logger.Info("recycling browser",
		logging.Field{Key: "uses", Value: cdc.browserUses},
		logging.Field{Key: "age", Value: time.Since(cdc.browserBirth).String()})
	if cdc.browserCancel != nil {
		cdc.browserCancel()
	}
	cdc.browserCtx, cdc.browserCancel = chromedp.NewContext(cdc.allocCtx)
	cdc.browserBirth = time.Now()
	cdc.browserUses = 0
}

// waitForDOMStable polls the page's mutation counter until the DOM is quiescent
// or the ceiling is reached. It is best-effort: any evaluation error or context
// cancellation simply ends the wait and lets capture proceed.
func (cdc *ChromeDPClient) waitForDOMStable(taskCtx context.Context) {
	deadline := time.Now().Add(domSettleCeiling)
	last := -1.0
	stable := 0
	for time.Now().Before(deadline) {
		var count float64
		if err := chromedp.Run(taskCtx, chromedp.Evaluate(`window.__mokuMutations || 0`, &count)); err != nil {
			return
		}
		if count == last {
			stable++
			if stable >= domSettleStablePolls {
				return
			}
		} else {
			last = count
			stable = 0
		}
		select {
		case <-taskCtx.Done():
			return
		case <-time.After(domSettleInterval):
		}
	}
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
	if cdc.browserCancel != nil {
		cdc.browserCancel()
	}
	cdc.allocCancel()
	return nil
}
