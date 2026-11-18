package config

import (
	"fmt"

	"github.com/go-co-op/gocron/v2"
)

// validateCronExpression rejects a schedule at boot rather than at first fire. A cron job
// registered with a bad expression fails silently until the scheduler starts, which for a
// once-a-day job means the failure surfaces a day late.
func validateCronExpression(name, expression string) error {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("validate %s: %w", name, err)
	}
	defer func() { _ = scheduler.Shutdown() }()

	if _, err := scheduler.NewJob(gocron.CronJob(expression, false), gocron.NewTask(func() {})); err != nil {
		return fmt.Errorf("invalid %s %q: %w", name, expression, err)
	}
	return nil
}
