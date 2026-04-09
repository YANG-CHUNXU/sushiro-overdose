package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func groupSlotsByDate(slots []Slot, loc *time.Location) map[string][]string {
	grouped := map[string][]string{}
	for _, slot := range slots {
		day, err := parseCompactDate(slot.Date, loc)
		if err != nil {
			continue
		}
		key := formatDateWithWeekday(day)
		grouped[key] = append(grouped[key], fmt.Sprintf("%s-%s", formatCompactTime(slot.Start), formatCompactTime(defaultString(slot.End, slot.Start))))
	}
	for key := range grouped {
		sort.Strings(grouped[key])
	}
	return grouped
}

func fallbackString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
