package store

import (
	"context"
	"database/sql"

	"github.com/seakee/cpa-manager-plus/usage-service/internal/model"
	"github.com/seakee/cpa-manager-plus/usage-service/internal/repository/apikeyalias"
	"github.com/seakee/cpa-manager-plus/usage-service/internal/repository/deadletter"
	"github.com/seakee/cpa-manager-plus/usage-service/internal/repository/modelprice"
	"github.com/seakee/cpa-manager-plus/usage-service/internal/repository/setting"
	sqliterepo "github.com/seakee/cpa-manager-plus/usage-service/internal/repository/sqlite"
	"github.com/seakee/cpa-manager-plus/usage-service/internal/repository/usageevent"
	"github.com/seakee/cpa-manager-plus/usage-service/internal/usage"
)

type Setup = model.Setup
type ManagerConfig = model.ManagerConfig
type ManagerCPAConnectionConfig = model.ManagerCPAConnectionConfig
type ManagerCollectorConfig = model.ManagerCollectorConfig
type ManagerExternalUsageServiceConfig = model.ManagerExternalUsageServiceConfig
type InsertResult = model.InsertResult
type ModelPrice = model.ModelPrice
type ModelPriceSyncResult = model.ModelPriceSyncResult
type APIKeyAlias = model.APIKeyAlias

// Aggregation result types re-exported for service-layer consumers.
type Aggregate = usageevent.Aggregate
type ModelStat = usageevent.ModelStat
type RecentFailure = usageevent.RecentFailure

type Store struct {
	db *sql.DB

	Settings      setting.Repository
	UsageEvents   usageevent.Repository
	DeadLetters   deadletter.Repository
	ModelPrices   modelprice.Repository
	APIKeyAliases apikeyalias.Repository
}

func Open(path string) (*Store, error) {
	db, err := sqliterepo.Open(path)
	if err != nil {
		return nil, err
	}
	return New(db), nil
}

func New(db *sql.DB) *Store {
	return &Store{
		db:            db,
		Settings:      setting.New(db),
		UsageEvents:   usageevent.New(db),
		DeadLetters:   deadletter.New(db),
		ModelPrices:   modelprice.New(db),
		APIKeyAliases: apikeyalias.New(db),
	}
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) SaveSetup(ctx context.Context, setup Setup) error {
	return s.Settings.SaveSetup(ctx, setup)
}

func (s *Store) LoadSetup(ctx context.Context) (Setup, bool, error) {
	return s.Settings.LoadSetup(ctx)
}

func (s *Store) SaveManagerConfig(ctx context.Context, cfg ManagerConfig) error {
	return s.Settings.SaveManagerConfig(ctx, cfg)
}

func (s *Store) LoadManagerConfig(ctx context.Context) (ManagerConfig, bool, error) {
	return s.Settings.LoadManagerConfig(ctx)
}

func (s *Store) LoadModelPrices(ctx context.Context) (map[string]ModelPrice, error) {
	return s.ModelPrices.LoadAll(ctx)
}

func (s *Store) SaveModelPrices(ctx context.Context, prices map[string]ModelPrice) error {
	return s.ModelPrices.ReplaceAll(ctx, prices)
}

func (s *Store) UpsertSyncedModelPrices(ctx context.Context, prices map[string]ModelPrice) (ModelPriceSyncResult, error) {
	return s.ModelPrices.UpsertSynced(ctx, prices)
}

func (s *Store) LoadAPIKeyAliases(ctx context.Context) ([]APIKeyAlias, error) {
	return s.APIKeyAliases.LoadAll(ctx)
}

func (s *Store) UpsertAPIKeyAliases(ctx context.Context, aliases []APIKeyAlias) error {
	return s.APIKeyAliases.UpsertMany(ctx, aliases)
}

func (s *Store) DeleteAPIKeyAlias(ctx context.Context, apiKeyHash string) error {
	return s.APIKeyAliases.Delete(ctx, apiKeyHash)
}

func (s *Store) InsertEvents(ctx context.Context, events []usage.Event) (InsertResult, error) {
	return s.UsageEvents.InsertBatch(ctx, events)
}

func (s *Store) AddDeadLetter(ctx context.Context, payload string, parseErr error) error {
	return s.DeadLetters.Insert(ctx, payload, parseErr.Error())
}

func (s *Store) RecentEvents(ctx context.Context, limit int) ([]usage.Event, error) {
	return s.UsageEvents.ListRecent(ctx, limit)
}

func (s *Store) Counts(ctx context.Context) (events int64, deadLetters int64, err error) {
	events, err = s.UsageEvents.Count(ctx)
	if err != nil {
		return 0, 0, err
	}
	deadLetters, err = s.DeadLetters.Count(ctx)
	if err != nil {
		return 0, 0, err
	}
	return events, deadLetters, nil
}

func (s *Store) ExportJSONL(ctx context.Context) ([]byte, error) {
	return s.UsageEvents.ExportJSONL(ctx)
}

// AggregateBetween computes summary metrics over [fromMs, toMs).
func (s *Store) AggregateBetween(ctx context.Context, fromMs, toMs int64) (Aggregate, error) {
	return s.UsageEvents.AggregateBetween(ctx, fromMs, toMs)
}

// TopModelsBetween returns the most active models ordered by call count.
func (s *Store) TopModelsBetween(ctx context.Context, fromMs, toMs int64, limit int) ([]ModelStat, error) {
	return s.UsageEvents.TopModelsBetween(ctx, fromMs, toMs, limit)
}

// ModelStatsBetween returns per-model totals for all models in a window.
func (s *Store) ModelStatsBetween(ctx context.Context, fromMs, toMs int64) ([]ModelStat, error) {
	return s.UsageEvents.ModelStatsBetween(ctx, fromMs, toMs)
}

// RecentFailuresBetween returns the most recent failed events in window.
func (s *Store) RecentFailuresBetween(ctx context.Context, fromMs, toMs int64, limit int) ([]RecentFailure, error) {
	return s.UsageEvents.RecentFailuresBetween(ctx, fromMs, toMs, limit)
}
