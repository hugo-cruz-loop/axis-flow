package unit_test

import (
	"testing"

	"axis-flow-back/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateConfig mirrors the function in cmd/server/main.go for unit testing.
func validateConfig(cfg config.Config) error {
	if cfg.Feature.DeleteAllData && cfg.AppEnv == "production" {
		return errDeleteAllDataInProduction
	}
	return nil
}

var errDeleteAllDataInProduction = errMsg("FEATURE_DELETE_ALL_DATA=true is not allowed in production")

type errMsg string

func (e errMsg) Error() string { return string(e) }

func TestStartupGuard_FailsInProductionWithDeleteFeature(t *testing.T) {
	cfg := config.Config{
		AppEnv: "production",
		Feature: config.FeatureConfig{
			DeleteAllData: true,
		},
	}
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FEATURE_DELETE_ALL_DATA")
}

func TestStartupGuard_AllowsDeleteFeatureInNonProduction(t *testing.T) {
	for _, env := range []string{"development", "staging", "test"} {
		t.Run(env, func(t *testing.T) {
			cfg := config.Config{
				AppEnv: env,
				Feature: config.FeatureConfig{
					DeleteAllData: true,
				},
			}
			assert.NoError(t, validateConfig(cfg))
		})
	}
}

func TestStartupGuard_AllowsProductionWithoutDeleteFeature(t *testing.T) {
	cfg := config.Config{
		AppEnv: "production",
		Feature: config.FeatureConfig{
			DeleteAllData: false,
		},
	}
	assert.NoError(t, validateConfig(cfg))
}
