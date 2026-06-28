package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	mysqlconfig "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type persistenceStore interface {
	SaveAuthSession(context.Context, string, authSession) error
	FindAuthSession(context.Context, string) (authSession, bool, error)
	DeleteAuthSession(context.Context, string) error
	DeleteExpiredAuthSessions(context.Context, time.Time) error
	LoadChannelScores(context.Context) (map[string]scoreRecord, error)
	SaveChannelScores(context.Context, []scoreRecord) error
	Close() error
}

type mariaDBStore struct {
	db *gorm.DB
}

type oauthSessionModel struct {
	SessionID    string    `gorm:"column:session_id;type:varchar(128);primaryKey"`
	AccessToken  string    `gorm:"column:access_token;type:text;not null"`
	TokenType    string    `gorm:"column:token_type;type:varchar(64);not null"`
	ExpiresIn    int       `gorm:"column:expires_in;not null"`
	RefreshToken string    `gorm:"column:refresh_token;type:text;not null"`
	Scope        string    `gorm:"column:scope;type:text;not null"`
	ExpiresAt    time.Time `gorm:"column:expires_at;type:datetime(6);not null;index:idx_oauth_sessions_expires_at"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime(6);not null;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime(6);not null;autoUpdateTime"`
}

type channelScoreModel struct {
	ChannelID     string     `gorm:"column:channel_id;type:varchar(128);primaryKey"`
	Score         float64    `gorm:"column:score;not null"`
	LastSyncScore float64    `gorm:"column:last_sync_score;not null"`
	LastSyncAt    time.Time  `gorm:"column:last_sync_at;type:datetime(6);not null"`
	LastDecayAt   time.Time  `gorm:"column:last_decay_at;type:datetime(6);not null"`
	LastViewAt    *time.Time `gorm:"column:last_view_at;type:datetime(6)"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:datetime(6);not null;autoUpdateTime"`
}

func (oauthSessionModel) TableName() string {
	return "oauth_sessions"
}

func (channelScoreModel) TableName() string {
	return "channel_scores"
}

func openMariaDBStore(ctx context.Context, cfg mariaDBConfig) (*mariaDBStore, error) {
	db, err := gorm.Open(gormmysql.Open(cfg.dsn()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	store := &mariaDBStore{db: db}
	sqlDB, err := db.DB()
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := store.ensureSchema(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (cfg mariaDBConfig) dsn() string {
	mysqlCfg := mysqlconfig.NewConfig()
	mysqlCfg.User = cfg.user
	mysqlCfg.Passwd = cfg.password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = net.JoinHostPort(cfg.hostname, cfg.port)
	mysqlCfg.DBName = cfg.database
	mysqlCfg.ParseTime = true
	mysqlCfg.Loc = time.UTC
	mysqlCfg.Params = map[string]string{
		"charset": "utf8mb4",
	}
	return mysqlCfg.FormatDSN()
}

func (s *mariaDBStore) ensureSchema(ctx context.Context) error {
	return s.db.WithContext(ctx).
		Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").
		AutoMigrate(&oauthSessionModel{}, &channelScoreModel{})
}

func (s *mariaDBStore) SaveAuthSession(ctx context.Context, sessionID string, session authSession) error {
	model := oauthSessionModel{
		SessionID:    sessionID,
		AccessToken:  session.Token.AccessToken,
		TokenType:    session.Token.TokenType,
		ExpiresIn:    session.Token.ExpiresIn,
		RefreshToken: session.Token.RefreshToken,
		Scope:        session.Token.Scope,
		ExpiresAt:    dbTime(session.ExpiresAt),
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"access_token",
			"token_type",
			"expires_in",
			"refresh_token",
			"scope",
			"expires_at",
		}),
	}).Create(&model).Error
}

func (s *mariaDBStore) FindAuthSession(ctx context.Context, sessionID string) (authSession, bool, error) {
	var model oauthSessionModel
	err := s.db.WithContext(ctx).First(&model, "session_id = ?", sessionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return authSession{}, false, nil
	}
	if err != nil {
		return authSession{}, false, err
	}
	session := authSession{
		Token: tokenResponse{
			AccessToken:  model.AccessToken,
			TokenType:    model.TokenType,
			ExpiresIn:    model.ExpiresIn,
			RefreshToken: model.RefreshToken,
			Scope:        model.Scope,
		},
		ExpiresAt: model.ExpiresAt.UTC(),
	}
	return session, true, nil
}

func (s *mariaDBStore) DeleteAuthSession(ctx context.Context, sessionID string) error {
	return s.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&oauthSessionModel{}).Error
}

func (s *mariaDBStore) DeleteExpiredAuthSessions(ctx context.Context, now time.Time) error {
	return s.db.WithContext(ctx).Where("expires_at <= ?", dbTime(now)).Delete(&oauthSessionModel{}).Error
}

func (s *mariaDBStore) LoadChannelScores(ctx context.Context) (map[string]scoreRecord, error) {
	var models []channelScoreModel
	if err := s.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}

	records := make(map[string]scoreRecord, len(models))
	for _, model := range models {
		record := scoreRecord{
			ChannelID:     model.ChannelID,
			Score:         model.Score,
			LastSyncScore: model.LastSyncScore,
			LastSyncTime:  model.LastSyncAt.UTC(),
			LastDecayTime: model.LastDecayAt.UTC(),
		}
		if model.LastViewAt != nil {
			record.LastViewTime = model.LastViewAt.UTC()
		}
		records[record.ChannelID] = record
	}
	return records, nil
}

func (s *mariaDBStore) SaveChannelScores(ctx context.Context, records []scoreRecord) error {
	if len(records) == 0 {
		return nil
	}

	models := make([]channelScoreModel, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.ChannelID) == "" {
			continue
		}
		models = append(models, channelScoreModel{
			ChannelID:     record.ChannelID,
			Score:         record.Score,
			LastSyncScore: record.LastSyncScore,
			LastSyncAt:    dbTime(record.LastSyncTime),
			LastDecayAt:   dbTime(record.LastDecayTime),
			LastViewAt:    dbOptionalTime(record.LastViewTime),
		})
	}
	if len(models) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "channel_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"score",
				"last_sync_score",
				"last_sync_at",
				"last_decay_at",
				"last_view_at",
			}),
		}).Create(&models).Error; err != nil {
			return fmt.Errorf("save channel scores: %w", err)
		}
		return nil
	})
}

func (s *mariaDBStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func dbTime(t time.Time) time.Time {
	return t.UTC()
}

func dbOptionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	value := dbTime(t)
	return &value
}
