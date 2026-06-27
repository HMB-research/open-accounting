package email

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"
	"gorm.io/gorm"
)

func stubWave11GormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func TestWave11NewServiceUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	service := NewService(pool)

	require.True(t, called)
	require.NotNil(t, service)
	repo, ok := service.repo.(*GORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, repo.db)
	require.IsType(t, &DefaultMailSender{}, service.mailer)
}

func TestWave11NewServicePanicsOnInjectedGormError(t *testing.T) {
	expectedErr := errors.New("pool unavailable")
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, expectedErr
	})

	require.PanicsWithError(t, "create email GORM repository: pool unavailable", func() {
		_ = NewService(new(pgxpool.Pool))
	})
}

func TestWave11UpdateSMTPConfigWrapsMergeError(t *testing.T) {
	original := mergeSMTPConfig
	mergeSMTPConfig = func([]byte, *UpdateSMTPConfigRequest) ([]byte, error) {
		return nil, errors.New("merge failed")
	}
	t.Cleanup(func() {
		mergeSMTPConfig = original
	})

	service := NewServiceWithRepository(&MockRepository{
		GetTenantSettingsFn: func(context.Context, string) ([]byte, error) {
			return validSMTPSettingsJSON(), nil
		},
	}, &MockMailSender{})

	err := service.UpdateSMTPConfig(context.Background(), "tenant-1", &UpdateSMTPConfigRequest{Host: "smtp.example.com"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to marshal settings")
	require.Contains(t, err.Error(), "merge failed")
}

func TestWave11SendEmailAttachmentErrorLogsFailure(t *testing.T) {
	original := attachEmailReader
	attachEmailReader = func(*mail.Msg, string, []byte) error {
		return errors.New("attachment failed")
	}
	t.Cleanup(func() {
		attachEmailReader = original
	})

	var updatedStatus EmailStatus
	var loggedError string
	service := NewServiceWithRepository(&MockRepository{
		GetTenantSettingsFn: func(context.Context, string) ([]byte, error) {
			return validSMTPSettingsJSON(), nil
		},
		CreateEmailLogFn: func(context.Context, string, *EmailLog) error {
			return nil
		},
		UpdateEmailLogStatusFn: func(_ context.Context, _, _ string, status EmailStatus, _ *time.Time, errorMessage string) error {
			updatedStatus = status
			loggedError = errorMessage
			return nil
		},
	}, &MockMailSender{})

	result, err := service.SendEmail(
		context.Background(),
		"tenant_test",
		"tenant-1",
		"INVOICE_SEND",
		"customer@example.com",
		"",
		"Subject",
		"<p>Body</p>",
		"",
		[]Attachment{{Filename: "receipt.pdf", Content: []byte("payload")}},
		"invoice-1",
	)

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attach file receipt.pdf")
	require.Equal(t, StatusFailed, updatedStatus)
	require.Contains(t, loggedError, "attachment failed")
}

func TestWave11LogEmailErrorWithoutRepository(t *testing.T) {
	var service *Service

	result, err := service.logEmailError(context.Background(), "tenant_test", "log-1", errors.New("send failed"))

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to send email: send failed")
}

func TestWave11DefaultMailSenderClientCreationError(t *testing.T) {
	msg := mail.NewMsg()
	err := (&DefaultMailSender{}).SendMail(&SMTPConfig{Host: "localhost", Port: -1}, msg)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create mail client")
}

func TestWave11DefaultMailSenderSendErrorUsesDefaultDialer(t *testing.T) {
	msg := mail.NewMsg()
	require.NoError(t, msg.From("sender@example.com"))
	require.NoError(t, msg.To("recipient@example.com"))
	msg.Subject("Test")
	msg.SetBodyString(mail.TypeTextPlain, "Hello")

	err := (&DefaultMailSender{}).SendMail(&SMTPConfig{Host: "127.0.0.1", Port: 1}, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to send email")
}

func TestWave11DefaultMailSenderSuccess(t *testing.T) {
	original := dialAndSendMail
	dialAndSendMail = func(*mail.Client, *mail.Msg) error {
		return nil
	}
	t.Cleanup(func() {
		dialAndSendMail = original
	})

	msg := mail.NewMsg()
	err := (&DefaultMailSender{}).SendMail(&SMTPConfig{Host: "localhost", Port: 25}, msg)
	require.NoError(t, err)
}
