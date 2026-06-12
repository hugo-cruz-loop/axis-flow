package config

// ReportConfig holds runtime settings for the ReportBro report-generation
// service.
//
// Spec reference: 14_ReportBro_Service_Spec §Configuration.
type ReportConfig struct {
	// SigningKey is the HMAC-SHA256 secret used to sign download URLs.
	// Loaded from REPORT_SIGNING_KEY. Required. NEVER log this value.
	SigningKey string

	// TokenTTLSeconds is the lifetime of a signed download token in seconds.
	// Loaded from REPORT_TOKEN_TTL_SECONDS. Default: 180.
	TokenTTLSeconds int
}

// loadReportConfig builds a ReportConfig from the current process environment.
// It appends a fatal-level error message to errs if REPORT_SIGNING_KEY is
// missing (the caller is expected to check errs and abort startup).
func loadReportConfig(errs *[]string) ReportConfig {
	signingKey := getenvDefault("REPORT_SIGNING_KEY", "")
	if signingKey == "" {
		*errs = append(*errs, "REPORT_SIGNING_KEY is required")
	}

	return ReportConfig{
		SigningKey:       signingKey,
		TokenTTLSeconds: parseIntDefault("REPORT_TOKEN_TTL_SECONDS", 180),
	}
}
