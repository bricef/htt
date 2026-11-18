package dates

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AtForms names the forms ParseAt reads, for help and errors.
const AtForms = `HH:MM for today, "yesterday HH:MM" or "YYYY-MM-DD HH:MM", on the 24-hour clock, or ISO 8601's YYYY-MM-DDTHH:MM, with :SS and a zone (Z, +05:30) if you like`

var (
	clockPattern = regexp.MustCompile(`^([0-9]{1,2}):([0-9]{2})$`)
	datePattern  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	// isoPattern is ISO 8601's extended date and time: the date, T,
	// HH:MM, then optionally :SS (and a fraction of a second), then
	// optionally a zone: Z, ±HH:MM or ±HHMM.
	isoPattern = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})T([0-9]{2}):([0-9]{2})(?::([0-9]{2})(?:\.[0-9]+)?)?(Z|[+-][0-9]{2}:?[0-9]{2})?$`)
)

// ParseAt reads the time a time-log entry is logged at, as --at takes
// it: "HH:MM" (or "H:MM") today, "yesterday HH:MM", or
// "YYYY-MM-DD HH:MM", on the 24-hour clock, seconds zero; or an ISO
// 8601 date and time, "YYYY-MM-DDTHH:MM", with seconds if given. now
// anchors today and yesterday (the calendar day before now's), and its
// location is the zone the time is read in; tests inject both. Anything
// else is an error that says which forms work.
//
// An ISO time with a zone (Z, ±HH:MM or ±HHMM) is that instant, as
// a ts: in the log is, and the result is in now's location, wherever
// that puts it in the day; without one it's local time, as ISO 8601
// reads it. A fraction of a second is dropped, as the log's ts: has
// none.
//
// A time later than now is a time all the same: the time log refuses
// it (domain.Timelog.AppendAt), not the parser.
//
// On a day the clocks change, a local time they skip or repeat is
// whatever time.Date makes of it, and time.Date doesn't promise which
// side of the change it picks. It differs by zone: in London 01:30 on
// the day the clocks go forward is 02:30 BST, and 01:30 on the day they
// go back is the second one, in GMT; in New York 02:30 on the day they
// go forward is 01:30 EST, and 01:30 on the day they go back is the
// first, in EDT. Where they go forward at midnight, as in Santiago,
// 00:30 that day is 23:30 the day before. The tests pin these. An ISO
// time with a zone says which it means.
func ParseAt(input string, now time.Time) (time.Time, error) {
	bad := fmt.Errorf("%q isn't a time htt can read (use %s)", input, AtForms)
	y, m, d := now.Date()
	var clock string
	switch fields := strings.Fields(input); {
	case len(fields) == 1 && isoPattern.MatchString(fields[0]):
		at, ok := parseISO(isoPattern.FindStringSubmatch(fields[0]), now.Location())
		if !ok {
			return time.Time{}, bad
		}
		return at, nil
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

// parseISO reads isoPattern's parts (date, hour, minute, second, zone;
// second and zone may be empty) in local, or in the zone if there is
// one, and gives the time in local. ok is false for a date, time or
// zone that doesn't exist (2026-02-30, 24:00, +05:60).
func parseISO(parts []string, local *time.Location) (at time.Time, ok bool) {
	date, err := time.Parse("2006-01-02", parts[1])
	if err != nil {
		return time.Time{}, false
	}
	hour, _ := strconv.Atoi(parts[2])
	minute, _ := strconv.Atoi(parts[3])
	second, _ := strconv.Atoi(parts[4]) // 0 when there are no seconds
	if hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, false
	}
	loc := local
	if zone := parts[5]; zone != "" {
		offset := 0
		if zone != "Z" {
			digits := strings.ReplaceAll(zone[1:], ":", "")
			hours, _ := strconv.Atoi(digits[:2])
			minutes, _ := strconv.Atoi(digits[2:])
			if hours > 23 || minutes > 59 {
				return time.Time{}, false
			}
			offset = (hours*60 + minutes) * 60
			if zone[0] == '-' {
				offset = -offset
			}
		}
		loc = time.FixedZone(zone, offset)
	}
	y, m, d := date.Date()
	return time.Date(y, m, d, hour, minute, second, 0, loc).In(local), true
}
