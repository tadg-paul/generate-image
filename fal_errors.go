// ABOUTME: Multi-envelope error parser for FAL responses.
// ABOUTME: Walks gateway-shape, FastAPI-detail-shape, and other variants.

package main

import (
	"encoding/json"
	"strings"
)

const falErrorBodyTruncateAt = 500

// formatFALErrorBody converts a raw FAL error response body into a clean,
// human-readable single-line-ish message.
//
// FAL is not uniform. The gateway returns one envelope:
//
//	{"error": {"type": "...", "message": "...", "request_id": "..."}}
//
// Individual model endpoints (notably the kontext family) return FastAPI's
// validation-error envelope instead:
//
//	{"detail": [{"type": "missing", "loc": ["body", "image_url"], "msg": "Field required", "input": {...echoed request...}}]}
//
// Both envelopes can also degrade to plain strings, dicts without the
// expected keys, HTML pages from upstream proxies, etc. The walker tries
// known shapes first and falls back to a truncated raw body so the user's
// terminal scrollback is never lost to a multi-megabyte base64 payload.
//
// Ported in spirit from storyboard-gen/src/storyboard_gen/errors.py::clean_api_error.
func formatFALErrorBody(body []byte) string {
	if len(body) == 0 {
		return "(empty response body)"
	}

	// Try parsing as a JSON object. If parsing fails, fall through to the
	// truncate-raw path -- the body might be an HTML 502, a plaintext
	// message, or otherwise non-JSON.
	var doc map[string]interface{}
	if err := json.Unmarshal(body, &doc); err == nil {
		if msg := extractFALMessage(doc); msg != "" {
			return msg
		}
	}

	// Unknown shape -- truncate so we don't dump multi-MB request echoes.
	if len(body) > falErrorBodyTruncateAt {
		return string(body[:falErrorBodyTruncateAt]) + "... (truncated)"
	}
	return string(body)
}

// extractFALMessage walks the known envelope shapes of a parsed JSON object
// and returns the most informative message it can build, or "" if none of
// the known shapes match.
func extractFALMessage(doc map[string]interface{}) string {
	// Shape 1: FAL gateway -- {"error": {"type", "message", "request_id"}}.
	if errObj, ok := doc["error"].(map[string]interface{}); ok {
		msg, _ := errObj["message"].(string)
		typ, _ := errObj["type"].(string)
		reqID, _ := errObj["request_id"].(string)
		if msg != "" {
			out := msg
			if typ != "" {
				out = typ + ": " + out
			}
			if reqID != "" {
				out += " (request_id: " + reqID + ")"
			}
			return out
		}
	}

	// Shape 2: Direct top-level message -- {"message": "..."}.
	if msg, ok := doc["message"].(string); ok && msg != "" {
		return msg
	}

	// Shape 3: FastAPI detail.
	if detail, ok := doc["detail"]; ok {
		if msg := extractFastAPIDetail(detail); msg != "" {
			return msg
		}
	}

	return ""
}

// extractFastAPIDetail handles the two shapes of FastAPI's `detail` field.
// String form: `{"detail": "Forbidden"}`. List form: `{"detail": [{"msg": "...", "loc": [...]}]}`.
func extractFastAPIDetail(detail interface{}) string {
	switch d := detail.(type) {
	case string:
		return d
	case []interface{}:
		messages := make([]string, 0, len(d))
		for _, item := range d {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			msg, _ := obj["msg"].(string)
			if msg == "" {
				if m2, ok := obj["message"].(string); ok {
					msg = m2
				}
			}
			if msg == "" {
				continue
			}
			// Include loc when present -- it points at the offending field
			// (e.g. body.image_url) which is the diagnostic the user needs.
			if locField, ok := obj["loc"].([]interface{}); ok && len(locField) > 0 {
				parts := make([]string, 0, len(locField))
				for _, p := range locField {
					switch v := p.(type) {
					case string:
						parts = append(parts, v)
					default:
						// Skip non-string components (FastAPI sometimes
						// includes integer array indices); they add noise
						// without diagnostic value here.
					}
				}
				if len(parts) > 0 {
					msg = msg + ": " + strings.Join(parts, ".")
				}
			}
			messages = append(messages, msg)
		}
		if len(messages) > 0 {
			return strings.Join(messages, "; ")
		}
	}
	return ""
}
