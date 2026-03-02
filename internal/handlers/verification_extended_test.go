package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
	"go.uber.org/zap"
)

// TestVerifyPatronCredentials_NotRequired tests the path where verification is disabled.
func TestVerifyPatronCredentials_NotRequired(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = false
	tc.UsePinForPatronVerification = false
	session := testutil.NewSession(tc)
	patronClient := &MockPatronClient{}

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-id",
		"anypassword",
	)

	assert.True(t, result.Verified)
	assert.False(t, result.Required)
	assert.NoError(t, result.Error)
	patronClient.AssertExpectations(t)
}

// TestVerifyPatronCredentials_RequiredButEmpty tests the path where password is empty.
func TestVerifyPatronCredentials_RequiredButEmpty(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = false
	session := testutil.NewSession(tc)
	patronClient := &MockPatronClient{}

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-id",
		"", // Empty password
	)

	assert.False(t, result.Verified)
	assert.True(t, result.Required)
	assert.Equal(t, ErrVerificationRequired, result.Error)
	patronClient.AssertExpectations(t)
}

// TestVerifyPatronCredentials_PINMode_NetworkFailure tests PIN verification when FOLIO unavailable.
func TestVerifyPatronCredentials_PINMode_NetworkFailure(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = true
	session := testutil.NewSession(tc)

	patronClient := &MockPatronClient{}
	patronClient.On("VerifyPatronPin", mock.Anything, "test-token", "user-id", "1234pin").
		Return(false, errors.New("connection refused"))

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-barcode",
		"1234pin",
	)

	assert.False(t, result.Verified)
	assert.True(t, result.Required)
	assert.Error(t, result.Error)
	patronClient.AssertExpectations(t)
}

// TestVerifyPatronCredentials_LoginMode_NetworkFailure tests login-based verification when FOLIO unavailable.
func TestVerifyPatronCredentials_LoginMode_NetworkFailure(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = false // login mode
	session := testutil.NewSession(tc)

	patronClient := &MockPatronClient{}
	patronClient.On("VerifyPatronPasswordWithLogin", mock.Anything, "patron-barcode", "password123").
		Return(false, errors.New("connection refused"))

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-barcode",
		"password123",
	)

	assert.False(t, result.Verified)
	assert.True(t, result.Required)
	assert.Error(t, result.Error)
	patronClient.AssertExpectations(t)
}

// TestGetVerificationErrorMessage verifies the error message is non-empty and consistent.
func TestGetVerificationErrorMessage(t *testing.T) {
	msg := GetVerificationErrorMessage()
	assert.NotEmpty(t, msg)
}
