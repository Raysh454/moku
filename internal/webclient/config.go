package webclient

type Client string

const (
	ClientNetHTTP  Client = "nethttp"
	ClientChromedp Client = "chromedp"
)

// Config is the minimal interface required for constructing a WebClient.
// It is implemented by app.Config without creating an import cycle.
type Config struct {
	Client Client
}
