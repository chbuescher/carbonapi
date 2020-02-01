package v3

import (
	"context"
	"encoding/json"
	"github.com/ansel1/merry"
	"net"
	"net/http"
	"net/url"

	"github.com/go-graphite/carbonapi/limiter"
	"github.com/go-graphite/carbonapi/zipper/helper"
	"github.com/go-graphite/carbonapi/zipper/httpHeaders"
	"github.com/go-graphite/carbonapi/zipper/metadata"
	"github.com/go-graphite/carbonapi/zipper/types"
	protov3 "github.com/go-graphite/protocol/carbonapi_v3_pb"

	"go.uber.org/zap"
)

const (
	format = "carbonapi_v3_pb"
)

func init() {
	aliases := []string{"carbonapi_v3_pb", "proto_v3_pb", "v3_pb"}
	metadata.Metadata.Lock()
	for _, name := range aliases {
		metadata.Metadata.SupportedProtocols[name] = struct{}{}
		metadata.Metadata.ProtocolInits[name] = New
		metadata.Metadata.ProtocolInitsWithLimiter[name] = NewWithLimiter
	}
	defer metadata.Metadata.Unlock()
}

// RoundRobin is used to connect to backends inside clientGroups, implements BackendServer interface
type ClientProtoV3Group struct {
	groupName string
	servers   []string

	client *http.Client

	limiter              limiter.ServerLimiter
	logger               *zap.Logger
	timeout              types.Timeouts
	maxTries             int
	maxMetricsPerRequest int

	httpQuery *helper.HttpQuery
}

func (c *ClientProtoV3Group) Children() []types.BackendServer {
	return []types.BackendServer{c}
}

func New(logger *zap.Logger, config types.BackendV2, tldCacheDisabled bool) (types.BackendServer, merry.Error) {
	if config.ConcurrencyLimit == nil {
		return nil, types.ErrConcurrencyLimitNotSet
	}
	if len(config.Servers) == 0 {
		return nil, types.ErrNoServersSpecified
	}
	limiter := limiter.NewServerLimiter([]string{config.GroupName}, *config.ConcurrencyLimit)

	return NewWithLimiter(logger, config, tldCacheDisabled, limiter)
}

func NewWithLimiter(logger *zap.Logger, config types.BackendV2, tldCacheDisabled bool, limiter limiter.ServerLimiter) (types.BackendServer, merry.Error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: *config.MaxIdleConnsPerHost,
			DialContext: (&net.Dialer{
				Timeout:   config.Timeouts.Connect,
				KeepAlive: *config.KeepAliveInterval,
				DualStack: true,
			}).DialContext,
		},
	}

	logger = logger.With(zap.String("type", "protoV3Group"), zap.String("name", config.GroupName))

	httpQuery := helper.NewHttpQuery(config.GroupName, config.Servers, *config.MaxTries, limiter, httpClient, httpHeaders.ContentTypeCarbonAPIv3PB)

	c := &ClientProtoV3Group{
		groupName:            config.GroupName,
		servers:              config.Servers,
		timeout:              *config.Timeouts,
		maxTries:             *config.MaxTries,
		maxMetricsPerRequest: config.MaxBatchSize,

		client:  httpClient,
		limiter: limiter,
		logger:  logger,

		httpQuery: httpQuery,
	}
	return c, nil
}

func (c ClientProtoV3Group) MaxMetricsPerRequest() int {
	return c.maxMetricsPerRequest
}

func (c ClientProtoV3Group) Name() string {
	return c.groupName
}

func (c ClientProtoV3Group) Backends() []string {
	return c.servers
}

func (c *ClientProtoV3Group) Fetch(ctx context.Context, request *protov3.MultiFetchRequest) (*protov3.MultiFetchResponse, *types.Stats, merry.Error) {
	stats := &types.Stats{}
	rewrite, _ := url.Parse("http://127.0.0.1/render/")
	logger := c.logger.With(zap.String("type", "fetch"), zap.String("request", request.String()))

	v := url.Values{
		"format": []string{format},
	}
	rewrite.RawQuery = v.Encode()

	res, e := c.httpQuery.DoQuery(ctx, logger, rewrite.RequestURI(), types.MultiFetchRequestV3{*request})
	if e != nil {
		return nil, stats, e

	}

	if res == nil {
		return nil, stats, types.ErrNoResponseFetched
	}

	var r protov3.MultiFetchResponse
	err := r.Unmarshal(res.Response)
	if err != nil {
		return nil, stats, merry.Wrap(err)
	}

	return &r, stats, nil
}

func (c *ClientProtoV3Group) Find(ctx context.Context, request *protov3.MultiGlobRequest) (*protov3.MultiGlobResponse, *types.Stats, merry.Error) {
	logger := c.logger.With(zap.String("type", "find"), zap.Strings("request", request.Metrics))
	stats := &types.Stats{}
	rewrite, _ := url.Parse("http://127.0.0.1/metrics/find/")

	v := url.Values{
		"format": []string{format},
	}
	rewrite.RawQuery = v.Encode()

	res, err := c.httpQuery.DoQuery(ctx, logger, rewrite.RequestURI(), types.MultiGlobRequestV3{*request})
	if err != nil {
		return nil, stats, err
	}

	if res == nil {
		return nil, stats, types.ErrNotFound
	}
	var globs protov3.MultiGlobResponse
	err2 := globs.Unmarshal(res.Response)
	if err2 != nil {
		return nil, nil, merry.Wrap(err2)
	}

	return &globs, stats, nil
}

func (c *ClientProtoV3Group) Info(ctx context.Context, request *protov3.MultiMetricsInfoRequest) (*protov3.ZipperInfoResponse, *types.Stats, merry.Error) {
	logger := c.logger.With(zap.String("type", "info"), zap.String("request", request.String()))
	stats := &types.Stats{}
	rewrite, _ := url.Parse("http://127.0.0.1/metrics/find/")

	v := url.Values{
		"format": []string{format},
	}
	rewrite.RawQuery = v.Encode()

	res, err := c.httpQuery.DoQuery(ctx, logger, rewrite.RequestURI(), types.MultiMetricsInfoV3{*request})
	if err != nil {
		return nil, stats, err
	}

	if res == nil {
		return nil, stats, types.ErrNoResponseFetched
	}
	var infos protov3.MultiMetricsInfoResponse
	err2 := infos.Unmarshal(res.Response)
	if err2 != nil {
		return nil, nil, merry.Wrap(err2)
	}

	stats.MemoryUsage = int64(infos.Size())

	r := &protov3.ZipperInfoResponse{
		Info: map[string]protov3.MultiMetricsInfoResponse{
			c.Name(): infos,
		},
	}

	return r, stats, nil
}

func (c *ClientProtoV3Group) List(ctx context.Context) (*protov3.ListMetricsResponse, *types.Stats, merry.Error) {
	return nil, nil, types.ErrNotImplementedYet
}
func (c *ClientProtoV3Group) Stats(ctx context.Context) (*protov3.MetricDetailsResponse, *types.Stats, merry.Error) {
	return nil, nil, types.ErrNotImplementedYet
}

func (c *ClientProtoV3Group) doTagQuery(ctx context.Context, isTagName bool, query string, limit int64) ([]string, merry.Error) {
	logger := c.logger
	var rewrite *url.URL
	if isTagName {
		logger = logger.With(zap.String("sub_type", "tagName"))
		rewrite, _ = url.Parse("http://127.0.0.1/tags/autoComplete/tags")
	} else {
		logger = logger.With(zap.String("sub_type", "tagValues"))
		rewrite, _ = url.Parse("http://127.0.0.1/tags/autoComplete/values")
	}

	var r []string

	rewrite.RawQuery = query
	res, e := c.httpQuery.DoQuery(ctx, logger, rewrite.RequestURI(), nil)
	if e != nil {
		return r, e
	}

	err := json.Unmarshal(res.Response, &r)
	if err != nil {
		return r, merry.Wrap(err)
	}

	logger.Debug("got client response",
		zap.Any("r", r),
	)

	return r, nil
}

func (c *ClientProtoV3Group) TagNames(ctx context.Context, query string, limit int64) ([]string, merry.Error) {
	return c.doTagQuery(ctx, true, query, limit)
}

func (c *ClientProtoV3Group) TagValues(ctx context.Context, query string, limit int64) ([]string, merry.Error) {
	return c.doTagQuery(ctx, false, query, limit)
}

func (c *ClientProtoV3Group) ProbeTLDs(ctx context.Context) ([]string, merry.Error) {
	logger := c.logger.With(zap.String("function", "prober"))
	req := &protov3.MultiGlobRequest{
		Metrics: []string{"*"},
	}

	logger.Debug("doing request",
		zap.Strings("request", req.Metrics),
	)

	res, _, err := c.Find(ctx, req)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return nil, err
	}
	var tlds []string
	for _, m := range res.Metrics {
		for _, v := range m.Matches {
			tlds = append(tlds, v.Path)
		}
	}

	logger.Debug("will return data",
		zap.Strings("tlds", tlds),
	)

	return tlds, nil
}
