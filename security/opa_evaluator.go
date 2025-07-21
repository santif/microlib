package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/santif/microlib/observability"
)

// OPAEvaluator implements the PolicyEvaluator interface using OPA
type OPAEvaluator struct {
	endpoint string
	client   *http.Client
	logger   observability.Logger
}

// OPARequest represents a request to the OPA server
type OPARequest struct {
	Input map[string]interface{} `json:"input"`
}

// OPAResponse represents a response from the OPA server
type OPAResponse struct {
	Result map[string]interface{} `json:"result"`
}

// NewOPAEvaluator creates a new OPA evaluator
func NewOPAEvaluator(endpoint string, logger observability.Logger) *OPAEvaluator {
	return &OPAEvaluator{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// Evaluate evaluates a policy against an authorization request using OPA
func (e *OPAEvaluator) Evaluate(ctx context.Context, req AuthorizationRequest) (*AuthorizationResponse, error) {
	// Convert the request to a map for OPA
	input := map[string]interface{}{
		"subject": map[string]interface{}{
			"id":          req.Subject.ID,
			"roles":       req.Subject.Roles,
			"permissions": req.Subject.Permissions,
			"attributes":  req.Subject.Attributes,
		},
		"resource": map[string]interface{}{
			"type":       req.Resource.Type,
			"id":         req.Resource.ID,
			"attributes": req.Resource.Attributes,
		},
		"action":  req.Action,
		"context": req.Context,
	}

	// Create OPA request
	opaReq := OPARequest{
		Input: input,
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(opaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OPA request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request to OPA
	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to OPA: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OPA returned non-OK status: %d", resp.StatusCode)
	}

	// Parse response
	var opaResp OPAResponse
	if err := json.NewDecoder(resp.Body).Decode(&opaResp); err != nil {
		return nil, fmt.Errorf("failed to decode OPA response: %w", err)
	}

	// Extract authorization decision
	allowed := false
	reason := "Default deny"
	obligations := make(map[string]interface{})

	if result, ok := opaResp.Result["allow"].(bool); ok {
		allowed = result
	}

	if reasonStr, ok := opaResp.Result["reason"].(string); ok {
		reason = reasonStr
	}

	if oblMap, ok := opaResp.Result["obligations"].(map[string]interface{}); ok {
		obligations = oblMap
	}

	// Create authorization response
	authResp := &AuthorizationResponse{
		Allowed:     allowed,
		Reason:      reason,
		Obligations: obligations,
	}

	return authResp, nil
}

// WithOPAClient sets the HTTP client for the OPA evaluator
func (e *OPAEvaluator) WithOPAClient(client *http.Client) *OPAEvaluator {
	e.client = client
	return e
}
