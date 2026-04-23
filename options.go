package polymarket

import (
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/bridge"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob/ws"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/ctf"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/data"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/gamma"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/rtds"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/transport"
)

// Option mutates the root client.
type Option func(*Client)

func WithConfig(cfg Config) Option {
	return func(c *Client) {
		c.Config = cfg
	}
}

func WithHTTPClient(doer transport.Doer) Option {
	return func(c *Client) {
		c.Config.HTTPClient = doer
	}
}

func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.Config.UserAgent = userAgent
	}
}

func WithUseServerTime(use bool) Option {
	return func(c *Client) {
		c.Config.UseServerTime = use
	}
}

// WithCLOBWSConfig sets explicit WebSocket runtime behavior for the CLOB WS client.
func WithCLOBWSConfig(cfg ws.ClientConfig) Option {
	return func(c *Client) {
		c.Config.CLOBWSConfig = cfg
	}
}

// WithRTDSConfig sets explicit runtime behavior for the RTDS WebSocket client.
func WithRTDSConfig(cfg rtds.ClientConfig) Option {
	return func(c *Client) {
		c.Config.RTDSConfig = cfg
	}
}

func WithCLOB(client clob.Client) Option {
	return func(c *Client) {
		c.CLOB = client
	}
}

func WithCLOBWS(client ws.Client) Option {
	return func(c *Client) {
		c.CLOBWS = client
	}
}

func WithGamma(client gamma.Client) Option {
	return func(c *Client) {
		c.Gamma = client
	}
}

func WithData(client data.Client) Option {
	return func(c *Client) {
		c.Data = client
	}
}

func WithBridge(client bridge.Client) Option {
	return func(c *Client) {
		c.Bridge = client
	}
}

func WithRTDS(client rtds.Client) Option {
	return func(c *Client) {
		c.RTDS = client
	}
}

func WithCTF(client ctf.Client) Option {
	return func(c *Client) {
		c.CTF = client
	}
}


