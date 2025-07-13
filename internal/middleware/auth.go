package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// AuthMiddleware provides JWT-based authentication
type AuthMiddleware struct {
	logger *zap.Logger
	config AuthConfig
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret     string   `json:"jwt_secret"`
	JWTIssuer     string   `json:"jwt_issuer"`
	SkipPaths     []string `json:"skip_paths"`
	TokenLocation string   `json:"token_location"` // "header", "cookie", "query"
	TokenName     string   `json:"token_name"`
	AuthType      string   `json:"auth_type"`
	SecretKey     string   `json:"secret_key"`
	TokenHeader   string   `json:"token_header"`
	PublicPaths   []string `json:"public_paths"`
}

// Claims represents JWT claims
type Claims struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(logger *zap.Logger, config map[string]any) (*AuthMiddleware, error) {
	authConfig := AuthConfig{
		TokenLocation: "header", // Default: Authorization header
		TokenName:     "Authorization",
		AuthType:      "jwt", // Default auth type
	}

	// Support both jwt_secret and secret_key parameter names
	if secret, ok := config["jwt_secret"].(string); ok {
		authConfig.JWTSecret = secret
	}
	if secretKey, ok := config["secret_key"].(string); ok {
		authConfig.SecretKey = secretKey
		// Use secret_key as jwt_secret if jwt_secret is not set
		if authConfig.JWTSecret == "" {
			authConfig.JWTSecret = secretKey
		}
	}

	if issuer, ok := config["jwt_issuer"].(string); ok {
		authConfig.JWTIssuer = issuer
	}

	// Support both skip_paths and public_paths parameter names
	if skipPaths, ok := config["skip_paths"].([]any); ok {
		for _, path := range skipPaths {
			if pathStr, ok := path.(string); ok {
				authConfig.SkipPaths = append(authConfig.SkipPaths, pathStr)
			}
		}
	}
	if publicPaths, ok := config["public_paths"].([]any); ok {
		for _, path := range publicPaths {
			if pathStr, ok := path.(string); ok {
				authConfig.PublicPaths = append(authConfig.PublicPaths, pathStr)
			}
		}
		// Use public_paths as skip_paths if skip_paths is not set
		if len(authConfig.SkipPaths) == 0 {
			authConfig.SkipPaths = authConfig.PublicPaths
		}
	}

	if tokenLocation, ok := config["token_location"].(string); ok {
		authConfig.TokenLocation = tokenLocation
	}

	if tokenName, ok := config["token_name"].(string); ok {
		authConfig.TokenName = tokenName
	}

	if tokenHeader, ok := config["token_header"].(string); ok {
		authConfig.TokenHeader = tokenHeader
		// Use token_header as token_name if token_name is not set
		if authConfig.TokenName == "Authorization" {
			authConfig.TokenName = tokenHeader
		}
	}

	if authType, ok := config["auth_type"].(string); ok {
		authConfig.AuthType = authType
	}

	// Validate required fields
	if authConfig.JWTSecret == "" {
		return nil, fmt.Errorf("jwt_secret or secret_key is required for auth middleware")
	}

	return &AuthMiddleware{
		logger: logger,
		config: authConfig,
	}, nil
}

// Handle implements the middleware interface
func (am *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if path should be skipped
		for _, skipPath := range am.config.SkipPaths {
			if strings.HasPrefix(r.URL.Path, skipPath) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract token
		token, err := am.extractToken(r)
		if err != nil {
			am.logger.Warn("Failed to extract token", zap.Error(err))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate token
		claims, err := am.validateToken(token)
		if err != nil {
			am.logger.Warn("Invalid token", zap.Error(err))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Add user information to request headers
		r.Header.Set("X-User-ID", claims.UserID)
		r.Header.Set("X-User-Email", claims.Email)
		r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))

		am.logger.Debug("Request authenticated",
			zap.String("user_id", claims.UserID),
			zap.String("email", claims.Email))

		next.ServeHTTP(w, r)
	})
}

// Name returns the middleware name
func (am *AuthMiddleware) Name() string {
	return "auth"
}

// extractToken extracts JWT token from request
func (am *AuthMiddleware) extractToken(r *http.Request) (string, error) {
	switch am.config.TokenLocation {
	case "header":
		authHeader := r.Header.Get(am.config.TokenName)
		if authHeader == "" {
			return "", fmt.Errorf("authorization header not found")
		}

		// Handle "Bearer <token>" format
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer "), nil
		}

		return authHeader, nil

	case "cookie":
		cookie, err := r.Cookie(am.config.TokenName)
		if err != nil {
			return "", fmt.Errorf("token cookie not found")
		}
		return cookie.Value, nil

	case "query":
		token := r.URL.Query().Get(am.config.TokenName)
		if token == "" {
			return "", fmt.Errorf("token query parameter not found")
		}
		return token, nil

	default:
		return "", fmt.Errorf("unsupported token location: %s", am.config.TokenLocation)
	}
}

// validateToken validates the JWT token and returns claims
func (am *AuthMiddleware) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Ensure the token is signed with the expected method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(am.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer if configured
	if am.config.JWTIssuer != "" && claims.Issuer != am.config.JWTIssuer {
		return nil, fmt.Errorf("invalid token issuer")
	}

	// Check token expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// GenerateToken generates a JWT token for the given user
func (am *AuthMiddleware) GenerateToken(userID, email string, roles []string, duration time.Duration) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    am.config.JWTIssuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(am.config.JWTSecret))
}

// ValidateRole checks if the user has the required role
func (am *AuthMiddleware) ValidateRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles := r.Header.Get("X-User-Roles")
			if roles == "" {
				am.logger.Warn("No roles found in request")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			userRoles := strings.Split(roles, ",")
			for _, role := range userRoles {
				if strings.TrimSpace(role) == requiredRole {
					next.ServeHTTP(w, r)
					return
				}
			}

			am.logger.Warn("Insufficient permissions",
				zap.String("required_role", requiredRole),
				zap.String("user_roles", roles))
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

// RequireAuth creates a handler that requires authentication
func (am *AuthMiddleware) RequireAuth(handler http.Handler) http.Handler {
	return am.Handle(handler)
}
