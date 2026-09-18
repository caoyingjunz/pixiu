package assistant

import "fmt"

// Providers place stream errors either on the event, under error, or under response.error.
func upstreamStreamError(event map[string]interface{}) string {
	if response, ok := event["response"].(map[string]interface{}); ok {
		if detail, ok := response["incomplete_details"].(map[string]interface{}); ok {
			if reason, ok := detail["reason"].(string); ok && reason != "" {
				return "AI response incomplete: " + reason
			}
		}
		if message := streamErrorMessage(response); message != "" {
			return message
		}
	}
	if message := streamErrorMessage(event); message != "" {
		return message
	}
	return fmt.Sprintf("AI stream reported %v", event["type"])
}

func streamErrorMessage(object map[string]interface{}) string {
	if nested, ok := object["error"].(map[string]interface{}); ok {
		if message := streamErrorMessage(nested); message != "" {
			return message
		}
	}
	if message, ok := object["message"].(string); ok && message != "" {
		return message
	}
	if message, ok := object["error"].(string); ok && message != "" {
		return message
	}
	return ""
}
