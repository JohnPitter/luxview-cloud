package agent

import (
	"fmt"
	"strings"
)

const DefaultFreeModel = "openrouter/free"

// ResolveFreeModel guarantees that the deploy agent only uses free OpenRouter models.
func ResolveFreeModel(model string) (string, error) {
	normalized := strings.TrimSpace(model)
	if normalized == "" {
		return DefaultFreeModel, nil
	}

	if normalized == DefaultFreeModel || strings.HasSuffix(normalized, ":free") {
		return normalized, nil
	}

	return "", fmt.Errorf("model %q is not a free OpenRouter model", normalized)
}
