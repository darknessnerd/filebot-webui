package auth

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateJWT_WithProvidedToken(t *testing.T) {
	tokenString := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImNyaW5pdGliQGdtYWlsLmNvbSIsImV4cCI6MTc1NTkzNzk5OSwiaWF0IjoxNzU1ODUxNTk5LCJpc3MiOiJ3ZWJ1aS1za2VsZXRvbiIsIm5hbWUiOiJicnVuaW5peCIsInVzZXJfaWQiOjF9.ujc6Ol2AsUZxiIkZi3YE762biBFAx22h1DnJdLmPZQM"

	service := &Service{
		jwtSecret: []byte("dev-secret-key-change-in-production"), // Correct secret for the provided token
		jwtIssuer: "webui-skeleton",
	}

	claims, err := service.ValidateJWT(tokenString)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "crinitib@gmail.com", claims.Email)
	assert.Equal(t, "bruninix", claims.Name)
	assert.Equal(t, "1", claims.UserID)
}
