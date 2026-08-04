package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression coverage for the gorm tag cleanup: SubscriptionPlan dropped
// gorm:"default:true" on Enabled (now enforced in BeforeCreate) and Task moved
// from the legacy primary_key;AUTO_INCREMENT tag to the GORM v2 primaryKey
// tag. TestMain's SQLite AutoMigrate already proves both models migrate; these
// tests lock the behavior the old tags implied.

func TestSubscriptionPlanCreateDefaultsToEnabled(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{Title: "smoke", DurationUnit: SubscriptionDurationMonth, DurationValue: 1}
	require.NoError(t, DB.Create(plan).Error)

	var stored SubscriptionPlan
	require.NoError(t, DB.First(&stored, plan.Id).Error)
	assert.True(t, stored.Enabled, "a created plan must persist enabled=true, matching the old DB default")
}

func TestTaskPrimaryKeyAutoIncrements(t *testing.T) {
	truncateTables(t)

	first := &Task{TaskID: "smoke-1", Status: TaskStatusNotStart}
	second := &Task{TaskID: "smoke-2", Status: TaskStatusNotStart}
	insertTask(t, first)
	insertTask(t, second)

	require.NotZero(t, first.ID)
	assert.Greater(t, second.ID, first.ID)
}
