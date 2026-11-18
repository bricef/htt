package dates

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AtForms names the forms ParseAt reads, for help and errors.
const AtForms = `HH:MM for today, "yesterday HH:MM" or "YYYY-MM-DD HH:MM", on the 24-hour clock`

var (
	clockPattern = regexp.MustCompile(`^([0-9]{1,2}):([0-9]{2})$`)
	datePattern  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

// ParseAt reads the time a time-log entry is logged at, as --at takes
// it: "HH:MM" (or "H:MM") today, "yesterday HH:MM", or
// "YYYY-MM-DD HH:MM", on the 24-hour clock, seconds zero. now anchors
// today and yesterday (the calendar day before now's), and its location
// is the zone the time is read in; tests inject both. Anything else is
// an error that says which forms work.
//
// A time later than now is a time all the same: the time log refuses
// it (domain.Timelog.AppendAt), not the parser.
//
// On a day the clocks change, a time they skip or repeat is whatever
// time.Date makes of it, and time.Date doesn't promise which side of
// the change it picks. It differs by zone: in London 01:30 on the day
// the clocks go forward is 02:30 BST, and 01:30 on the day they go back
// is the second one, in GMT; in New York 02:30 on the day they go
// forward is 01:30 EST, and 01:30 on the day they go back is the first,
// in EDT. Where they go forward at midnight, as in Santiago, 00:30 that
// day is 23:30 the day before. The tests pin these.
func ParseAt(input string, now time.Time) (time.Time, error) {
	bad := fmt.Errorf("%q isn't a time htt can read (use %s)", input, AtForms)
	y, m, d := now.Date()
	var clock string
	switch fields := strings.Fields(input); {
	case len(fields) == 1:
		clock = fields[0]
	case len(fields) == 2 && fields[0] == "yesterday":
		d-- // time.Date takes day 0 as the last of the month before
		clock = fields[1]
	case len(fields) == 2 && datePattern.MatchString(fields[0]):
		// Parsed in UTC for its year, month and day only: a zone whose
		// clocks go forward at midnight has no midnight that day.
		date, err := time.Parse("2006-01-02", fields[0])
		if err != nil {
			return time.Time{}, bad
		}
		y, m, d = date.Date()
		clock = fields[1]
	default:
		return time.Time{}, bad
	}
	parts := clockPattern.FindStringSubmatch(clock)
	if parts == nil {
		return time.Time{}, bad
	}
	hour, _ := strconv.Atoi(parts[1])
	minute, _ := strconv.Atoi(parts[2])
	if hour > 23 || minute > 59 {
		return time.Time{}, bad
	}
	return time.Date(y, m, d, hour, minute, 0, 0, now.Location()), nil
}
