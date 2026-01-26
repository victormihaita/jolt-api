package jobs

import (
	"context"
	"log"

	"github.com/user/remind-me/backend/internal/repository"
)

// DeviceCleanupJob handles cleaning up stale devices
type DeviceCleanupJob struct {
	deviceRepo *repository.DeviceRepository
}

// NewDeviceCleanupJob creates a new device cleanup job handler
func NewDeviceCleanupJob(deviceRepo *repository.DeviceRepository) *DeviceCleanupJob {
	return &DeviceCleanupJob{
		deviceRepo: deviceRepo,
	}
}

// CleanupStaleDevices removes devices that haven't been seen in the specified number of days
// This should be called by a daily cron job
func (j *DeviceCleanupJob) CleanupStaleDevices(ctx context.Context, days int) (int64, error) {
	log.Printf("[DeviceCleanupJob] Starting cleanup of devices not seen in %d days", days)

	count, err := j.deviceRepo.DeleteStaleDevices(days)
	if err != nil {
		log.Printf("[DeviceCleanupJob] Error cleaning stale devices: %v", err)
		return 0, err
	}

	log.Printf("[DeviceCleanupJob] Cleaned up %d stale devices", count)
	return count, nil
}

// DeviceCleanupResult represents the result of cleaning up stale devices
type DeviceCleanupResult struct {
	Deleted int64 `json:"deleted"`
}
