package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newFeedbackSettingsFixture(t *testing.T, stored dto.FeedbackSettings) (*OrganizationSettingsService, *mocks.MockOrganizationSettingsRepository, *dto.FeedbackSettings) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	current := stored
	repo.EXPECT().UpdateFeedback(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, merge ports.FeedbackMerge) (*dto.OrganizationSettingsView, error) {
			next, err := merge(current)
			if err != nil {
				return nil, err
			}
			current = next
			return &dto.OrganizationSettingsView{Feedback: next, Timezone: bogotaTZ}, nil
		}).Maybe()
	return NewOrganizationSettingsService(repo), repo, &current
}

func TestOrganizationSettingsService_Feedback_DefaultIsLinkOff(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: bogotaTZ}, nil).Once()

	res, err := NewOrganizationSettingsService(repo).Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, dto.FeedbackSettings{ReviewLinkEnabled: false, ReviewURL: ""}, res.Feedback)
}

func TestOrganizationSettingsService_Feedback_TurningTheLinkOn(t *testing.T) {
	svc, repo, current := newFeedbackSettingsFixture(t, dto.FeedbackSettings{})

	res, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Feedback: &dto.FeedbackSettingsPatch{
		ReviewLinkEnabled: boolPtr(true), ReviewURL: strPtr(" https://g.page/r/abc/review "),
	}})

	require.NoError(t, err)
	want := dto.FeedbackSettings{ReviewLinkEnabled: true, ReviewURL: "https://g.page/r/abc/review"}
	assert.Equal(t, want, *current)
	assert.Equal(t, want, res.Feedback)
	repo.AssertNotCalled(t, "UpdateOrdering", mock.Anything, mock.Anything)
}

func TestOrganizationSettingsService_Feedback_PartialPatchKeepsTheRest(t *testing.T) {
	svc, _, current := newFeedbackSettingsFixture(t, dto.FeedbackSettings{ReviewLinkEnabled: true, ReviewURL: "https://g.page/r/abc/review"})

	_, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Feedback: &dto.FeedbackSettingsPatch{ReviewLinkEnabled: boolPtr(false)}})

	require.NoError(t, err)
	assert.Equal(t, dto.FeedbackSettings{ReviewURL: "https://g.page/r/abc/review"}, *current)
}

func TestOrganizationSettingsService_Feedback_InvalidURLChangesNothing(t *testing.T) {
	for _, url := range []string{"http://example.com", "g.page/r/abc", "https://", "ftp://example.com"} {
		t.Run(url, func(t *testing.T) {
			svc, repo, current := newFeedbackSettingsFixture(t, dto.FeedbackSettings{})

			_, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{
				Ordering: &dto.OrderingSettingsPatch{Delivery: &dto.OrderTypeSettingsPatch{Paused: boolPtr(true)}},
				Feedback: &dto.FeedbackSettingsPatch{ReviewURL: &url},
			})

			var validation *domainError.ValidationError
			require.True(t, errors.As(err, &validation))
			assert.Equal(t, "feedback.review_url", validation.Field)
			assert.Equal(t, dto.FeedbackSettings{}, *current)
			repo.AssertNotCalled(t, "UpdateOrdering", mock.Anything, mock.Anything)
		})
	}
}

func TestOrganizationSettingsService_Feedback_ClearingTheURL(t *testing.T) {
	svc, _, current := newFeedbackSettingsFixture(t, dto.FeedbackSettings{ReviewURL: "https://g.page/r/abc/review"})

	_, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Feedback: &dto.FeedbackSettingsPatch{ReviewURL: strPtr("")}})

	require.NoError(t, err)
	assert.Empty(t, current.ReviewURL)
}

func TestOrganizationSettingsService_Feedback_RepositoryError(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().UpdateFeedback(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()

	_, err := NewOrganizationSettingsService(repo).Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{
		Feedback: &dto.FeedbackSettingsPatch{ReviewLinkEnabled: boolPtr(true)},
	})

	require.ErrorContains(t, err, "db down")
}

func TestOrganizationSettingsService_Feedback_RequiresItsOwnPermission(t *testing.T) {
	svc := NewOrganizationSettingsService(mocks.NewMockOrganizationSettingsRepository(t))

	assert.Equal(t, []permissions.Permission{permissions.SettingsFeedbackUpdate},
		svc.RequiredPermissions(&dto.UpdateOrganizationSettingsRequest{Feedback: &dto.FeedbackSettingsPatch{ReviewURL: strPtr("")}}))
	assert.Empty(t, svc.RequiredPermissions(&dto.UpdateOrganizationSettingsRequest{Feedback: &dto.FeedbackSettingsPatch{}}))
	assert.False(t, permissions.HasPermission([]int{permissions.RoleAgent}, permissions.SettingsFeedbackUpdate), "the agent's patch is refused (403)")
}
