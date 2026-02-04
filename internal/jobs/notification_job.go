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
	// Find reminders that are due now or past due
	now := time.Now()

	reminders, err := j.reminderRepo.FindDueForNotification(now)
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
		// Determine the notification sound
		// Use custom sound filename if set, otherwise default
		notificationSound := "default"
		if reminder.SoundID != nil && *reminder.SoundID != "" {
			// SoundID is the filename (e.g., "ambient.wav")
			notificationSound = *reminder.SoundID
		}

		// Build the notification payload
		// Title = reminder title, Body = notes (if any)
		body := ""
		if reminder.Notes != nil && *reminder.Notes != "" {
			body = *reminder.Notes
		}

		payload := notification.Payload{
			Title:      reminder.Title,
			Body:       body,
			Sound:      notificationSound,
			Category:   "REMINDER_ACTIONS",
			ReminderID: reminder.ID,
			DueAt:      reminder.DueAt.Format(time.RFC3339),
			Data: map[string]string{
				"type":        "reminder_due",
				"reminder_id": reminder.ID.String(),
				"is_alarm":    strconv.FormatBool(reminder.IsAlarm),
			},
		}

		// Add sound_id to notification data for foreground handling
		if reminder.SoundID != nil && *reminder.SoundID != "" {
			payload.Data["sound_id"] = *reminder.SoundID
		}

		// Add notes to data payload for in-app banner
		if reminder.Notes != nil && *reminder.Notes != "" {
			payload.Data["notes"] = *reminder.Notes
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
