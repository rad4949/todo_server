package handler

import (
    "encoding/json"
    "net/http"
    "todo_server/internal/service"
)

type AuthHandler struct {
    jwtService *service.JWTService
}

func NewAuthHandler(jwtService *service.JWTService) *AuthHandler {
    return &AuthHandler{jwtService: jwtService}
}

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
}

// POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid json")
        return
    }

    // Для простоти — статичний користувач.
    // У реальному проєкті: шукаємо user у БД і перевіряємо bcrypt hash пароля
    if req.Username != "igor" || req.Password != "1234" {
        writeError(w, http.StatusUnauthorized, "invalid credentials")
        return
    }

    accessToken, err := h.jwtService.GenerateAccessToken("user-001", req.Username)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to generate token")
        return
    }

    refreshToken, err := h.jwtService.GenerateRefreshToken("user-001", req.Username)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to generate token")
        return
    }

    writeJSON(w, http.StatusOK, TokenResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
    })
}

// POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
    var body struct {
        RefreshToken string `json:"refresh_token"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeError(w, http.StatusBadRequest, "invalid json")
        return
    }

    // Валідуємо refresh token
    claims, err := h.jwtService.ValidateRefreshToken(body.RefreshToken)
    if err != nil {
        writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
        return
    }

    // Генеруємо новий access token
    accessToken, err := h.jwtService.GenerateAccessToken(claims.UserID, claims.Username)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to generate token")
        return
    }

    writeJSON(w, http.StatusOK, map[string]string{
        "access_token": accessToken,
    })
}