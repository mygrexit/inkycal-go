package builder

import "strings"

// explain turns a module error into advice for someone who has never seen an HTTP
// status code. "" when there is nothing better to say than the error itself.
func explain(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "unknown module"):
		return "This module does not exist (any more): remove the tile."
	case strings.Contains(m, "is missing") || strings.Contains(m, "are missing") || strings.Contains(m, "are required"):
		return "Please fill in the marked fields of this tile."
	case strings.Contains(m, "http 401") || strings.Contains(m, "http 403"):
		return "Access denied: check the key or token you entered (copy it again, without spaces)."
	case strings.Contains(m, "http 404"):
		return "The address was not found: check that you copied the whole address."
	case strings.Contains(m, "http 502") || strings.Contains(m, "http 503") || strings.Contains(m, "http 504"):
		return "The service is not available right now. The panel tries again at the next update."
	case strings.Contains(m, "http 429"):
		return "The service is busy or you asked too often. It will work again later."
	case strings.Contains(m, "got html") || strings.Contains(m, "not an icalendar"):
		return "The address shows a web page (often a login page), not a calendar. Use the secret address in iCal format."
	case strings.Contains(m, "no such host") || strings.Contains(m, "connection refused") || strings.Contains(m, "network is unreachable"):
		return "No connection: check the address and the internet connection."
	case strings.Contains(m, "deadline exceeded") || strings.Contains(m, "timeout"):
		return "The service took too long to answer. Try again with \"Refresh data\"."
	case strings.Contains(m, "invalid json") || strings.Contains(m, "cannot unmarshal"):
		return "The address does not return the data this tile expects."
	case strings.Contains(m, "too small"):
		return "The tile is too small for this module: drag it bigger."
	case strings.Contains(m, "no such file"):
		return "The file does not exist on this computer."
	}
	return ""
}
