package alert

import "time"

type Threshold struct {
	CountGt *int `yaml:"countGt"`
}

type Dedup struct {
	QuietPeriod string `yaml:"quietPeriod"`
}

func (d Dedup) GetQuietPeriod(defaultVal time.Duration) time.Duration {
	if d.QuietPeriod == "" {
		return defaultVal
	}
	v, err := time.ParseDuration(d.QuietPeriod)
	if err != nil {
		return defaultVal
	}
	return v
}

type Alerts struct {
	Channels []string `yaml:"channels"`
}

type Rule struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Index       string    `yaml:"index"`
	Cron        string    `yaml:"cron"`
	TimeWindow  string    `yaml:"timeWindow"`
	QueryString string    `yaml:"queryString"`
	DSL         any       `yaml:"dsl"`
	Threshold   Threshold `yaml:"threshold"`
	Dedup       Dedup     `yaml:"dedup"`
	Alerts      Alerts    `yaml:"alerts"`
	// Severity 用于展示在通知模板中（如 High / Medium / Low），不影响告警逻辑
	Severity string `yaml:"severity"`
}


