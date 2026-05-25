package service

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

// Claims — це payload токена: що зберігаємо всередині
type Claims struct {
    UserID   string `json:"userId"`
    Username string `json:"username"`
    jwt.RegisteredClaims  // стандартні поля: exp, iat, тощо
}

type JWTService struct {
    accessSecret  string
    refreshSecret string
}

func NewJWTService(accessSecret, refreshSecret string) *JWTService {
    return &JWTService{
        accessSecret:  accessSecret,
        refreshSecret: refreshSecret,
    }
}

// GenerateAccessToken — живе 15 хвилин
func (s *JWTService) GenerateAccessToken(userID, username string) (string, error) {
    claims := Claims{
        UserID:   userID,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(s.accessSecret))
}

// GenerateRefreshToken — живе 7 днів
func (s *JWTService) GenerateRefreshToken(userID, username string) (string, error) {
    claims := Claims{
        UserID:   userID,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(s.refreshSecret))
}

// ValidateAccessToken — перевіряє і розкодовує access token
func (s *JWTService) ValidateAccessToken(tokenStr string) (*Claims, error) {
    return s.parseToken(tokenStr, s.accessSecret)
}

// ValidateRefreshToken — перевіряє і розкодовує refresh token
func (s *JWTService) ValidateRefreshToken(tokenStr string) (*Claims, error) {
    return s.parseToken(tokenStr, s.refreshSecret)
}

func (s *JWTService) parseToken(tokenStr, secret string) (*Claims, error) {
    claims := &Claims{}
    _, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
        return []byte(secret), nil
    })
    if err != nil {
        return nil, err
    }
    return claims, nil
}