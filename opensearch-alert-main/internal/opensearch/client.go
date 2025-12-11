package opensearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"opensearch-alert/pkg/types"
	"time"

	"github.com/sirupsen/logrus"
)

// Client OpenSearch 客户端
type Client struct {
	config     types.OpenSearchConfig
	httpClient *http.Client
	baseURL    string
	logger     *logrus.Logger
}

// NewClient 创建新的 OpenSearch 客户端
func NewClient(config types.OpenSearchConfig) *Client {
	baseURL := fmt.Sprintf("%s://%s:%d", config.Protocol, config.Host, config.Port)

	// 创建 HTTP 客户端，根据配置决定是否验证证书
	httpClient := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	// 如果配置了不验证证书，则跳过 TLS 验证
	if !config.VerifyCerts {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	// 创建日志器
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Client{
		config:     config,
		httpClient: httpClient,
		baseURL:    baseURL,
		logger:     logger,
	}
}

// Search 执行搜索查询
func (c *Client) Search(ctx context.Context, index string, query map[string]interface{}) (*types.OpenSearchResponse, error) {
	url := fmt.Sprintf("%s/%s/_search", c.baseURL, index)
	c.logger.Debugf("执行 OpenSearch 查询: %s", url)

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("序列化查询失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(queryBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Errorf("OpenSearch 查询请求失败: %v", err)
		return nil, fmt.Errorf("执行请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Errorf("OpenSearch 查询失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("OpenSearch 查询失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Errorf("读取 OpenSearch 响应失败: %v", err)
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var response types.OpenSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		c.logger.Errorf("解析 OpenSearch 响应失败: %v", err)
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	c.logger.Debugf("OpenSearch 查询成功，匹配 %d 条记录", response.Hits.Total.Value)
	return &response, nil
}

// Count 执行计数查询
func (c *Client) Count(ctx context.Context, index string, query map[string]interface{}) (int, error) {
	url := fmt.Sprintf("%s/%s/_count", c.baseURL, index)

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("序列化查询失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(queryBytes))
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("执行请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("OpenSearch 计数查询失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}

	var countResp struct {
		Count int `json:"count"`
	}

	if err := json.Unmarshal(body, &countResp); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	return countResp.Count, nil
}

// Index 索引文档
func (c *Client) Index(ctx context.Context, index string, id string, doc interface{}) error {
	url := fmt.Sprintf("%s/%s/_doc/%s", c.baseURL, index, id)

	docBytes, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化文档失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(docBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("执行请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenSearch 索引失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	return nil
}

// IndexDocument 索引文档（自动生成ID）
func (c *Client) IndexDocument(ctx context.Context, index string, doc interface{}) error {
	url := fmt.Sprintf("%s/%s/_doc", c.baseURL, index)

	docBytes, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化文档失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(docBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("执行请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenSearch 索引失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	return nil
}

// BuildTimeRangeQuery 构建时间范围查询
func (c *Client) BuildTimeRangeQuery(rule types.AlertRule, bufferTime int) map[string]interface{} {
	now := time.Now()
	// 只使用规则的时间窗口，不使用bufferTime
	startTime := now.Add(-time.Duration(rule.Timeframe) * time.Second)

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"range": map[string]interface{}{
							"@timestamp": map[string]interface{}{
								"gte": startTime.Format(time.RFC3339),
								"lte": now.Format(time.RFC3339),
							},
						},
					},
				},
			},
		},
		"size": 100, // 减少返回结果数量，只用于告警判断
		"sort": []map[string]interface{}{
			{
				"@timestamp": map[string]interface{}{
					"order": "desc",
				},
			},
		},
	}

	// 合并规则查询条件
	if rule.Query != nil {
		if boolQuery, ok := query["query"].(map[string]interface{})["bool"].(map[string]interface{}); ok {
			if must, ok := boolQuery["must"].([]map[string]interface{}); ok {
				must = append(must, rule.Query)
				boolQuery["must"] = must
			}
		}
	}

	return query
}

// HealthCheck 检查 OpenSearch 连接状态
func (c *Client) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/_cluster/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建健康检查请求失败: %w", err)
	}

	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("执行健康检查请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenSearch 健康检查失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取健康检查响应失败: %w", err)
	}

	var healthResp struct {
		Status        string `json:"status"`
		ClusterName   string `json:"cluster_name"`
		NumberOfNodes int    `json:"number_of_nodes"`
	}

	if err := json.Unmarshal(body, &healthResp); err != nil {
		return fmt.Errorf("解析健康检查响应失败: %w", err)
	}

	if healthResp.Status == "red" {
		return fmt.Errorf("OpenSearch 集群状态为红色，可能存在问题")
	}

	return nil
}

// TestConnection 测试 OpenSearch 连接
func (c *Client) TestConnection(ctx context.Context) error {
	// 首先进行健康检查
	if err := c.HealthCheck(ctx); err != nil {
		return fmt.Errorf("健康检查失败: %w", err)
	}

	// 尝试执行一个简单的搜索查询
	url := fmt.Sprintf("%s/_search", c.baseURL)
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": 0,
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("序列化测试查询失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(queryBytes))
	if err != nil {
		return fmt.Errorf("创建测试查询请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("执行测试查询失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("测试查询失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	return nil
}
