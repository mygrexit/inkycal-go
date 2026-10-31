package builder

import "inkycal/internal/problem"

// advice turns each kind of error into a sentence for someone who has never seen an
// HTTP status code. Kinds missing here get no advice: the error itself is all there is.
var advice = map[problem.Kind]string{
	problem.UnknownModule: "This module does not exist (any more): remove the tile.",
	problem.Missing:       "Please fill in the marked fields of this tile.",
	problem.BadSetting:    "A setting of this tile has the wrong format: enter it again and save.",
	problem.Style:         "This style is invalid: choose the font, size and colours again.",
	problem.Placement:     "A tile has no place on the grid or overlaps another one: move or resize it.",
	problem.TooSmall:      "The tile is too small for this module: drag it bigger.",
	problem.Padding:       "The inner margin is larger than the tile: make the margin smaller.",
	problem.Denied:        "Access denied: check the key or token you entered (copy it again, without spaces).",
	problem.NotFound:      "The address or file was not found: check that you copied the whole address.",
	problem.Unavailable:   "The service is busy or not available right now. The panel tries again at the next update.",
	problem.Offline:       "No connection: check the address and the internet connection.",
	problem.Timeout:       "The service took too long to answer. Try again with \"Refresh data\".",
	problem.NotCalendar:   "The address shows a web page (often a login page), not a calendar. Use the secret address in iCal format.",
	problem.BadResponse:   "The address does not return the data this tile expects.",
}

// explain is the advice for err, "" when there is nothing better to say than the error.
func explain(err error) string { return advice[problem.Classify(err)] }
