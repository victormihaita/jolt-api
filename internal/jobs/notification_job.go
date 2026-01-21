package jobs

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/user/remind-me/backend/internal/notification"
	"github.com/user/remind-me/backend/internal/repository"
)

// NotificationJob handles sending notifications for due reminders
type NotificationJob struct {
	reminderRepo *repository.ReminderRepository
	dispatcher   *notification.Dispatcher
}

// NewNotificationJob creates a new notification job handler
func NewNotificationJob(
	reminderRepo *repository.ReminderRepository,
	dispatcher *notification.Dispatcher,
) *NotificationJob {
	return &NotificationJob{
		reminderRepo: reminderRepo,
		dispatcher:   dispatcher,
	}
}

// ProcessDueReminders finds reminders that are due and sends notifications
// This should be called by a cron job every minute
func (j *NotificationJob) ProcessDueReminders(ctx context.Context) (int, error) {
	// Look for reminders due within the next minute
	windowEnd := time.Now().Add(1 * time.Minute)

	reminders, err := j.reminderRepo.FindDueForNotification(windowEnd)
	if err != nil {
		log.Printf("[NotificationJob] Error finding due reminders: %v", err)
		return 0, err
	}

	if len(reminders) == 0 {
		log.Printf("[NotificationJob] No reminders due for notification")
		return 0, nil
	}

	log.Printf("[NotificationJob] Found %d reminders due for notification", len(reminders))

	sentCount := 0
	for _, reminder := range reminders {
		// Build the notification payload
		payload := notification.Payload{
			Title:      "Reminder",
			Body:       reminder.Title,
			Sound:      "default",
			Category:   "REMINDER_ACTIONS",
			ReminderID: reminder.ID,
			DueAt:      reminder.DueAt.Format(time.RFC3339),
			Data: map[string]string{
				"type":        "reminder_due",
				"reminder_id": reminder.ID.String(),
				"is_alarm":    strconv.FormatBool(reminder.IsAlarm),
			},
		}

		// Add notes to body if present
		if reminder.Notes != nil && *reminder.Notes != "" {
			payload.Body = reminder.Title + "\n" + *reminder.Notes
		}

		// Set alarm category if this is an alarm
		if reminder.IsAlarm {
			payload.Category = "ALARM_ACTIONS"
			payload.Data["type"] = "alarm_due"
		}

		// Send notification to all user's devices
		err := j.dispatcher.SendToUser(ctx, reminder.UserID, payload)
		if err != nil {
			log.Printf("[NotificationJob] Failed to send notification for reminder %s: %v", reminder.ID, err)
			// Continue with other reminders even if one fails
			continue
		}

		// Mark the reminder as notification sent
		err = j.reminderRepo.MarkNotificationSent(reminder.ID)
		if err != nil {
			log.Printf("[NotificationJob] Failed to mark notification sent for reminder %s: %v", reminder.ID, err)
			// Continue anyway - the notification was sent
		}

		sentCount++
		log.Printf("[NotificationJob] Sent notification for reminder %s to user %s", reminder.ID, reminder.UserID)
	}

	log.Printf("[NotificationJob] Completed: sent %d/%d notifications", sentCount, len(reminders))
	return sentCount, nil
}

// ProcessDueRemindersResult represents the result of processing due reminders
type ProcessDueRemindersResult struct {
	Processed int `json:"processed"`
	Sent      int `json:"sent"`
	Errors    int `json:"errors"`
}
