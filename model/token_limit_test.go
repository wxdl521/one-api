package model

import (
	"errors"
	"sync"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTokenWithUserLimitSerializesConcurrentCreation(t *testing.T) {
	truncateTables(t)
	user := &User{
		Username: "token-limit-user", Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "token-limit-aff",
	}
	require.NoError(t, DB.Create(user).Error)

	tokens := []*Token{
		{UserId: user.Id, Name: "first", Key: "token-limit-first", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, Group: "default"},
		{UserId: user.Id, Name: "second", Key: "token-limit-second", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, Group: "default"},
	}
	errs := make([]error, len(tokens))
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(tokens))
	for index, token := range tokens {
		go func(index int, token *Token) {
			defer waitGroup.Done()
			<-start
			errs[index] = CreateTokenWithUserLimit(token, 1)
		}(index, token)
	}
	close(start)
	waitGroup.Wait()

	successes := 0
	limitFailures := 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, ErrTokenLimitReached) {
			limitFailures++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, limitFailures)

	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}
