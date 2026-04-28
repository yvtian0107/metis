package tools

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"metis/internal/app/itsm/form"
)

var (
	rangeSeparatorPattern     = regexp.MustCompile(`\s*(?:到|至|~|～|-|—)\s*`)
	clockPattern              = regexp.MustCompile(`(\d{1,2})(?::|：|点|时)(\d{1,2})?`)
	relativeDateKeywordRegexp = regexp.MustCompile(`今天|今晚|今早|今晨|今日|明天|明晚|后天|次日|第二天|下午|晚上|早上|上午|中午|晚些时候|下班后|周末`)
	dateTimeValuePattern      = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}`)
	timeSemanticFieldPattern  = regexp.MustCompile(`time|window|period|时段|时间|时间段|时间窗|时间窗口|访问时段|访问时间`)
	explicitTimeRangePattern  = regexp.MustCompile(`((?:今天|今晚|今早|今晨|今日|明天|明晚|后天|次日|第二天|上午|下午|晚上|早上|中午)?\s*\d{1,2}(?::|：|点|时)\d{0,2})\s*(?:到|至|~|～|-)\s*((?:今天|今晚|今早|今晨|今日|明天|明晚|后天|次日|第二天|上午|下午|晚上|早上|中午)?\s*\d{1,2}(?::|：|点|时)\d{0,2})`)
)

type resolvedDateTimeRange struct {
	Start time.Time
	End   time.Time
}

type relativeDateTimeRangeResult struct {
	Resolved           *resolvedDateTimeRange
	NeedsClarification bool
	Reason             string
}

func validateDateTimeRangeField(field FormField, raw any, requestText string) *DraftWarning {
	if !needsDateTimeRangeValidation(field, raw, requestText) {
		return nil
	}

	startText, endText, err := extractDateTimeRangeValue(raw)
	if err != nil {
		return &DraftWarning{
			Type:    "invalid_datetime_range",
			Field:   field.Key,
			Message: fmt.Sprintf("%s 必须是包含完整起止时间的时间段。", field.Label),
		}
	}
	startText = strings.TrimSpace(startText)
	endText = strings.TrimSpace(endText)
	if startText == "" || endText == "" {
		return &DraftWarning{
			Type:    "invalid_datetime_range",
			Field:   field.Key,
			Message: fmt.Sprintf("%s 需要同时包含开始时间和结束时间。", field.Label),
		}
	}

	now := chinaNow()
	startAt, err := parseFlexibleDateTime(startText, chinaLocation())
	if err != nil {
		return &DraftWarning{
			Type:    "invalid_datetime_range",
			Field:   field.Key,
			Message: fmt.Sprintf("%s 的开始时间无法解析，请填写完整日期和时间。", field.Label),
		}
	}
	endAt, err := parseFlexibleDateTime(endText, chinaLocation())
	if err != nil {
		return &DraftWarning{
			Type:    "invalid_datetime_range",
			Field:   field.Key,
			Message: fmt.Sprintf("%s 的结束时间无法解析，请填写完整日期和时间。", field.Label),
		}
	}

	if !endAt.After(startAt) {
		return &DraftWarning{
			Type:    "invalid_datetime_range",
			Field:   field.Key,
			Message: fmt.Sprintf("%s 的结束时间必须晚于开始时间。", field.Label),
		}
	}
	if startAt.Before(now) || endAt.Before(now) {
		return &DraftWarning{
			Type:    "past_datetime_range",
			Field:   field.Key,
			Message: fmt.Sprintf("%s 不能早于当前时间，请确认申请的是未来时间段。", field.Label),
		}
	}

	if result := resolveRelativeDateTimeRange(requestText, now); result.NeedsClarification {
		return &DraftWarning{
			Type:    "ambiguous_datetime_range",
			Field:   field.Key,
			Message: result.Reason,
		}
	} else if result.Resolved != nil {
		if !timesNearlyEqual(result.Resolved.Start, startAt) || !timesNearlyEqual(result.Resolved.End, endAt) {
			return &DraftWarning{
				Type:    "invalid_datetime_range",
				Field:   field.Key,
				Message: fmt.Sprintf("%s 与原始时间描述不一致，请重新确认具体起止时间。", field.Label),
			}
		}
	}

	return nil
}

func normalizeDateTimeSemanticFieldValue(field FormField, raw any, requestText string) (any, bool) {
	if !needsDateTimeRangeValidation(field, raw, requestText) {
		return raw, false
	}
	result := resolveRelativeDateTimeRange(requestText, chinaNow())
	if result.NeedsClarification || result.Resolved == nil {
		return raw, false
	}
	startText := result.Resolved.Start.Format("2006-01-02 15:04:05")
	endText := result.Resolved.End.Format("2006-01-02 15:04:05")
	if field.Type == form.FieldDateRange {
		return map[string]any{
			"start": startText,
			"end":   endText,
		}, true
	}
	if _, ok := raw.(string); ok {
		return fmt.Sprintf("%s ~ %s", startText, endText), true
	}
	return raw, false
}

func needsDateTimeRangeValidation(field FormField, raw any, requestText string) bool {
	if field.Type == form.FieldDateRange {
		if hasTimeRangeFlag(field) {
			return true
		}
		if rangeValue, ok := raw.(map[string]any); ok {
			if hasDateTimeComponent(rangeValue["start"]) || hasDateTimeComponent(rangeValue["end"]) {
				return true
			}
		}
		return relativeDateKeywordRegexp.MatchString(requestText)
	}

	if field.Type != form.FieldText && field.Type != form.FieldTextarea {
		return false
	}
	semantic := strings.ToLower(field.Key + " " + field.Label + " " + field.Description + " " + field.Placeholder)
	if !timeSemanticFieldPattern.MatchString(semantic) {
		return false
	}
	if relativeDateKeywordRegexp.MatchString(requestText) {
		return true
	}
	rawText, ok := raw.(string)
	if !ok {
		return false
	}
	rawText = strings.TrimSpace(rawText)
	return rawText != "" && rangeSeparatorPattern.MatchString(rawText)
}

func hasTimeRangeFlag(field FormField) bool {
	if field.Props == nil {
		return false
	}
	if enabled, ok := field.Props["withTime"].(bool); ok {
		return enabled
	}
	if mode, ok := field.Props["mode"].(string); ok {
		return strings.EqualFold(strings.TrimSpace(mode), "datetime")
	}
	return false
}

func hasDateTimeComponent(value any) bool {
	s, _ := value.(string)
	s = strings.TrimSpace(s)
	return dateTimeValuePattern.MatchString(s)
}

func chinaNow() time.Time {
	return time.Now().In(chinaLocation())
}

func chinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

func parseFlexibleDateTime(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if layout == time.RFC3339 {
			if t, err := time.Parse(layout, value); err == nil {
				return t.In(loc), nil
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime %q", value)
}

func extractDateTimeRangeValue(raw any) (string, string, error) {
	switch v := raw.(type) {
	case map[string]any:
		startText, _ := v["start"].(string)
		endText, _ := v["end"].(string)
		return strings.TrimSpace(startText), strings.TrimSpace(endText), nil
	case string:
		startText, endText, ok := parseTimeRangeString(v)
		if !ok {
			return "", "", fmt.Errorf("invalid textual datetime range")
		}
		return startText, endText, nil
	default:
		return "", "", fmt.Errorf("unsupported datetime range value type")
	}
}

func parseTimeRangeString(value string) (string, string, bool) {
	normalized := normalizeTimeText(value)
	var parts []string
	switch {
	case strings.Contains(normalized, "~"):
		parts = strings.SplitN(normalized, "~", 2)
	case strings.Contains(normalized, "～"):
		parts = strings.SplitN(normalized, "～", 2)
	case strings.Contains(normalized, "到"), strings.Contains(normalized, "至"):
		parts = regexp.MustCompile(`\s*(?:到|至)\s*`).Split(normalized, 2)
	case strings.Contains(normalized, " - "), strings.Contains(normalized, " — "):
		parts = regexp.MustCompile(`\s+(?:-|—)\s+`).Split(normalized, 2)
	default:
		return "", "", false
	}
	if len(parts) != 2 {
		return "", "", false
	}
	startText := strings.TrimSpace(parts[0])
	endText := strings.TrimSpace(parts[1])
	if startText == "" || endText == "" {
		return "", "", false
	}
	return startText, endText, true
}

func timesNearlyEqual(a, b time.Time) bool {
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	return diff < time.Minute
}

func resolveRelativeDateTimeRange(text string, base time.Time) relativeDateTimeRangeResult {
	normalized := normalizeTimeText(text)
	if normalized == "" || !relativeDateKeywordRegexp.MatchString(normalized) {
		return relativeDateTimeRangeResult{}
	}
	parts := []string{}
	if matched := explicitTimeRangePattern.FindStringSubmatch(normalized); len(matched) >= 3 {
		parts = []string{strings.TrimSpace(matched[1]), strings.TrimSpace(matched[2])}
	} else {
		parts = rangeSeparatorPattern.Split(normalized, 2)
	}
	if len(parts) != 2 {
		return relativeDateTimeRangeResult{
			NeedsClarification: true,
			Reason:             "检测到相对时间描述，但缺少明确的开始和结束时刻，请补充完整时间。",
		}
	}

	startClock, ok := parseClockWithContext(parts[0])
	if !ok {
		return relativeDateTimeRangeResult{
			NeedsClarification: true,
			Reason:             "开始时间还不够明确，请提供具体到小时分钟的时间。",
		}
	}
	endClock, ok := parseClockWithContext(parts[1])
	startClock = applyWholeRangeMeridiem(parts[0], normalized, startClock)
	endClock = applyWholeRangeMeridiem(parts[1], normalized, endClock)
	if !ok {
		return relativeDateTimeRangeResult{
			NeedsClarification: true,
			Reason:             "结束时间还不够明确，请提供具体到小时分钟的时间。",
		}
	}

	startOffset, startKnown := relativeDayOffset(parts[0], normalized)
	endOffset, endKnown := relativeDayOffset(parts[1], normalized)
	if !startKnown && !endKnown {
		return relativeDateTimeRangeResult{
			NeedsClarification: true,
			Reason:             "检测到相对时间描述，但无法定位到具体日期，请说明是今天、明天还是后天。",
		}
	}
	if !startKnown {
		startOffset = endOffset
	}
	if !endKnown {
		endOffset = startOffset
	}

	startDate := midnight(base).AddDate(0, 0, startOffset)
	endDate := midnight(base).AddDate(0, 0, endOffset)
	startAt := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), startClock.hour, startClock.minute, 0, 0, base.Location())
	endAt := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), endClock.hour, endClock.minute, 0, 0, base.Location())

	if !endKnown && !endAt.After(startAt) {
		return relativeDateTimeRangeResult{
			NeedsClarification: true,
			Reason:             "结束时间看起来早于开始时间，请确认是否跨天到次日。",
		}
	}
	if !endAt.After(startAt) {
		return relativeDateTimeRangeResult{
			NeedsClarification: true,
			Reason:             "结束时间必须晚于开始时间，请重新确认起止时间。",
		}
	}

	return relativeDateTimeRangeResult{
		Resolved: &resolvedDateTimeRange{Start: startAt, End: endAt},
	}
}

type parsedClock struct {
	hour   int
	minute int
}

func parseClockWithContext(part string) (parsedClock, bool) {
	match := clockPattern.FindStringSubmatch(part)
	if len(match) < 2 {
		return parsedClock{}, false
	}
	hour := mustAtoi(match[1])
	minute := 0
	if len(match) > 2 && match[2] != "" {
		minute = mustAtoi(match[2])
	}
	if hour > 23 || minute > 59 {
		return parsedClock{}, false
	}

	lower := strings.ToLower(part)
	switch {
	case strings.Contains(lower, "下午"), strings.Contains(lower, "晚上"), strings.Contains(lower, "今晚"), strings.Contains(lower, "明晚"):
		if hour < 12 {
			hour += 12
		}
	case strings.Contains(lower, "中午"):
		if hour < 11 {
			hour += 12
		}
	case hour == 24:
		hour = 0
	}
	return parsedClock{hour: hour, minute: minute}, true
}

func applyWholeRangeMeridiem(part, whole string, clock parsedClock) parsedClock {
	if hasExplicitMeridiem(part) || hasExplicitDayAnchor(part) {
		return clock
	}
	lower := strings.ToLower(whole)
	switch {
	case strings.Contains(lower, "下午"), strings.Contains(lower, "晚上"), strings.Contains(lower, "今晚"), strings.Contains(lower, "明晚"):
		if clock.hour < 12 {
			clock.hour += 12
		}
	case strings.Contains(lower, "中午"):
		if clock.hour < 11 {
			clock.hour += 12
		}
	}
	return clock
}

func hasExplicitMeridiem(part string) bool {
	lower := strings.ToLower(part)
	return strings.Contains(lower, "上午") ||
		strings.Contains(lower, "下午") ||
		strings.Contains(lower, "晚上") ||
		strings.Contains(lower, "今晚") ||
		strings.Contains(lower, "明晚") ||
		strings.Contains(lower, "中午")
}

func hasExplicitDayAnchor(part string) bool {
	return strings.Contains(part, "明天") ||
		strings.Contains(part, "后天") ||
		strings.Contains(part, "次日") ||
		strings.Contains(part, "第二天") ||
		strings.Contains(part, "今天") ||
		strings.Contains(part, "今晚") ||
		strings.Contains(part, "今早") ||
		strings.Contains(part, "今晨") ||
		strings.Contains(part, "今日")
}

func relativeDayOffset(part, whole string) (int, bool) {
	switch {
	case strings.Contains(part, "后天"):
		return 2, true
	case strings.Contains(part, "明天"):
		return 1, true
	case strings.Contains(part, "今天"), strings.Contains(part, "今晚"), strings.Contains(part, "今早"), strings.Contains(part, "今晨"), strings.Contains(part, "今日"):
		return 0, true
	case strings.Contains(part, "次日"), strings.Contains(part, "第二天"):
		return 1, true
	}
	switch {
	case strings.Contains(whole, "后天"):
		return 2, true
	case strings.Contains(whole, "明天"), strings.Contains(whole, "明晚"):
		return 1, true
	case strings.Contains(whole, "今天"), strings.Contains(whole, "今晚"), strings.Contains(whole, "今早"), strings.Contains(whole, "今晨"), strings.Contains(whole, "今日"):
		return 0, true
	}
	return 0, false
}

func normalizeTimeText(text string) string {
	text = strings.TrimSpace(text)
	replacer := strings.NewReplacer(
		"，", " ",
		"。", " ",
		"：", ":",
		"－", "-",
		"—", "-",
		"～", "~",
		" ", " ",
	)
	text = replacer.Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func mustAtoi(raw string) int {
	n := 0
	for _, r := range raw {
		n = n*10 + int(r-'0')
	}
	return n
}
