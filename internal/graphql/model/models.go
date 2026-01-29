package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/remind-me/backend/internal/models"
)

// Platform enum
type Platform string

const (
	PlatformIOS     Platform = "IOS"
	PlatformAndroid Platform = "ANDROID"
)

func (p Platform) IsValid() bool {
	switch p {
	case PlatformIOS, PlatformAndroid:
		return true
	}
	return false
}

func (p Platform) String() string {
	return string(p)
}

// Priority enum
type Priority string

const (
	PriorityNone   Priority = "NONE"
	PriorityLow    Priority = "LOW"
	PriorityNormal Priority = "NORMAL"
	PriorityHigh   Priority = "HIGH"
)

func (p Priority) IsValid() bool {
	switch p {
	case PriorityNone, PriorityLow, PriorityNormal, PriorityHigh:
		return true
	}
	return false
}

func (p Priority) String() string {
	return string(p)
}

// Convert from internal model Priority to GraphQL Priority
func PriorityFromModel(p models.Priority) Priority {
	switch p {
	case models.PriorityLow:
		return PriorityLow
	case models.PriorityMedium:
		return PriorityNormal // Map Medium to Normal for clients
	case models.PriorityHigh:
		return PriorityHigh
	default:
		return PriorityNone
	}
}

// Convert from GraphQL Priority to internal model Priority
func PriorityToModel(p Priority) models.Priority {
	switch p {
	case PriorityLow:
		return models.PriorityLow
	case PriorityNormal:
		return models.PriorityMedium // Map Normal to Medium internally
	case PriorityHigh:
		return models.PriorityHigh
	default:
		return models.PriorityNone
	}
}

// ReminderStatus enum
type ReminderStatus string

const (
	ReminderStatusActive    ReminderStatus = "ACTIVE"
	ReminderStatusCompleted ReminderStatus = "COMPLETED"
	ReminderStatusSnoozed   ReminderStatus = "SNOOZED"
	ReminderStatusDismissed ReminderStatus = "DISMISSED"
)

func (s ReminderStatus) IsValid() bool {
	switch s {
	case ReminderStatusActive, ReminderStatusCompleted, ReminderStatusSnoozed, ReminderStatusDismissed:
		return true
	}
	return false
}

func (s ReminderStatus) String() string {
	return string(s)
}

func ReminderStatusFromModel(s models.ReminderStatus) ReminderStatus {
	switch s {
	case models.StatusCompleted:
		return ReminderStatusCompleted
	case models.StatusSnoozed:
		return ReminderStatusSnoozed
	case models.StatusDismissed:
		return ReminderStatusDismissed
	default:
		return ReminderStatusActive
	}
}

func ReminderStatusToModel(s ReminderStatus) models.ReminderStatus {
	switch s {
	case ReminderStatusCompleted:
		return models.StatusCompleted
	case ReminderStatusSnoozed:
		return models.StatusSnoozed
	case ReminderStatusDismissed:
		return models.StatusDismissed
	default:
		return models.StatusActive
	}
}

// Frequency enum
type Frequency string

const (
	FrequencyHourly  Frequency = "HOURLY"
	FrequencyDaily   Frequency = "DAILY"
	FrequencyWeekly  Frequency = "WEEKLY"
	FrequencyMonthly Frequency = "MONTHLY"
	FrequencyYearly  Frequency = "YEARLY"
)

func (f Frequency) IsValid() bool {
	switch f {
	case FrequencyHourly, FrequencyDaily, FrequencyWeekly, FrequencyMonthly, FrequencyYearly:
		return true
	}
	return false
}

func (f Frequency) String() string {
	return string(f)
}

func FrequencyFromModel(f models.Frequency) Frequency {
	switch f {
	case models.FrequencyHourly:
		return FrequencyHourly
	case models.FrequencyDaily:
		return FrequencyDaily
	case models.FrequencyWeekly:
		return FrequencyWeekly
	case models.FrequencyMonthly:
		return FrequencyMonthly
	case models.FrequencyYearly:
		return FrequencyYearly
	default:
		return FrequencyDaily
	}
}

func FrequencyToModel(f Frequency) models.Frequency {
	switch f {
	case FrequencyHourly:
		return models.FrequencyHourly
	case FrequencyDaily:
		return models.FrequencyDaily
	case FrequencyWeekly:
		return models.FrequencyWeekly
	case FrequencyMonthly:
		return models.FrequencyMonthly
	case FrequencyYearly:
		return models.FrequencyYearly
	default:
		return models.FrequencyDaily
	}
}

// ChangeAction enum
type ChangeAction string

const (
	ChangeActionCreated ChangeAction = "CREATED"
	ChangeActionUpdated ChangeAction = "UPDATED"
	ChangeActionDeleted ChangeAction = "DELETED"
)

func (c ChangeAction) IsValid() bool {
	switch c {
	case ChangeActionCreated, ChangeActionUpdated, ChangeActionDeleted:
		return true
	}
	return false
}

func (c ChangeAction) String() string {
	return string(c)
}

// User type
type User struct {
	TypeName     string     `json:"__typename"`
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"displayName"`
	AvatarURL    *string    `json:"avatarUrl"`
	Timezone     string     `json:"timezone"`
	IsPremium    bool       `json:"isPremium"`
	PremiumUntil *time.Time `json:"premiumUntil"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func UserFromModel(u *models.User) *User {
	if u == nil {
		return nil
	}
	var avatarURL *string
	if u.AvatarURL != "" {
		avatarURL = &u.AvatarURL
	}
	return &User{
		TypeName:     "User",
		ID:           u.ID,
		Email:        u.Email,
		DisplayName:  u.DisplayName,
		AvatarURL:    avatarURL,
		Timezone:     u.Timezone,
		IsPremium:    u.HasActivePremium(),
		PremiumUntil: u.PremiumUntil,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// AuthPayload type
type AuthPayload struct {
	TypeName               string `json:"__typename"`
	AccessToken            string `json:"accessToken"`
	RefreshToken           string `json:"refreshToken"`
	ExpiresIn              int    `json:"expiresIn"`
	User                   *User  `json:"user"`
	AccountPendingDeletion bool   `json:"accountPendingDeletion"`
}

// AuthenticateWithAppleInput type
type AuthenticateWithAppleInput struct {
	IdentityToken  string  `json:"identityToken"`
	UserIdentifier string  `json:"userIdentifier"`
	Email          *string `json:"email"`
	DisplayName    *string `json:"displayName"`
}

// Device type
type Device struct {
	TypeName         string    `json:"__typename"`
	ID               uuid.UUID `json:"id"`
	Platform         Platform  `json:"platform"`
	DeviceIdentifier string    `json:"deviceIdentifier"`
	PushToken        string    `json:"pushToken"`
	DeviceName       *string   `json:"deviceName"`
	AppVersion       *string   `json:"appVersion"`
	OsVersion        *string   `json:"osVersion"`
	LastSeenAt       time.Time `json:"lastSeenAt"`
	CreatedAt        time.Time `json:"createdAt"`
}

func DeviceFromModel(d *models.Device) *Device {
	if d == nil {
		return nil
	}
	platform := PlatformIOS
	if d.Platform == "android" {
		platform = PlatformAndroid
	}
	var deviceName, appVersion, osVersion *string
	if d.DeviceName != "" {
		deviceName = &d.DeviceName
	}
	if d.AppVersion != "" {
		appVersion = &d.AppVersion
	}
	if d.OSVersion != "" {
		osVersion = &d.OSVersion
	}
	return &Device{
		TypeName:         "Device",
		ID:               d.ID,
		Platform:         platform,
		DeviceIdentifier: d.DeviceIdentifier,
		PushToken:        d.PushToken,
		DeviceName:       deviceName,
		AppVersion:       appVersion,
		OsVersion:        osVersion,
		LastSeenAt:       d.LastSeenAt,
		CreatedAt:        d.CreatedAt,
	}
}

// RecurrenceRule type
type RecurrenceRule struct {
	TypeName             string     `json:"__typename"`
	Frequency            Frequency  `json:"frequency"`
	Interval             int        `json:"interval"`
	DaysOfWeek           []int      `json:"daysOfWeek"`
	DayOfMonth           *int       `json:"dayOfMonth"`
	MonthOfYear          *int       `json:"monthOfYear"`
	EndAfterOccurrences  *int       `json:"endAfterOccurrences"`
	EndDate              *time.Time `json:"endDate"`
}

func RecurrenceRuleFromModel(r *models.RecurrenceRule) *RecurrenceRule {
	if r == nil {
		return nil
	}
	var endDate *time.Time
	if r.EndDate != nil {
		parsed, err := time.Parse(time.RFC3339, *r.EndDate)
		if err == nil {
			endDate = &parsed
		}
	}
	return &RecurrenceRule{
		TypeName:             "RecurrenceRule",
		Frequency:            FrequencyFromModel(r.Frequency),
		Interval:             r.Interval,
		DaysOfWeek:           r.DaysOfWeek,
		DayOfMonth:           r.DayOfMonth,
		MonthOfYear:          r.MonthOfYear,
		EndAfterOccurrences:  r.EndAfterOccurrences,
		EndDate:              endDate,
	}
}

func RecurrenceRuleToModel(r *RecurrenceRuleInput) *models.RecurrenceRule {
	if r == nil {
		return nil
	}
	var endDateStr *string
	if r.EndDate != nil {
		s := r.EndDate.Format(time.RFC3339)
		endDateStr = &s
	}
	return &models.RecurrenceRule{
		Frequency:            FrequencyToModel(r.Frequency),
		Interval:             r.Interval,
		DaysOfWeek:           r.DaysOfWeek,
		DayOfMonth:           r.DayOfMonth,
		MonthOfYear:          r.MonthOfYear,
		EndAfterOccurrences:  r.EndAfterOccurrences,
		EndDate:              endDateStr,
	}
}

// RecurrenceRuleInput type
type RecurrenceRuleInput struct {
	Frequency            Frequency  `json:"frequency"`
	Interval             int        `json:"interval"`
	DaysOfWeek           []int      `json:"daysOfWeek"`
	DayOfMonth           *int       `json:"dayOfMonth"`
	MonthOfYear          *int       `json:"monthOfYear"`
	EndAfterOccurrences  *int       `json:"endAfterOccurrences"`
	EndDate              *time.Time `json:"endDate"`
}

// ReminderList type
type ReminderList struct {
	TypeName      string    `json:"__typename"`
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	ColorHex      string    `json:"colorHex"`
	IconName      string    `json:"iconName"`
	SortOrder     int       `json:"sortOrder"`
	IsDefault     bool      `json:"isDefault"`
	ReminderCount int       `json:"reminderCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func ReminderListFromModel(l *models.ReminderList, reminderCount int64) *ReminderList {
	if l == nil {
		return nil
	}
	return &ReminderList{
		TypeName:      "ReminderList",
		ID:            l.ID,
		Name:          l.Name,
		ColorHex:      l.ColorHex,
		IconName:      l.IconName,
		SortOrder:     l.SortOrder,
		IsDefault:     l.IsDefault,
		ReminderCount: int(reminderCount),
		CreatedAt:     l.CreatedAt,
		UpdatedAt:     l.UpdatedAt,
	}
}

// Reminder type
type Reminder struct {
	TypeName       string          `json:"__typename"`
	ID             uuid.UUID       `json:"id"`
	ListID         *uuid.UUID      `json:"listId"`
	List           *ReminderList   `json:"list"`
	Title          string          `json:"title"`
	Notes          *string         `json:"notes"`
	Priority       Priority        `json:"priority"`
	DueAt          *time.Time      `json:"dueAt"`   // Optional: reminders without dates don't trigger notifications
	AllDay         *bool           `json:"allDay"`  // Optional: only relevant when DueAt is set
	RecurrenceRule *RecurrenceRule `json:"recurrenceRule"`
	RecurrenceEnd  *time.Time      `json:"recurrenceEnd"`
	Status         ReminderStatus  `json:"status"`
	CompletedAt    *time.Time      `json:"completedAt"`
	SnoozedUntil   *time.Time      `json:"snoozedUntil"`
	SnoozeCount    int             `json:"snoozeCount"`
	IsAlarm        bool            `json:"isAlarm"`
	SoundID        *string         `json:"soundId"`
	Tags           []string        `json:"tags"`
	LocalID        *string         `json:"localId"`
	Version        int             `json:"version"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

func ReminderFromModel(r *models.Reminder) *Reminder {
	if r == nil {
		return nil
	}
	tags := r.Tags
	if tags == nil {
		tags = []string{}
	}
	return &Reminder{
		TypeName:       "Reminder",
		ID:             r.ID,
		ListID:         r.ListID,
		Title:          r.Title,
		Notes:          r.Notes,
		Priority:       PriorityFromModel(r.Priority),
		DueAt:          r.DueAt,   // Already a pointer, maps directly
		AllDay:         r.AllDay,  // Already a pointer, maps directly
		RecurrenceRule: RecurrenceRuleFromModel(r.RecurrenceRule),
		RecurrenceEnd:  r.RecurrenceEnd,
		Status:         ReminderStatusFromModel(r.Status),
		CompletedAt:    r.CompletedAt,
		SnoozedUntil:   r.SnoozedUntil,
		SnoozeCount:    r.SnoozeCount,
		IsAlarm:        r.IsAlarm,
		SoundID:        r.SoundID,
		Tags:           tags,
		LocalID:        r.LocalID,
		Version:        r.Version,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// Input types
type CreateReminderInput struct {
	ListID         *uuid.UUID           `json:"listId"`
	Title          string               `json:"title"`
	Notes          *string              `json:"notes"`
	Priority       *Priority            `json:"priority"`
	DueAt          *time.Time           `json:"dueAt"`  // Optional: reminders without dates don't trigger notifications
	AllDay         *bool                `json:"allDay"` // Optional: only relevant when DueAt is set
	RecurrenceRule *RecurrenceRuleInput `json:"recurrenceRule"`
	RecurrenceEnd  *time.Time           `json:"recurrenceEnd"`
	IsAlarm        *bool                `json:"isAlarm"`
	SoundID        *string              `json:"soundId"`
	Tags           []string             `json:"tags"`
	LocalID        *string              `json:"localId"`
}

type UpdateReminderInput struct {
	ListID         *uuid.UUID           `json:"listId"`
	Title          *string              `json:"title"`
	Notes          *string              `json:"notes"`
	Priority       *Priority            `json:"priority"`
	DueAt          *time.Time           `json:"dueAt"`
	AllDay         *bool                `json:"allDay"`
	RecurrenceRule *RecurrenceRuleInput `json:"recurrenceRule"`
	RecurrenceEnd  *time.Time           `json:"recurrenceEnd"`
	IsAlarm        *bool                `json:"isAlarm"`
	SoundID        *string              `json:"soundId"`
	Status         *ReminderStatus      `json:"status"`
	Tags           []string             `json:"tags"`
}

type ReminderFilter struct {
	ListID   *uuid.UUID      `json:"listId"`
	Status   *ReminderStatus `json:"status"`
	FromDate *time.Time      `json:"fromDate"`
	ToDate   *time.Time      `json:"toDate"`
	Priority *Priority       `json:"priority"`
	Tags     []string        `json:"tags"`
}

// ReminderList input types
type CreateReminderListInput struct {
	Name     string  `json:"name"`
	ColorHex *string `json:"colorHex"`
	IconName *string `json:"iconName"`
}

type UpdateReminderListInput struct {
	Name      *string `json:"name"`
	ColorHex  *string `json:"colorHex"`
	IconName  *string `json:"iconName"`
	SortOrder *int    `json:"sortOrder"`
}

// ReminderList change event
type ReminderListChangeEvent struct {
	TypeName       string        `json:"__typename"`
	Action         ChangeAction  `json:"action"`
	ReminderList   *ReminderList `json:"reminderList"`
	ReminderListID uuid.UUID     `json:"reminderListId"`
	Timestamp      time.Time     `json:"timestamp"`
}

type PaginationInput struct {
	First  *int    `json:"first"`
	After  *string `json:"after"`
	Last   *int    `json:"last"`
	Before *string `json:"before"`
}

type RegisterDeviceInput struct {
	Platform         Platform `json:"platform"`
	DeviceIdentifier string   `json:"deviceIdentifier"`
	PushToken        string   `json:"pushToken"`
	DeviceName       *string  `json:"deviceName"`
	AppVersion       *string  `json:"appVersion"`
	OsVersion        *string  `json:"osVersion"`
}

// Connection types
type PageInfo struct {
	TypeName        string  `json:"__typename"`
	HasNextPage     bool    `json:"hasNextPage"`
	HasPreviousPage bool    `json:"hasPreviousPage"`
	StartCursor     *string `json:"startCursor"`
	EndCursor       *string `json:"endCursor"`
}

type ReminderEdge struct {
	TypeName string    `json:"__typename"`
	Node     *Reminder `json:"node"`
	Cursor   string    `json:"cursor"`
}

type ReminderConnection struct {
	TypeName   string          `json:"__typename"`
	Edges      []*ReminderEdge `json:"edges"`
	PageInfo   *PageInfo       `json:"pageInfo"`
	TotalCount int             `json:"totalCount"`
}

// Subscription types
type ReminderChangeEvent struct {
	TypeName   string       `json:"__typename"`
	Action     ChangeAction `json:"action"`
	Reminder   *Reminder    `json:"reminder"`
	ReminderID uuid.UUID    `json:"reminderId"`
	Timestamp  time.Time    `json:"timestamp"`
}

// NotificationSound type
type NotificationSound struct {
	TypeName string    `json:"__typename"`
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Filename string    `json:"filename"`
	IsFree   bool      `json:"isFree"`
}

func NotificationSoundFromModel(s *models.NotificationSound) *NotificationSound {
	if s == nil {
		return nil
	}
	return &NotificationSound{
		TypeName: "NotificationSound",
		ID:       s.ID,
		Name:     s.Name,
		Filename: s.Filename,
		IsFree:   s.IsFree,
	}
}
