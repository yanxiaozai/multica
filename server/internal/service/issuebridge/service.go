package issuebridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	ProviderGitLab             = "gitlab"
	DefaultIntegrationName     = "GitLab"
	MinPollIntervalSeconds     = 60
	DefaultPollIntervalSeconds = 300
)

type Queries interface {
	GetSkillByWorkspaceAndName(context.Context, db.GetSkillByWorkspaceAndNameParams) (db.Skill, error)
	CreateSkill(context.Context, db.CreateSkillParams) (db.Skill, error)
	ListIssueIntegrationsByWorkspace(context.Context, pgtype.UUID) ([]db.IssueIntegration, error)
	GetIssueIntegrationInWorkspace(context.Context, db.GetIssueIntegrationInWorkspaceParams) (db.IssueIntegration, error)
	GetIssueIntegrationByProviderName(context.Context, db.GetIssueIntegrationByProviderNameParams) (db.IssueIntegration, error)
	CreateIssueIntegration(context.Context, db.CreateIssueIntegrationParams) (db.IssueIntegration, error)
	UpdateIssueIntegration(context.Context, db.UpdateIssueIntegrationParams) (db.IssueIntegration, error)
	DeleteIssueIntegration(context.Context, db.DeleteIssueIntegrationParams) (pgtype.UUID, error)
}

type SecretBox interface {
	Seal([]byte) ([]byte, error)
	Open([]byte) ([]byte, error)
}

type GitLabConnectionTester interface {
	TestConnection(context.Context) (GitLabUser, error)
}

type ClientFactory func(baseURL, token string) (GitLabConnectionTester, error)

type Service struct {
	Queries       Queries
	SecretBox     SecretBox
	ClientFactory ClientFactory
}

func NewService(queries Queries, box SecretBox) *Service {
	return &Service{
		Queries:       queries,
		SecretBox:     box,
		ClientFactory: defaultClientFactory,
	}
}

type SaveIntegrationInput struct {
	WorkspaceID                pgtype.UUID
	Name                       string
	BaseURL                    string
	Token                      string
	DefaultIssueSkillID        pgtype.UUID
	PollingEnabled             bool
	DefaultPollIntervalSeconds int32
	Config                     []byte
}

func (s *Service) EnsureDefaultIssueCreator(ctx context.Context, workspaceID, createdBy pgtype.UUID) (db.Skill, error) {
	if s == nil || s.Queries == nil {
		return db.Skill{}, fmt.Errorf("issue bridge service requires queries")
	}
	existing, err := s.Queries.GetSkillByWorkspaceAndName(ctx, db.GetSkillByWorkspaceAndNameParams{
		WorkspaceID: workspaceID,
		Name:        DefaultIssueCreatorName,
	})
	if err == nil {
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return db.Skill{}, err
	}
	return s.Queries.CreateSkill(ctx, db.CreateSkillParams{
		WorkspaceID: workspaceID,
		Name:        DefaultIssueCreatorName,
		Description: DefaultIssueCreatorDescription,
		Content:     DefaultIssueCreatorContent,
		Config:      []byte("{}"),
		CreatedBy:   createdBy,
	})
}

func (s *Service) ListIntegrations(ctx context.Context, workspaceID pgtype.UUID) ([]db.IssueIntegration, error) {
	if s == nil || s.Queries == nil {
		return nil, fmt.Errorf("issue bridge service requires queries")
	}
	return s.Queries.ListIssueIntegrationsByWorkspace(ctx, workspaceID)
}

func (s *Service) GetIntegration(ctx context.Context, workspaceID, id pgtype.UUID) (db.IssueIntegration, error) {
	if s == nil || s.Queries == nil {
		return db.IssueIntegration{}, fmt.Errorf("issue bridge service requires queries")
	}
	return s.Queries.GetIssueIntegrationInWorkspace(ctx, db.GetIssueIntegrationInWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (s *Service) GetGitLabIntegrationByName(ctx context.Context, workspaceID pgtype.UUID, name string) (db.IssueIntegration, error) {
	if s == nil || s.Queries == nil {
		return db.IssueIntegration{}, fmt.Errorf("issue bridge service requires queries")
	}
	return s.Queries.GetIssueIntegrationByProviderName(ctx, db.GetIssueIntegrationByProviderNameParams{
		WorkspaceID: workspaceID,
		Provider:    ProviderGitLab,
		Name:        CleanIntegrationName(name),
	})
}

func (s *Service) CreateGitLabIntegration(ctx context.Context, input SaveIntegrationInput) (db.IssueIntegration, error) {
	if s == nil || s.Queries == nil {
		return db.IssueIntegration{}, fmt.Errorf("issue bridge service requires queries")
	}
	baseURL, err := NormalizeGitLabBaseURL(input.BaseURL)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	config, err := CleanConfig(input.Config)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	interval, err := ValidatePollInterval(input.DefaultPollIntervalSeconds)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	encryptedToken, err := s.encryptToken(input.Token)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	return s.Queries.CreateIssueIntegration(ctx, db.CreateIssueIntegrationParams{
		WorkspaceID:                input.WorkspaceID,
		Provider:                   ProviderGitLab,
		Name:                       CleanIntegrationName(input.Name),
		BaseUrl:                    baseURL,
		EncryptedToken:             encryptedToken,
		DefaultIssueSkillID:        input.DefaultIssueSkillID,
		PollingEnabled:             input.PollingEnabled,
		DefaultPollIntervalSeconds: interval,
		Config:                     config,
	})
}

func (s *Service) UpdateGitLabIntegration(ctx context.Context, id pgtype.UUID, input SaveIntegrationInput) (db.IssueIntegration, error) {
	if s == nil || s.Queries == nil {
		return db.IssueIntegration{}, fmt.Errorf("issue bridge service requires queries")
	}
	baseURL, err := NormalizeGitLabBaseURL(input.BaseURL)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	config, err := CleanConfig(input.Config)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	interval, err := ValidatePollInterval(input.DefaultPollIntervalSeconds)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	encryptedToken, err := s.optionalEncryptedToken(input.Token)
	if err != nil {
		return db.IssueIntegration{}, err
	}
	return s.Queries.UpdateIssueIntegration(ctx, db.UpdateIssueIntegrationParams{
		ID:                         id,
		WorkspaceID:                input.WorkspaceID,
		Name:                       CleanIntegrationName(input.Name),
		BaseUrl:                    baseURL,
		DefaultIssueSkillID:        input.DefaultIssueSkillID,
		PollingEnabled:             input.PollingEnabled,
		DefaultPollIntervalSeconds: interval,
		Config:                     config,
		EncryptedToken:             encryptedToken,
	})
}

func (s *Service) DeleteIntegration(ctx context.Context, workspaceID, id pgtype.UUID) error {
	if s == nil || s.Queries == nil {
		return fmt.Errorf("issue bridge service requires queries")
	}
	_, err := s.Queries.DeleteIssueIntegration(ctx, db.DeleteIssueIntegrationParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	return err
}

func (s *Service) TestGitLabConnection(ctx context.Context, baseURL, token string) (GitLabUser, error) {
	factory := defaultClientFactory
	if s != nil && s.ClientFactory != nil {
		factory = s.ClientFactory
	}
	client, err := factory(baseURL, token)
	if err != nil {
		return GitLabUser{}, err
	}
	return client.TestConnection(ctx)
}

func (s *Service) TestStoredGitLabConnection(ctx context.Context, integration db.IssueIntegration) (GitLabUser, error) {
	token, err := s.decryptToken(integration.EncryptedToken)
	if err != nil {
		return GitLabUser{}, err
	}
	return s.TestGitLabConnection(ctx, integration.BaseUrl, token)
}

func CleanIntegrationName(name string) string {
	cleaned := strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
	if cleaned == "" {
		return DefaultIntegrationName
	}
	return cleaned
}

func CleanConfig(raw []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return []byte("{}"), nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("integration config must be valid JSON: %w", err)
	}
	if decoded == nil {
		return nil, fmt.Errorf("integration config must be a JSON object")
	}
	cleaned, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return cleaned, nil
}

func ValidatePollInterval(seconds int32) (int32, error) {
	if seconds == 0 {
		return DefaultPollIntervalSeconds, nil
	}
	if seconds < MinPollIntervalSeconds {
		return 0, fmt.Errorf("poll interval must be at least %d seconds", MinPollIntervalSeconds)
	}
	return seconds, nil
}

func (s *Service) encryptToken(token string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("gitlab token is required")
	}
	if s == nil || s.SecretBox == nil {
		return "", fmt.Errorf("issue bridge service requires secret box")
	}
	sealed, err := s.SecretBox.Seal([]byte(strings.TrimSpace(token)))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *Service) optionalEncryptedToken(token string) (pgtype.Text, error) {
	if strings.TrimSpace(token) == "" {
		return pgtype.Text{}, nil
	}
	encrypted, err := s.encryptToken(token)
	if err != nil {
		return pgtype.Text{}, err
	}
	return pgtype.Text{String: encrypted, Valid: true}, nil
}

func (s *Service) decryptToken(encrypted string) (string, error) {
	if strings.TrimSpace(encrypted) == "" {
		return "", fmt.Errorf("gitlab token is not configured")
	}
	if s == nil || s.SecretBox == nil {
		return "", fmt.Errorf("issue bridge service requires secret box")
	}
	sealed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return "", fmt.Errorf("gitlab token is not valid ciphertext: %w", err)
	}
	plain, err := s.SecretBox.Open(sealed)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func defaultClientFactory(baseURL, token string) (GitLabConnectionTester, error) {
	return NewGitLabClient(baseURL, token)
}
