package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/santif/microlib/observability"
)

// LocalPolicyEvaluator implements the PolicyEvaluator interface using local policies
type LocalPolicyEvaluator struct {
	policyPath string
	logger     observability.Logger
	policies   map[string]Policy
	mutex      sync.RWMutex
}

// Policy represents an authorization policy
type Policy struct {
	// Name is the unique identifier of the policy
	Name string `json:"name"`

	// Description is a human-readable description of the policy
	Description string `json:"description"`

	// Rules are the rules that make up the policy
	Rules []PolicyRule `json:"rules"`

	// DefaultAction is the action to take if no rules match
	DefaultAction string `json:"default_action"`
}

// PolicyRule represents a rule in a policy
type PolicyRule struct {
	// Name is the unique identifier of the rule
	Name string `json:"name"`

	// Description is a human-readable description of the rule
	Description string `json:"description"`

	// Effect is the effect of the rule (allow or deny)
	Effect string `json:"effect"`

	// Subjects are the subjects that the rule applies to
	Subjects []string `json:"subjects"`

	// Resources are the resources that the rule applies to
	Resources []string `json:"resources"`

	// Actions are the actions that the rule applies to
	Actions []string `json:"actions"`

	// Conditions are additional conditions for the rule
	Conditions map[string]interface{} `json:"conditions"`
}

// NewLocalPolicyEvaluator creates a new local policy evaluator
func NewLocalPolicyEvaluator(policyPath string, logger observability.Logger) *LocalPolicyEvaluator {
	evaluator := &LocalPolicyEvaluator{
		policyPath: policyPath,
		logger:     logger,
		policies:   make(map[string]Policy),
	}

	// Load policies if path is provided
	if policyPath != "" {
		if err := evaluator.LoadPolicies(); err != nil {
			logger.Error("Failed to load policies", err,
				observability.NewField("policy_path", policyPath),
			)
		}
	}

	return evaluator
}

// LoadPolicies loads policies from the policy path
func (e *LocalPolicyEvaluator) LoadPolicies() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Clear existing policies
	e.policies = make(map[string]Policy)

	// Check if policy path exists
	if _, err := os.Stat(e.policyPath); os.IsNotExist(err) {
		return fmt.Errorf("policy path does not exist: %s", e.policyPath)
	}

	// Walk the policy directory
	return filepath.Walk(e.policyPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip non-JSON files
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// Read policy file
		data, err := ioutil.ReadFile(path)
		if err != nil {
			e.logger.Error("Failed to read policy file", err,
				observability.NewField("path", path),
			)
			return nil
		}

		// Parse policy
		var policy Policy
		if err := json.Unmarshal(data, &policy); err != nil {
			e.logger.Error("Failed to parse policy file", err,
				observability.NewField("path", path),
			)
			return nil
		}

		// Add policy to map
		e.policies[policy.Name] = policy
		e.logger.Info("Loaded policy",
			observability.NewField("name", policy.Name),
			observability.NewField("path", path),
		)

		return nil
	})
}

// Evaluate evaluates a policy against an authorization request
func (e *LocalPolicyEvaluator) Evaluate(ctx context.Context, req AuthorizationRequest) (*AuthorizationResponse, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Get resource type from request
	resourceType := req.Resource.Type

	// Find policy for resource type
	policy, exists := e.policies[resourceType]
	if !exists {
		// Try to find default policy
		policy, exists = e.policies["default"]
		if !exists {
			return &AuthorizationResponse{
				Allowed: false,
				Reason:  "No applicable policy found",
			}, nil
		}
	}

	// Evaluate rules
	for _, rule := range policy.Rules {
		// Check if rule applies to subject
		subjectMatches := false
		for _, subjectPattern := range rule.Subjects {
			if matchesPattern(string(req.Subject.ID), subjectPattern) ||
				matchesPattern("role:"+joinRoles(req.Subject.Roles), subjectPattern) {
				subjectMatches = true
				break
			}
		}
		if !subjectMatches && len(rule.Subjects) > 0 {
			continue
		}

		// Check if rule applies to resource
		resourceMatches := false
		resourceStr := fmt.Sprintf("%s:%s", req.Resource.Type, req.Resource.ID)
		for _, resourcePattern := range rule.Resources {
			if matchesPattern(resourceStr, resourcePattern) {
				resourceMatches = true
				break
			}
		}
		if !resourceMatches && len(rule.Resources) > 0 {
			continue
		}

		// Check if rule applies to action
		actionMatches := false
		for _, actionPattern := range rule.Actions {
			if matchesPattern(string(req.Action), actionPattern) {
				actionMatches = true
				break
			}
		}
		if !actionMatches && len(rule.Actions) > 0 {
			continue
		}

		// Check conditions
		if !evaluateConditions(rule.Conditions, req) {
			continue
		}

		// Rule matches, return effect
		return &AuthorizationResponse{
			Allowed: rule.Effect == "allow",
			Reason:  rule.Name,
		}, nil
	}

	// No rules matched, use default action
	return &AuthorizationResponse{
		Allowed: policy.DefaultAction == "allow",
		Reason:  "Default action",
	}, nil
}

// matchesPattern checks if a string matches a pattern
func matchesPattern(s, pattern string) bool {
	// Simple wildcard matching
	if pattern == "*" {
		return true
	}

	// Exact match
	if s == pattern {
		return true
	}

	// Prefix match with wildcard
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(s, prefix)
	}

	// Suffix match with wildcard
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(s, suffix)
	}

	return false
}

// joinRoles joins roles into a space-separated string
func joinRoles(roles []Role) string {
	var sb strings.Builder
	for i, role := range roles {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(string(role))
	}
	return sb.String()
}

// evaluateConditions evaluates conditions against a request
func evaluateConditions(conditions map[string]interface{}, req AuthorizationRequest) bool {
	// If no conditions, match
	if len(conditions) == 0 {
		return true
	}

	// Simple condition evaluation
	for key, value := range conditions {
		parts := strings.Split(key, ".")
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "subject":
			if !evaluateSubjectCondition(parts[1:], value, req.Subject) {
				return false
			}
		case "resource":
			if !evaluateResourceCondition(parts[1:], value, req.Resource) {
				return false
			}
		case "context":
			if !evaluateContextCondition(parts[1:], value, req.Context) {
				return false
			}
		}
	}

	return true
}

// evaluateSubjectCondition evaluates a condition on the subject
func evaluateSubjectCondition(path []string, value interface{}, subject Subject) bool {
	if len(path) == 0 {
		return false
	}

	switch path[0] {
	case "id":
		return subject.ID == value
	case "roles":
		for _, role := range subject.Roles {
			if string(role) == value {
				return true
			}
		}
		return false
	case "permissions":
		for _, perm := range subject.Permissions {
			if string(perm) == value {
				return true
			}
		}
		return false
	case "attributes":
		if len(path) < 2 {
			return false
		}
		attrValue, exists := subject.Attributes[path[1]]
		if !exists {
			return false
		}
		return attrValue == value
	}

	return false
}

// evaluateResourceCondition evaluates a condition on the resource
func evaluateResourceCondition(path []string, value interface{}, resource Resource) bool {
	if len(path) == 0 {
		return false
	}

	switch path[0] {
	case "type":
		return resource.Type == value
	case "id":
		return resource.ID == value
	case "attributes":
		if len(path) < 2 {
			return false
		}
		attrValue, exists := resource.Attributes[path[1]]
		if !exists {
			return false
		}
		return attrValue == value
	}

	return false
}

// evaluateContextCondition evaluates a condition on the context
func evaluateContextCondition(path []string, value interface{}, context map[string]interface{}) bool {
	if len(path) == 0 {
		return false
	}

	contextValue, exists := context[path[0]]
	if !exists {
		return false
	}

	if len(path) == 1 {
		return contextValue == value
	}

	// Handle nested context
	if nestedMap, ok := contextValue.(map[string]interface{}); ok {
		return evaluateContextCondition(path[1:], value, nestedMap)
	}

	return false
}
