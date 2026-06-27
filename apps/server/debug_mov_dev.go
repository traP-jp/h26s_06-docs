//go:build !production

package main

func debugMov(trigger triggerPayload, fromName string, toName string, result string, reason string, scoreAdded float64) {
	if trigger.Source == "demo" {
		return
	}
	source := trigger.Source
	if source == "" {
		source = "unknown"
	}
	detail := trigger.SourceDetail
	if detail == "" {
		detail = "unknown"
	}
	user := trigger.Usr
	if user == "" {
		user = "unknown"
	}
	from := trigger.From
	if from == "" {
		from = "unknown"
	}
	to := trigger.To
	if to == "" {
		to = "unknown"
	}

	color := logColorYellow
	if result == "applied" {
		color = logColorGreen
	}
	traqLog(
		color,
		"mov",
		"route=%s detail=%q user=%s from=%s fromName=%q to=%s toName=%q result=%s scoreAdded=%.1f reason=%q",
		source,
		detail,
		user,
		from,
		fromName,
		to,
		toName,
		result,
		scoreAdded,
		reason,
	)
}
