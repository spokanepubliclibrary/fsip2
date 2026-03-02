package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// TestVerifyPatronCredentials_PINMode_Valid verifies that when VerifyPatronPin
// returns true the result is Verified=true, Required=true.
func TestVerifyPatronCredentials_PINMode_Valid(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = true
	session := testutil.NewSession(tc)

	patronClient := &MockPatronClient{}
	patronClient.On("VerifyPatronPin", mock.Anything, "test-token", "user-id", "1234").
		Return(true, nil)

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-barcode",
		"1234",
	)

	assert.True(t, result.Verified)
	assert.True(t, result.Required)
	assert.NoError(t, result.Error)
	patronClient.AssertExpectations(t)
}

// TestVerifyPatronCredentials_PINMode_Invalid verifies that when VerifyPatronPin
// returns false the result is Verified=false, Required=true.
func TestVerifyPatronCredentials_PINMode_Invalid(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = true
	session := testutil.NewSession(tc)

	patronClient := &MockPatronClient{}
	patronClient.On("VerifyPatronPin", mock.Anything, "test-token", "user-id", "wrong-pin").
		Return(false, nil)

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-barcode",
		"wrong-pin",
	)

	assert.False(t, result.Verified)
	assert.True(t, result.Required)
	assert.Equal(t, ErrVerificationFailed, result.Error)
	patronClient.AssertExpectations(t)
}

// TestVerifyPatronCredentials_LoginMode_Valid verifies that when
// VerifyPatronPasswordWithLogin returns true the result is Verified=true.
func TestVerifyPatronCredentials_LoginMode_Valid(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = false
	session := testutil.NewSession(tc)

	patronClient := &MockPatronClient{}
	patronClient.On("VerifyPatronPasswordWithLogin", mock.Anything, "patron-barcode", "correct-pw").
		Return(true, nil)

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-barcode",
		"correct-pw",
	)

	assert.True(t, result.Verified)
	assert.True(t, result.Required)
	assert.NoError(t, result.Error)
	patronClient.AssertExpectations(t)
}

// TestVerifyPatronCredentials_LoginMode_Invalid verifies that when
// VerifyPatronPasswordWithLogin returns false the result is Verified=false.
func TestVerifyPatronCredentials_LoginMode_Invalid(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = false
	session := testutil.NewSession(tc)

	patronClient := &MockPatronClient{}
	patronClient.On("VerifyPatronPasswordWithLogin", mock.Anything, "patron-barcode", "wrong-pw").
		Return(false, nil)

	result := VerifyPatronCredentials(
		context.Background(),
		zap.NewNop(),
		session,
		patronClient,
		"test-token",
		"user-id",
		"patron-barcode",
		"wrong-pw",
	)

	assert.False(t, result.Verified)
	assert.True(t, result.Required)
	assert.Equal(t, ErrVerificationFailed, result.Error)
	patronClient.AssertExpectations(t)
}
