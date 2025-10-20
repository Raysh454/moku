package webclient

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/raysh454/moku/internal/model"
)


type ChromeDPClient struct {

}

func waitNetworkIdle(ctx context.Context, idleAfter time.Duration) chan struct{} {
	idleChan := make(chan struct{})
	var activeReqs int32
	var timer *time.Timer
	var timerMutex sync.Mutex
	var once sync.Once

	startTimer := func(){
		timerMutex.Lock()
		defer timerMutex.Unlock()

		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(idleAfter, func() {
			if atomic.LoadInt32(&activeReqs) == 0 {
				once.Do(func() {
					idleChan<-struct{}{}
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

func (cdc *ChromeDPClient) Do(ctx context.Context, req *model.Request) (res *model.Response, err error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	waitIdleChan := waitNetworkIdle(ctx, 2 * time.Second)

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		)

	if err != nil {
		return "", err
	}

	<-waitIdleChan

	var html string

	err = chromedp.Run(ctx,
		chromedp.OuterHTML("html", &html),
		)

	return html, err
}

