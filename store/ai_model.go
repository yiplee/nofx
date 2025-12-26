package store

import (
	"database/sql"
	"errors"
	"fmt"
	"nofx/logger"
	"time"
)

// AIModelStore AI model storage
type AIModelStore struct {
	db          *sql.DB
	encryptFunc func(string) string
	decryptFunc func(string) string
}

// AIModel AI model configuration
type AIModel struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	Enabled         bool      `json:"enabled"`
	APIKey          string    `json:"apiKey"`
	CustomAPIURL    string    `json:"customApiUrl"`
	CustomModelName string    `json:"customModelName"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (s *AIModelStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS ai_models (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			custom_api_url TEXT DEFAULT '',
			custom_model_name TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Trigger
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
		AFTER UPDATE ON ai_models
		BEGIN
			UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return err
	}

	// Backward compatibility: add potentially missing columns
	s.db.Exec(`ALTER TABLE ai_models ADD COLUMN custom_api_url TEXT DEFAULT ''`)
	s.db.Exec(`ALTER TABLE ai_models ADD COLUMN custom_model_name TEXT DEFAULT ''`)

	return nil
}

func (s *AIModelStore) initDefaultData() error {
	// No longer pre-populate AI models - create on demand when user configures
	return nil
}

func (s *AIModelStore) encrypt(plaintext string) string {
	if s.encryptFunc != nil {
		return s.encryptFunc(plaintext)
	}
	return plaintext
}

func (s *AIModelStore) decrypt(encrypted string) string {
	if s.decryptFunc != nil {
		return s.decryptFunc(encrypted)
	}
	return encrypted
}

// List retrieves user's AI model list
func (s *AIModelStore) List(userID string) ([]*AIModel, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, '') as custom_api_url,
		       COALESCE(custom_model_name, '') as custom_model_name,
		       created_at, updated_at
		FROM ai_models WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]*AIModel, 0)
	for rows.Next() {
		var model AIModel
		var createdAt, updatedAt string
		err := rows.Scan(
			&model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		model.APIKey = s.decrypt(model.APIKey)
		models = append(models, &model)
	}
	return models, nil
}

// Get retrieves a single AI model
func (s *AIModelStore) Get(userID, modelID string) (*AIModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID cannot be empty")
	}

	candidates := []string{}
	if userID != "" {
		candidates = append(candidates, userID)
	}
	if userID != "default" {
		candidates = append(candidates, "default")
	}
	if len(candidates) == 0 {
		candidates = append(candidates, "default")
	}

	for _, uid := range candidates {
		var model AIModel
		var createdAt, updatedAt string
		err := s.db.QueryRow(`
			SELECT id, user_id, name, provider, enabled, api_key,
			       COALESCE(custom_api_url, ''), COALESCE(custom_model_name, ''), created_at, updated_at
			FROM ai_models WHERE user_id = ? AND id = ? LIMIT 1
		`, uid, modelID).Scan(
			&model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&createdAt, &updatedAt,
		)
		if err == nil {
			model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
			model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
			model.APIKey = s.decrypt(model.APIKey)
			return &model, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return nil, sql.ErrNoRows
}

// GetByID retrieves an AI model by ID only (for debate engine)
func (s *AIModelStore) GetByID(modelID string) (*AIModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID cannot be empty")
	}

	var model AIModel
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, ''), COALESCE(custom_model_name, ''), created_at, updated_at
		FROM ai_models WHERE id = ? LIMIT 1
	`, modelID).Scan(
		&model.ID, &model.UserID, &model.Name, &model.Provider,
		&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	model.APIKey = s.decrypt(model.APIKey)
	return &model, nil
}

// GetDefault retrieves the default enabled AI model
func (s *AIModelStore) GetDefault(userID string) (*AIModel, error) {
	if userID == "" {
		userID = "default"
	}
	model, err := s.firstEnabled(userID)
	if err == nil {
		return model, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if userID != "default" {
		return s.firstEnabled("default")
	}
	return nil, fmt.Errorf("please configure an available AI model in the system first")
}

func (s *AIModelStore) firstEnabled(userID string) (*AIModel, error) {
	var model AIModel
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, ''), COALESCE(custom_model_name, ''), created_at, updated_at
		FROM ai_models WHERE user_id = ? AND enabled = 1
		ORDER BY datetime(updated_at) DESC, id ASC LIMIT 1
	`, userID).Scan(
		&model.ID, &model.UserID, &model.Name, &model.Provider,
		&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	model.APIKey = s.decrypt(model.APIKey)
	return &model, nil
}

// Update updates AI model, creates if not exists
// IMPORTANT: If apiKey is empty string, the existing API key will be preserved (not overwritten)
func (s *AIModelStore) Update(userID, id, name string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	// Try exact ID match first
	var existingID string
	err := s.db.QueryRow(`SELECT id FROM ai_models WHERE user_id = ? AND id = ? LIMIT 1`, userID, id).Scan(&existingID)
	if err != nil {
		return err
	}

	// If apiKey is empty, preserve the existing API key
	if apiKey == "" {
		_, err = s.db.Exec(`
				UPDATE ai_models SET name = ?, enabled = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
				WHERE id = ? AND user_id = ?
			`, name, enabled, customAPIURL, customModelName, existingID, userID)
	} else {
		encryptedAPIKey := s.encrypt(apiKey)
		_, err = s.db.Exec(`
				UPDATE ai_models SET name = ?, enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
				WHERE id = ? AND user_id = ?
			`, name, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
	}
	return err
}

// Create creates a new AI model with auto-generated ID
func (s *AIModelStore) Create(userID, name, provider string, enabled bool, apiKey, customAPIURL, customModelName string) (string, error) {
	// Generate unique ID: {userID}_{provider}_{timestamp}
	modelID := fmt.Sprintf("%s_%s_%d", provider, userID, time.Now().UnixNano())

	encryptedAPIKey := s.encrypt(apiKey)
	_, err := s.db.Exec(`
		INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, modelID, userID, name, provider, enabled, encryptedAPIKey, customAPIURL, customModelName)
	if err != nil {
		return "", err
	}
	logger.Infof("✓ Created new AI model: ID=%s, Provider=%s, Name=%s", modelID, provider, name)
	return modelID, nil
}

// Delete deletes an AI model by ID
func (s *AIModelStore) Delete(userID, modelID string) error {
	result, err := s.db.Exec(`DELETE FROM ai_models WHERE id = ? AND user_id = ?`, modelID, userID)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("model not found: %s", modelID)
	}
	logger.Infof("✓ Deleted AI model: ID=%s", modelID)
	return nil
}
