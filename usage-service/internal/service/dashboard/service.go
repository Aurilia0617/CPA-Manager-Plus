package dashboard

import (
	"context"
	"errors"
	"time"

	"github.com/seakee/cpa-manager-plus/usage-service/internal/service/pricing"
	"github.com/seakee/cpa-manager-plus/usage-service/internal/store"
)

const (
	defaultTopModels      = 5
	defaultRecentFailures = 5
	rollingWindowMinutes  = 30
	rollingWindowMs       = rollingWindowMinutes * 60 * 1000
)

type Service struct {
	store *store.Store
}

func New(store *store.Store) *Service {
	return &Service{store: store}
}

type SummaryParams struct {
	TodayStartMS   int64
	NowMS          int64
	TopModels      int
	RecentFailures int
}

type Window struct {
	TodayStartMS      int64 `json:"today_start_ms"`
	NowMS             int64 `json:"now_ms"`
	Rolling30MStartMS int64 `json:"rolling_30m_start_ms"`
}

type TodaySummary struct {
	TotalCalls       int64    `json:"total_calls"`
	SuccessCalls     int64    `json:"success_calls"`
	FailureCalls     int64    `json:"failure_calls"`
	SuccessRate      float64  `json:"success_rate"`
	InputTokens      int64    `json:"input_tokens"`
	OutputTokens     int64    `json:"output_tokens"`
	CachedTokens     int64    `json:"cached_tokens"`
	ReasoningTokens  int64    `json:"reasoning_tokens"`
	TotalTokens      int64    `json:"total_tokens"`
	TotalCost        float64  `json:"total_cost"`
	AverageLatencyMS *float64 `json:"average_latency_ms"`
	ZeroTokenCalls   int64    `json:"zero_token_calls"`
}

type RollingSummary struct {
	RPM         float64 `json:"rpm"`
	TPM         float64 `json:"tpm"`
	TotalCalls  int64   `json:"total_calls"`
	TotalTokens int64   `json:"total_tokens"`
}

type TopModel struct {
	Model       string  `json:"model"`
	Calls       int64   `json:"calls"`
	Tokens      int64   `json:"tokens"`
	Cost        float64 `json:"cost"`
	SuccessRate float64 `json:"success_rate"`
}

type RecentFailure struct {
	TimestampMS int64  `json:"timestamp_ms"`
	Model       string `json:"model"`
	APIKeyHash  string `json:"api_key_hash"`
	SourceHash  string `json:"source_hash"`
	AuthIndex   string `json:"auth_index"`
	Endpoint    string `json:"endpoint"`
	DurationMS  *int64 `json:"duration_ms"`
}

type SummaryResponse struct {
	GeneratedAtMS  int64           `json:"generated_at_ms"`
	Window         Window          `json:"window"`
	Today          TodaySummary    `json:"today"`
	Rolling30M     RollingSummary  `json:"rolling_30m"`
	TopModelsToday []TopModel      `json:"top_models_today"`
	RecentFailures []RecentFailure `json:"recent_failures"`
}

func (s *Service) Summary(ctx context.Context, p SummaryParams) (SummaryResponse, error) {
	if p.TodayStartMS <= 0 {
		return SummaryResponse{}, errors.New("today_start_ms is required")
	}

	generatedAt := time.Now().UnixMilli()
	nowMS := p.NowMS
	if nowMS <= 0 {
		nowMS = generatedAt
	}
	if nowMS < p.TodayStartMS {
		return SummaryResponse{}, errors.New("now_ms must be greater than or equal to today_start_ms")
	}

	topLimit := p.TopModels
	if topLimit <= 0 {
		topLimit = defaultTopModels
	}
	recentLimit := p.RecentFailures
	if recentLimit <= 0 {
		recentLimit = defaultRecentFailures
	}

	todayAgg, err := s.store.AggregateBetween(ctx, p.TodayStartMS, nowMS)
	if err != nil {
		return SummaryResponse{}, err
	}
	rollingStartMS := nowMS - rollingWindowMs
	rollingAgg, err := s.store.AggregateBetween(ctx, rollingStartMS, nowMS)
	if err != nil {
		return SummaryResponse{}, err
	}
	modelStats, err := s.store.ModelStatsBetween(ctx, p.TodayStartMS, nowMS)
	if err != nil {
		return SummaryResponse{}, err
	}
	topStats, err := s.store.TopModelsBetween(ctx, p.TodayStartMS, nowMS, topLimit)
	if err != nil {
		return SummaryResponse{}, err
	}
	recentFailures, err := s.store.RecentFailuresBetween(ctx, p.TodayStartMS, nowMS, recentLimit)
	if err != nil {
		return SummaryResponse{}, err
	}
	prices, err := s.store.LoadModelPrices(ctx)
	if err != nil {
		return SummaryResponse{}, err
	}

	return SummaryResponse{
		GeneratedAtMS: generatedAt,
		Window: Window{
			TodayStartMS:      p.TodayStartMS,
			NowMS:             nowMS,
			Rolling30MStartMS: rollingStartMS,
		},
		Today:          buildTodaySummary(todayAgg, modelStats, prices),
		Rolling30M:     buildRollingSummary(rollingAgg),
		TopModelsToday: buildTopModels(topStats, prices),
		RecentFailures: buildRecentFailures(recentFailures),
	}, nil
}

func buildTodaySummary(agg store.Aggregate, modelStats []store.ModelStat, prices map[string]store.ModelPrice) TodaySummary {
	return TodaySummary{
		TotalCalls:       agg.TotalCalls,
		SuccessCalls:     agg.SuccessCalls,
		FailureCalls:     agg.FailureCalls,
		SuccessRate:      rate(agg.SuccessCalls, agg.TotalCalls),
		InputTokens:      agg.InputTokens,
		OutputTokens:     agg.OutputTokens,
		CachedTokens:     agg.CachedTokens,
		ReasoningTokens:  agg.ReasoningTokens,
		TotalTokens:      agg.TotalTokens,
		TotalCost:        totalCost(modelStats, prices),
		AverageLatencyMS: nullableFloat(agg.AvgLatencyMS.Valid, agg.AvgLatencyMS.Float64),
		ZeroTokenCalls:   agg.ZeroTokenCalls,
	}
}

func buildRollingSummary(agg store.Aggregate) RollingSummary {
	return RollingSummary{
		RPM:         float64(agg.TotalCalls) / rollingWindowMinutes,
		TPM:         float64(agg.TotalTokens) / rollingWindowMinutes,
		TotalCalls:  agg.TotalCalls,
		TotalTokens: agg.TotalTokens,
	}
}

func buildTopModels(stats []store.ModelStat, prices map[string]store.ModelPrice) []TopModel {
	result := make([]TopModel, 0, len(stats))
	for _, stat := range stats {
		result = append(result, TopModel{
			Model:       stat.Model,
			Calls:       stat.Calls,
			Tokens:      stat.TotalTokens,
			Cost:        costForStat(stat, prices),
			SuccessRate: rate(stat.SuccessCalls, stat.Calls),
		})
	}
	return result
}

func buildRecentFailures(failures []store.RecentFailure) []RecentFailure {
	result := make([]RecentFailure, 0, len(failures))
	for _, failure := range failures {
		result = append(result, RecentFailure{
			TimestampMS: failure.TimestampMS,
			Model:       failure.Model,
			APIKeyHash:  failure.APIKeyHash,
			SourceHash:  failure.SourceHash,
			AuthIndex:   failure.AuthIndex,
			Endpoint:    failure.Endpoint,
			DurationMS:  nullableInt(failure.LatencyMS.Valid, failure.LatencyMS.Int64),
		})
	}
	return result
}

func totalCost(stats []store.ModelStat, prices map[string]store.ModelPrice) float64 {
	total := 0.0
	for _, stat := range stats {
		total += costForStat(stat, prices)
	}
	return total
}

func costForStat(stat store.ModelStat, prices map[string]store.ModelPrice) float64 {
	return pricing.CostForModel(stat.Model, pricing.ModelTokens{
		InputTokens:  stat.InputTokens,
		OutputTokens: stat.OutputTokens,
		CachedTokens: stat.CachedTokens,
	}, prices)
}

func rate(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total)
}

func nullableFloat(valid bool, value float64) *float64 {
	if !valid {
		return nil
	}
	return &value
}

func nullableInt(valid bool, value int64) *int64 {
	if !valid {
		return nil
	}
	return &value
}
