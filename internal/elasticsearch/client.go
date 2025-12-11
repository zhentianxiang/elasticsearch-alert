package elasticsearch

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"elasticsearch-alert/internal/config"

	es "github.com/elastic/go-elasticsearch/v8"
	osv2 "github.com/opensearch-project/opensearch-go/v2"
)

type Client struct {
	provider string
	es       *es.Client
	os       *osv2.Client
}

func NewClient(cfg config.ElasticsearchConfig) (*Client, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Default to Elasticsearch
	provider := cfg.Provider
	if provider == "" {
		provider = "elasticsearch"
	}

	if provider == "opensearch" {
		osCfg := osv2.Config{
			Addresses: cfg.Addresses,
			Username:  cfg.Username,
			Password:  cfg.Password,
			Transport: transport,
		}
		osClient, err := osv2.NewClient(osCfg)
		if err != nil {
			return nil, err
		}
		return &Client{provider: provider, os: osClient}, nil
	} else {
		esCfg := es.Config{
			Addresses: cfg.Addresses,
			Username:  cfg.Username,
			Password:  cfg.Password,
			CloudID:   cfg.CloudID,
			APIKey:    cfg.APIKey,
			Transport: transport,
		}
		// For older Elasticsearch (<7.14) or proxies stripping headers, allow skipping product check
		if cfg.SkipProductCheck {
			_ = os.Setenv("ELASTIC_CLIENT_SKIP_PRODUCT_CHECK", "true")
		}
		esClient, err := es.NewClient(esCfg)
		if err != nil {
			return nil, err
		}
		return &Client{provider: provider, es: esClient}, nil
	}
}

// Response is a minimal wrapper used by the alert engine
type Response struct {
	Body       io.ReadCloser
	statusCode int
	raw        string
	isError    bool
}

func (r *Response) IsError() bool  { return r.isError }
func (r *Response) String() string { return r.raw }

// Search executes a search on the given index with the provided JSON body
func (c *Client) Search(index string, body *bytes.Buffer) (*Response, error) {
	switch c.provider {
	case "opensearch":
		res, err := c.os.Search(
			c.os.Search.WithIndex(index),
			c.os.Search.WithBody(body),
			c.os.Search.WithTrackTotalHits(true),
			c.os.Search.WithPretty(),
		)
		if err != nil {
			return nil, err
		}
		return &Response{
			Body:       res.Body,
			statusCode: res.StatusCode,
			raw:        res.String(),
			isError:    res.IsError(),
		}, nil
	default:
		res, err := c.es.Search(
			c.es.Search.WithIndex(index),
			c.es.Search.WithBody(body),
			c.es.Search.WithTrackTotalHits(true),
			c.es.Search.WithRestTotalHitsAsInt(true),
			c.es.Search.WithPretty(),
		)
		if err != nil {
			return nil, err
		}
		return &Response{
			Body:       res.Body,
			statusCode: res.StatusCode,
			raw:        res.String(),
			isError:    res.IsError(),
		}, nil
	}
}
