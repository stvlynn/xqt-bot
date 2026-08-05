package application

import (
	"context"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

// defaultSummaryHours is the lookback window when none is requested.
const defaultSummaryHours = 24

// minSummaryMessages is the smallest message count worth summarizing.
const minSummaryMessages = 3

// SummaryResult is the outcome of one summary generation.
type SummaryResult struct {
	Text         string
	MessageCount int
	Hours        int
}

// SummaryService records group messages and produces LLM summaries, on
// demand or on a recurring schedule.
type SummaryService struct {
	settings ports.SettingsRepository
	msglog   ports.MessageLogRepository
	tasks    ports.TaskRepository
	tg       ports.TelegramGateway
	llm      ports.LLMGateway
	now      func() time.Time
}

// NewSummaryService builds the service. llm may be nil when unconfigured.
func NewSummaryService(settings ports.SettingsRepository, msglog ports.MessageLogRepository, tasks ports.TaskRepository, tg ports.TelegramGateway, llm ports.LLMGateway) *SummaryService {
	return &SummaryService{
		settings: settings,
		msglog:   msglog,
		tasks:    tasks,
		tg:       tg,
		llm:      llm,
		now:      clockNow,
	}
}

// RecordMessage appends one message to the chat's ring buffer.
func (s *SummaryService) RecordMessage(ctx context.Context, chatID int64, msg summary.Message) error {
	return s.msglog.Append(ctx, chatID, msg)
}

// SummarizeNow summarizes the chat's recent messages for an admin.
func (s *SummaryService) SummarizeNow(ctx context.Context, chatID, requesterID int64, hours int) (*SummaryResult, error) {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return nil, err
	}
	return s.summarizeChat(ctx, chatID, hours)
}

// summarizeChat is the admin-check-free core shared with the TaskRunner.
func (s *SummaryService) summarizeChat(ctx context.Context, chatID int64, hours int) (*SummaryResult, error) {
	if hours <= 0 {
		hours = defaultSummaryHours
	}
	msgs, err := s.msglog.Recent(ctx, chatID)
	if err != nil {
		return nil, err
	}
	since := s.now().Add(-time.Duration(hours) * time.Hour)
	window := make([]summary.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.At.After(since) {
			window = append(window, m)
		}
	}
	if len(window) < minSummaryMessages {
		return nil, ErrTooFewMessages
	}
	if s.llm == nil || !s.llm.Available() {
		return nil, ErrLLMNotConfigured
	}
	text, err := s.llm.Summarize(ctx, window)
	if err != nil {
		return nil, err
	}
	return &SummaryResult{Text: text, MessageCount: len(window), Hours: hours}, nil
}

// SetAutoSummary enables (intervalHours > 0) or disables (<= 0) the recurring
// summary task for the chat.
func (s *SummaryService) SetAutoSummary(ctx context.Context, chatID, requesterID int64, intervalHours int) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	if intervalHours <= 0 {
		st.Summary.AutoEnabled = false
		if err := s.settings.Save(ctx, st); err != nil {
			return err
		}
		return s.tasks.Delete(ctx, schedule.KindAutoSummary, chatID)
	}
	st.Summary.AutoEnabled = true
	st.Summary.IntervalHours = intervalHours
	if err := s.settings.Save(ctx, st); err != nil {
		return err
	}
	return s.tasks.Save(ctx, schedule.NewTask(schedule.KindAutoSummary, chatID, intervalHours, s.now()))
}
