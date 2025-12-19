package utils

import (
	"testing"

	sleepodv1alpha1 "github.com/shaygef123/SleePod-controller/api/v1alpha1"
)

func TestConfigHashCalc(t *testing.T) {
	// Base configuration
	baseConfig := sleepodv1alpha1.ResourceSleepParams{
		Name:      "test-resource",
		Namespace: "default",
		Kind:      "Deployment",
		SleepAt:   "20:00",
		WakeAt:    "08:00",
		Timezone:  "UTC",
	}

	t.Run("Consistency", func(t *testing.T) {
		hash1 := ConfigHashCalc(baseConfig)
		hash2 := ConfigHashCalc(baseConfig)
		if hash1 != hash2 {
			t.Errorf("Hash should be consistent: got %s != %s", hash1, hash2)
		}
	})

	t.Run("Uniqueness", func(t *testing.T) {
		hash1 := ConfigHashCalc(baseConfig)

		// Change one field
		diffConfig := baseConfig
		diffConfig.SleepAt = "21:00"

		hash2 := ConfigHashCalc(diffConfig)
		if hash1 == hash2 {
			t.Errorf("Hash should change when input changes: got %s == %s", hash1, hash2)
		}
	})

	t.Run("Length", func(t *testing.T) {
		hash := ConfigHashCalc(baseConfig)
		if len(hash) != 12 {
			t.Errorf("Hash length should be 12 characters (6 bytes hex encoded): got %d", len(hash))
		}
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		emptyConfig := sleepodv1alpha1.ResourceSleepParams{}
		hash := ConfigHashCalc(emptyConfig)
		if len(hash) == 0 {
			t.Error("Hash should not be empty even for empty config")
		}
		if len(hash) != 12 {
			t.Errorf("Hash length should be 12: got %d", len(hash))
		}
	})
}
