package requestlog

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"gpt-load/internal/platform/epochms"
	"gpt-load/internal/storage/dbtx"
)

const MaxGroupUsageBatchSize = 100

type GroupUsageReport struct {
	Items map[uint]UsageAggregate
	// Boundary details can fall outside retention, so missing data must not be treated as complete zero usage.
	DataComplete bool
}

// QueryGroupUsage aggregates only the current page's groups, reusing the exact window and final request attribution without calculating trends or leaderboards.
func (service *Service) QueryGroupUsage(ctx context.Context, groupIDs []uint, fromMS, toMS int64) (GroupUsageReport, error) {
	if service == nil || service.db == nil {
		return GroupUsageReport{}, fmt.Errorf("query group usage: database is nil")
	}
	input := UsageQuery{FromMS: fromMS, ToMS: toMS}
	if _, err := validateUsageQuery(input); err != nil {
		return GroupUsageReport{}, err
	}
	if len(groupIDs) == 0 || len(groupIDs) > MaxGroupUsageBatchSize {
		return GroupUsageReport{}, fmt.Errorf("query group usage: invalid batch size")
	}
	result := GroupUsageReport{Items: make(map[uint]UsageAggregate, len(groupIDs))}
	for _, id := range groupIDs {
		if _, exists := result.Items[id]; id == 0 || exists {
			return GroupUsageReport{}, fmt.Errorf("query group usage: invalid or duplicate group")
		}
		result.Items[id] = UsageAggregate{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	err := dbtx.Run(ctx, service.db, dbtx.Options{
		Mode: dbtx.ReadSnapshot, CleanupTimeout: usageRollbackTimeout,
		Operation: "group usage read transaction",
	}, func(connection *gorm.DB) error {
		// Push group conditions into all three sources to avoid reading boundary details for every group before filtering.
		scope := usageWindowScope(connection, input, groupIDs...)
		if err := validateUsageIntegrity(scope.Session(&gorm.Session{}), 0); err != nil {
			return err
		}
		var rows []struct {
			GroupID uint
			UsageAggregate
		}
		if err := scope.Select("group_id, " + usageAggregateSelect).Group("group_id").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := validateUsageAggregate(row.UsageAggregate); err != nil {
				return err
			}
			result.Items[row.GroupID] = row.UsageAggregate
		}
		return nil
	})
	if err != nil {
		return GroupUsageReport{}, fmt.Errorf("query group usage: %w", err)
	}
	hour := epochms.MillisecondsPerHour
	result.DataComplete = (toMS-fromMS > hour && fromMS%hour == 0 && toMS%hour == 0) || service.requestLogWindowRetained(fromMS)
	return result, nil
}
