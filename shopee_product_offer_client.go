// Package shopee provides a standalone client to call Shopee's GraphQL GetProductOffer (by itemID).
// Copy this single file into another service; it has no dependencies on affiliate-service internals.
// It covers Shopee's SHA256 auth and returns the raw GraphQL response.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultProductOfferSortType = 5
	defaultTimeout              = 30 * time.Second
)

// getProductOfferV2ByItemGraphQL is the full query + fragment used by Shopee's productOfferV2 (by itemId).
// Int64 scalars (itemId) are sent as string in variables.
const getProductOfferV2ByItemGraphQL = `query GetProductOfferV2ByItem ($itemId: Int64, $page: Int, $limit: Int, $sortType: Int) {
  productOfferV2(itemId: $itemId, page: $page, limit: $limit, sortType: $sortType) {
    nodes {
      itemId
      commissionRate
      appExistRate
      appNewRate
      webExistRate
      webNewRate
      commission
      price
      sales
      imageUrl
      productName
      shopName
      productLink
      offerLink
      periodEndTime
      periodStartTime
      priceMin
      priceMax
      productCatIds
      ratingStar
      priceDiscountRate
      shopId
      shopType
      sellerCommissionRate
      shopeeCommissionRate
    }
    pageInfo {
      page
      limit
      hasNextPage
      scrollId
    }
  }
}`

// getProductOfferV2ByKeywordGraphQL is the query for productOfferV2 by keyword (no sortType).
const getProductOfferV2ByKeywordGraphQL = `query GetProductOfferV2ByKeyword ($keyword: String, $page: Int, $limit: Int) {
  productOfferV2(keyword: $keyword, page: $page, limit: $limit) {
    nodes {
      itemId
      commissionRate
      appExistRate
      appNewRate
      webExistRate
      webNewRate
      commission
      price
      sales
      imageUrl
      productName
      shopName
      productLink
      offerLink
      periodEndTime
      periodStartTime
      priceMin
      priceMax
      productCatIds
      ratingStar
      priceDiscountRate
      shopId
      shopType
      sellerCommissionRate
      shopeeCommissionRate
    }
    pageInfo {
      page
      limit
      hasNextPage
      scrollId
    }
  }
}`

// ProductOfferClientConfig configures the Shopee Product Offer client.
type ProductOfferClientConfig struct {
	// BaseURL is the Shopee GraphQL endpoint (e.g. https://partner.shopeemobile.com/graphql or your env-specific URL).
	BaseURL string
	// AppID is the Shopee partner app ID (used in SHA256 auth).
	AppID string
	// Secret is the Shopee partner secret (used to sign the request body).
	Secret string
	// Timeout for HTTP requests. If zero, defaultTimeout is used.
	Timeout time.Duration
}

// ProductOfferClient calls Shopee's GetProductOffer GraphQL API with auth.
type ProductOfferClient struct {
	baseURL string
	client  *http.Client
	appID   string
	secret  string
}

// NewProductOfferClient creates a client that signs requests with Shopee's SHA256 auth.
func NewProductOfferClient(cfg ProductOfferClientConfig) *ProductOfferClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	transport := &shopeeAuthTransport{
		appID:   cfg.AppID,
		secret:  cfg.Secret,
		wrapped: http.DefaultTransport,
	}
	return &ProductOfferClient{
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		appID:   cfg.AppID,
		secret:  cfg.Secret,
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// shopeeAuthTransport implements http.RoundTripper and adds Shopee's Authorization header.
// Signature: SHA256(appID + timestamp + body + secret), lowercase hex.
// Header: SHA256 Credential=<appID>, Timestamp=<ts>, Signature=<sig>
type shopeeAuthTransport struct {
	appID   string
	secret  string
	wrapped http.RoundTripper
}

func (t *shopeeAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	defer body.Close()
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	payload := string(bodyBytes)
	ts := time.Now().Unix()
	sig := t.sign(payload, ts)
	req.Header.Set("Authorization", fmt.Sprintf("SHA256 Credential=%s, Timestamp=%d, Signature=%s", t.appID, ts, sig))
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(bodyBytes)), nil }
	return t.wrapped.RoundTrip(req)
}

func (t *shopeeAuthTransport) sign(payload string, timestamp int64) string {
	signatureFactor := fmt.Sprintf("%s%d%s%s", t.appID, timestamp, payload, t.secret)
	hash := sha256.Sum256([]byte(signatureFactor))
	return strings.ToLower(hex.EncodeToString(hash[:]))
}

// GetProductOfferRequest is the input for GetProductOffer.
type GetProductOfferRequest struct {
	// ItemID is the Shopee product item ID (required).
	ItemID string
	// Page for pagination (0-based). Optional.
	Page int
	// Limit max items (default 10 if 0).
	Limit int
	// SortType (default 5 if 0).
	SortType int
}

// GetProductOffer calls Shopee's productOfferV2 GraphQL query by itemId and returns the raw JSON response body.
// The response is the full GraphQL response: {"data": {...}, "errors": [...] (if any)}.
// You can json.Unmarshal into a struct or use as-is.
func (c *ProductOfferClient) GetProductOffer(ctx context.Context, req GetProductOfferRequest) (rawResponse json.RawMessage, err error) {
	if req.ItemID == "" {
		return nil, fmt.Errorf("itemID is required")
	}
	limit := req.Limit
	if limit == 0 {
		limit = 10
	}
	sortType := req.SortType
	if sortType == 0 {
		sortType = defaultProductOfferSortType
	}
	// Shopee Int64 scalar is sent as string in GraphQL variables.
	variables := map[string]interface{}{
		"itemId":   req.ItemID,
		"page":     req.Page,
		"limit":    limit,
		"sortType": sortType,
	}
	body := map[string]interface{}{
		"query":     getProductOfferV2ByItemGraphQL,
		"variables": variables,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// GetBody so auth transport can re-read the body for signing
	httpReq.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(bodyBytes)), nil }

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawResponse, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return rawResponse, fmt.Errorf("shopee API returned status %d: %s", resp.StatusCode, string(rawResponse))
	}
	return rawResponse, nil
}

// GetProductOfferByItemID is a convenience that takes itemID as int64 and returns raw JSON.
func (c *ProductOfferClient) GetProductOfferByItemID(ctx context.Context, itemID int64, page, limit, sortType int) (json.RawMessage, error) {
	return c.GetProductOffer(ctx, GetProductOfferRequest{
		ItemID:   strconv.FormatInt(itemID, 10),
		Page:     page,
		Limit:    limit,
		SortType: sortType,
	})
}

// GetProductOfferByKeywordRequest is the input for GetProductOfferByKeyword.
type GetProductOfferByKeywordRequest struct {
	Keyword string
	Page    int
	Limit   int
}

// GetProductOfferByKeyword calls Shopee's productOfferV2 GraphQL query by keyword (no sortType) and returns the raw JSON response.
func (c *ProductOfferClient) GetProductOfferByKeyword(ctx context.Context, req GetProductOfferByKeywordRequest) (json.RawMessage, error) {
	if req.Keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}
	limit := req.Limit
	if limit == 0 {
		limit = 10
	}
	variables := map[string]interface{}{
		"keyword": req.Keyword,
		"page":   req.Page,
		"limit":  limit,
	}
	body := map[string]interface{}{
		"query":     getProductOfferV2ByKeywordGraphQL,
		"variables": variables,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(bodyBytes)), nil }

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return rawResponse, fmt.Errorf("shopee API returned status %d: %s", resp.StatusCode, string(rawResponse))
	}
	return rawResponse, nil
}
