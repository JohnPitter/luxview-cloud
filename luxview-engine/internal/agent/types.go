package agent

// AnalysisResult represents the structured output from the Deploy Agent.
type AnalysisResult struct {
	Suggestions            []Suggestion            `json:"suggestions"`
	Dockerfile             string                  `json:"dockerfile"`
	Port                   int                     `json:"port"`
	Stack                  string                  `json:"stack"`
	EnvHints               []EnvHint               `json:"envHints"`
	Diagnosis              string                  `json:"diagnosis,omitempty"`
	ServiceRecommendations []ServiceRecommendation `json:"serviceRecommendations,omitempty"`
	RequiresAI             bool                    `json:"requiresAi,omitempty"`
}

// Suggestion represents a single recommendation from the agent.
type Suggestion struct {
	Type    string `json:"type"` // "error", "warning", "info"
	Message string `json:"message"`
}

// EnvHint represents an environment variable the application may need.
type EnvHint struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ServiceRecommendation represents a suggestion to migrate an external service
// to a LuxView Cloud managed alternative.
type ServiceRecommendation struct {
	CurrentService     string   `json:"currentService"`
	CurrentEvidence    string   `json:"currentEvidence"`
	RecommendedService string   `json:"recommendedService"` // postgres, redis, mongodb, rabbitmq, s3
	Reason             string   `json:"reason"`
	ManualSteps        []string `json:"manualSteps"`
}
