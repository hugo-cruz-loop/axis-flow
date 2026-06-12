// Package scheduler_test contains black-box tests for the scheduler
// domain types. We deliberately use the external test package
// (`scheduler_test`) instead of `scheduler` so the tests verify the
// PUBLIC API of the Job type — exactly the surface that callers like
// the repository layer and the cron runner depend on.
//
// PR 5A (this file) covers task 5.1: table-driven coverage of
// Job.ValidateSchedule — the cron/interval XOR and the parse check
// that backs the ck_scheduler_jobs_schedule and
// ck_scheduler_jobs_interval CHECK constraints in the V15 migration.
package scheduler_test

import (
	"strings"
	"testing"

	"axis-flow-back/internal/scheduler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSchedule drives Job.ValidateSchedule across the full
// truth table of cron/interval combinations plus the malformed-input
// edge cases. The expected error messages are matched as substrings
// (not exact strings) so the assertions stay stable if the production
// copy changes, while still pinning the contract — the substring is
// chosen so that "schedule", "cron", or "interval" all uniquely
// identify the failure class.
func TestValidateSchedule(t *testing.T) {
	t.Parallel()

	// Helpers: these are tiny because pointer-to-literal in struct
	// literals makes table cases noisy.
	strp := func(s string) *string { return &s }
	intp := func(i int) *int { return &i }

	// Each case builds a Job with only the fields ValidateSchedule
	// inspects (CronExpression / IntervalSeconds) populated. All other
	// fields are zero-valued — ValidateSchedule must not touch them.
	type wantErr struct {
		// mustError is true when ValidateSchedule should return a
		// non-nil error for this case.
		mustError bool
		// contains — if mustError is true — the substring that must
		// appear in the error message.
		contains string
	}

	cases := []struct {
		name     string
		cron     *string
		interval *int
		want     wantErr
	}{
		{
			name: "valid_cron_only_every_five_minutes",
			cron: strp("*/5 * * * *"),
			want: wantErr{mustError: false},
		},
		{
			name:     "valid_interval_only_300_seconds",
			interval: intp(300),
			want:     wantErr{mustError: false},
		},
		{
			name: "invalid_both_cron_and_interval_set",
			cron: strp("*/5 * * * *"), interval: intp(300),
			want: wantErr{mustError: true, contains: "cron"},
		},
		{
			name: "invalid_neither_cron_nor_interval",
			want: wantErr{mustError: true, contains: "schedule"},
		},
		{
			name: "invalid_cron_empty_string_and_interval_nil",
			cron: strp(""),
			want: wantErr{mustError: true, contains: "schedule"},
		},
		{
			name:     "invalid_interval_zero",
			interval: intp(0),
			want:     wantErr{mustError: true, contains: "interval"},
		},
		{
			name:     "invalid_interval_negative",
			interval: intp(-1),
			want:     wantErr{mustError: true, contains: "interval"},
		},
		{
			name: "invalid_cron_not_parseable",
			cron: strp("not a cron"),
			want: wantErr{mustError: true, contains: "cron"},
		},
		{
			// 6-field cron with seconds. The parser installed by
			// ValidateSchedule is the bare 5-field
			// Minute|Hour|Dom|Month|Dow parser (no Second flag),
			// so a 6-field expression is rejected as malformed.
			// This documents the contract: any cron row that
			// managed to slip a 6-field value past the API layer
			// is caught at the domain boundary before persistence.
			name: "invalid_six_field_cron_with_seconds",
			cron: strp("0 */5 * * * *"),
			want: wantErr{mustError: true, contains: "cron"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			job := scheduler.Job{
				JobKey:          "test_job",
				CronExpression:  tc.cron,
				IntervalSeconds: tc.interval,
			}
			err := job.ValidateSchedule()

			if !tc.want.mustError {
				require.NoError(t, err, "ValidateSchedule must accept %+v", job)
				return
			}
			require.Error(t, err, "ValidateSchedule must reject %+v", job)
			assert.True(t,
				strings.Contains(strings.ToLower(err.Error()), tc.want.contains),
				"error %q must mention %q so callers can classify the failure",
				err.Error(), tc.want.contains,
			)
		})
	}
}
